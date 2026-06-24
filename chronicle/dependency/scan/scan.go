package scan

// Package scan is the ONLY place in chronicle that imports syft and grype. It
// turns a materialized source directory into a dependency.Scan: a syft
// catalog of packages plus (optionally) a grype match of vulnerabilities. The
// pure diff/annotate logic in the parent package never touches these libraries.
// It implements dependency.Scanner, which ComputeDiff injects.

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/anchore/chronicle/chronicle/dependency"
	"github.com/anchore/chronicle/chronicle/dependency/source"
	"github.com/anchore/chronicle/internal/bus"
	"github.com/anchore/chronicle/internal/log"
	"github.com/anchore/grype/grype"
	grypeMatcher "github.com/anchore/grype/grype/matcher"
	grypePkg "github.com/anchore/grype/grype/pkg"
	"github.com/anchore/grype/grype/vulnerability"
	"github.com/anchore/syft/syft"
	"github.com/anchore/syft/syft/cataloging"
	"github.com/anchore/syft/syft/sbom"
	syftSource "github.com/anchore/syft/syft/source"
)

// catalog is the syft side of a scan: the resolved packages plus the opaque SBOM
// retained for a later vulnerability match. Cataloging (syft) and DB loading
// (grype) are independent, so a scan overlaps them and only joins them in match.
type catalog struct {
	packages []dependency.Package
	sb       *sbom.SBOM // consumed by matchSBOM
}

// scanner is the syft+grype implementation of dependency.Scanner. It owns the
// pipeline behind a ref: materializing it onto disk via the source Target,
// cataloging with syft, and — when a vulnerability DB was supplied — matching
// with grype. The DB itself is loaded by the caller (see LoadDB) and handed in,
// so the scanner no longer manages DB download/refresh; a nil DB means
// packages-only.
type scanner struct {
	target       source.Target // materializes a ref into a directory to catalog
	sourceName   string        // project name, so syft derives a stable artifact ID per ref
	ecosystems   []string      // syft cataloger selection expressions (e.g. "language", "go")
	excludePaths []string      // syft exclude patterns (each must start with ./, */, or **/)
	recursive    bool          // when false, scan only the root dir (top-level subdirs are pruned)

	// provider is the loaded grype vulnerability DB to match against; nil means
	// packages-only (no matching).
	provider vulnerability.Provider
}

// getSourceMu serializes syft.GetSource across concurrent ref scans. GetSource
// builds the shared stereoscope provider registry, which appends to a package
// global without locking; without this two parallel scans race there. The heavy
// cataloging (CreateSBOM) still runs in parallel.
var getSourceMu sync.Mutex

// NewScanner builds a Scanner. target materializes each ref into a directory to
// catalog; sourceName names the project so syft derives a stable artifact ID per
// ref. ecosystems are syft cataloger selection expressions that scope cataloging
// (e.g. ["language"] or ["go"]); empty means syft's default directory catalogers.
// excludePaths are syft exclude patterns that prune directories from the scan
// (each must start with ./, */, or **/); empty means scan everything. recursive
// controls scan depth: when false, only the root dir is cataloged (every
// top-level subdir is pruned); when true, the whole tree is scanned. db is the
// loaded vulnerability DB to match against (from LoadDB); nil means scan packages
// only.
func NewScanner(target source.Target, sourceName string, ecosystems, excludePaths []string, recursive bool, db *DB) dependency.Scanner {
	s := &scanner{
		target:       target,
		sourceName:   sourceName,
		ecosystems:   ecosystems,
		excludePaths: excludePaths,
		recursive:    recursive,
	}
	if db != nil {
		s.provider = db.provider
	}
	return s
}

// Scan materializes ref, catalogs it, and — when the scanner is annotating —
// waits for the shared DB and matches vulnerabilities, returning the per-ref
// Scan. A materialize or catalog failure is fatal (returned as an error); a
// DB-load or match failure is non-fatal — logged here and degraded to a
// packages-only Scan (nil Vulns), which the caller surfaces as a failed vuln
// branch.
func (s *scanner) Scan(ctx context.Context, ref string) (dependency.Scan, error) {
	dir, cleanup, err := s.target.Materialize(ctx, ref)
	if err != nil {
		return dependency.Scan{}, fmt.Errorf("unable to materialize ref %q: %w", ref, err)
	}
	defer func() {
		if cerr := cleanup(); cerr != nil {
			log.WithFields("error", cerr).Trace("unable to clean up materialized dependency source")
		}
	}()

	return s.scanDir(ctx, dir, ref)
}

// scanDir catalogs an already-materialized directory and, when a DB was supplied,
// matches it. Split from Scan so tests can exercise the syft/grype path against a
// fixture directory without going through git.
func (s *scanner) scanDir(ctx context.Context, dir, ref string) (dependency.Scan, error) {
	cat, err := s.catalog(ctx, dir, ref)
	if err != nil {
		return dependency.Scan{}, err
	}
	snap := dependency.Scan{Packages: cat.packages}

	// no DB supplied: packages-only.
	if s.provider == nil {
		return snap, nil
	}

	vulns, err := matchSBOM(ctx, s.provider, cat.sb)
	if err != nil {
		log.WithFields("error", err, "ref", ref).Warn("unable to match vulnerabilities; skipping")
		return snap, nil
	}
	snap.Vulns = vulns
	return snap, nil
}

// catalog turns a materialized dir into packages (plus the SBOM that matchSBOM
// consumes), linking the resolved syft source to the bus so the UI can attribute
// live cataloging progress to the right ref.
func (s *scanner) catalog(ctx context.Context, dir, ref string) (*catalog, error) {
	if len(s.excludePaths) > 0 || !s.recursive {
		// syft resolves symlinks when indexing the tree but derives exclusion
		// roots from filepath.Abs (no symlink resolution), so when the scan dir
		// sits behind a symlink (e.g. macOS /var → /private/var tmpdirs) the
		// patterns silently match nothing. Canonicalize the root up front so the
		// exclusions line up with the indexed paths. Best-effort: on error keep the
		// original path (exclusions may simply not apply). Done before deriving the
		// non-recursive prunes below so their names match the indexed paths too.
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			dir = resolved
		}
	}

	excludes, err := s.effectiveExcludes(dir)
	if err != nil {
		return nil, err
	}

	srcCfg := syft.DefaultGetSourceConfig()
	if s.sourceName != "" || ref != "" {
		// name the source so syft derives a stable artifact ID from the project
		// + ref rather than the throwaway tmpdir path (avoids the "no explicit
		// name and version provided for directory source" warning).
		srcCfg = srcCfg.WithAlias(syftSource.Alias{Name: s.sourceName, Version: ref})
	}
	if len(excludes) > 0 {
		// prune the requested paths from the index before cataloging (e.g. vendored
		// or test trees, plus every top-level subdir when non-recursive). Patterns
		// are relative to the scan root and resolved by syft's directory source.
		srcCfg = srcCfg.WithExcludeConfig(syftSource.ExcludeConfig{Paths: excludes})
	}

	getSourceMu.Lock()
	src, err := syft.GetSource(ctx, dir, srcCfg)
	getSourceMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("unable to create source from %q: %w", dir, err)
	}
	// link the source ID before cataloging so the UI can attribute the live
	// progress syft is about to publish to this ref's branch leaf. Published from
	// here (deep in the integration) rather than handed back up through the
	// Scanner contract — the core stays free of any UI concern.
	bus.LinkSBOMSource(ref, string(src.ID()))
	defer func() {
		if cerr := src.Close(); cerr != nil {
			// non-fatal: the source is a temp dir owned by the caller's cleanup.
			log.WithFields("error", cerr).Trace("unable to close syft source")
		}
	}()

	sbomCfg := syft.DefaultCreateSBOMConfig()
	if len(s.ecosystems) > 0 {
		// scope cataloging to the requested ecosystems (e.g. "language", "go").
		// syft fills the source-appropriate default cataloger set (directory)
		// automatically; the expressions sub-select within it.
		sbomCfg = sbomCfg.WithCatalogerSelection(
			cataloging.NewSelectionRequest().WithExpression(s.ecosystems...),
		)
	}

	sb, err := syft.CreateSBOM(ctx, src, sbomCfg)
	if err != nil {
		return nil, fmt.Errorf("unable to catalog packages: %w", err)
	}

	return &catalog{packages: mapPackages(sb), sb: sb}, nil
}

// effectiveExcludes combines the user's exclude patterns with the synthetic
// prunes that enforce non-recursive scanning. No single syft glob can express
// "root only" — a depth-1 pattern (e.g. "*") also matches root files, which the
// directory source can't distinguish from dirs — so instead we read the root's
// top-level entries and emit one "./<name>" prune per subdir. syft's directory
// source matches each against the indexed path and returns filepath.SkipDir for
// the dir, pruning the whole subtree while leaving root files untouched. dir is
// expected to already be symlink-resolved by the caller.
func (s *scanner) effectiveExcludes(dir string) ([]string, error) {
	if s.recursive {
		return s.excludePaths, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, fmt.Errorf("unable to read scan root %q for non-recursive pruning: %w", dir, err)
	}

	// preserve the user's patterns, then prune every top-level subdir. Track the
	// patterns already seen so a user exclude that coincides with a synthetic
	// subdir prune (e.g. a user "./vendor" plus the vendor/ dir) isn't emitted
	// twice.
	excludes := append([]string(nil), s.excludePaths...)
	seen := make(map[string]struct{}, len(excludes))
	for _, p := range excludes {
		seen[p] = struct{}{}
	}
	for _, e := range entries {
		if !isDir(dir, e) {
			continue
		}
		pattern := "./" + e.Name()
		if _, dup := seen[pattern]; dup {
			continue
		}
		seen[pattern] = struct{}{}
		excludes = append(excludes, pattern)
	}
	return excludes, nil
}

// isDir reports whether the top-level entry e is a directory, resolving symlinks.
// DirEntry.IsDir is false for a symlink even when it points at a directory, but
// syft resolves symlinks while indexing — so a root-level dir symlink would
// otherwise escape the non-recursive prune and leak its manifests. Best-effort:
// a symlink we can't stat is treated as a non-dir (left in the scan), matching
// the lenient symlink handling in catalog.
func isDir(dir string, e os.DirEntry) bool {
	if e.IsDir() {
		return true
	}
	if e.Type()&os.ModeSymlink == 0 {
		return false
	}
	info, err := os.Stat(filepath.Join(dir, e.Name()))
	return err == nil && info.IsDir()
}

// mapPackages folds syft's package collection into the pure dependency.Package
// model (identity is Type+Name). The raw syft type string carries through; the
// human-friendly ecosystem label is derived later, in the render layer.
func mapPackages(sb *sbom.SBOM) []dependency.Package {
	var out []dependency.Package
	for p := range sb.Artifacts.Packages.Enumerate() {
		out = append(out, dependency.Package{
			Name:    p.Name,
			Version: p.Version,
			Type:    string(p.Type),
		})
	}
	return out
}

// matchSBOM runs grype over the syft catalog and folds the results into a map
// keyed by package identity, so Annotate can attribute each vuln to a concrete
// change.
func matchSBOM(ctx context.Context, provider vulnerability.Provider, sb *sbom.SBOM) (map[dependency.PackageKey][]dependency.Vulnerability, error) {
	gpkgPtrs := grypePkg.FromCollection(sb.Artifacts.Packages, sb.Relationships, grypePkg.SynthesisConfig{})
	// FromCollection yields pointers; the matcher wants values (mirrors grype's
	// own pkg.Provide).
	gpkgs := make([]grypePkg.Package, 0, len(gpkgPtrs))
	for _, p := range gpkgPtrs {
		if p != nil {
			gpkgs = append(gpkgs, *p)
		}
	}

	vm := grype.VulnerabilityMatcher{
		VulnerabilityProvider: provider,
		Matchers:              grypeMatcher.NewDefaultMatchers(grypeMatcher.Config{}),
	}

	matches, _, err := vm.FindMatchesContext(ctx, gpkgs, grypePkg.Context{})
	if err != nil {
		return nil, fmt.Errorf("unable to find vulnerability matches: %w", err)
	}

	out := make(map[dependency.PackageKey][]dependency.Vulnerability)
	seen := make(map[dependency.PackageKey]map[string]struct{})
	for _, m := range matches.Sorted() {
		key := dependency.PackageKey{Type: string(m.Package.Type), Name: m.Package.Name}

		// dedupe vuln IDs per package — a single CVE can match a package via
		// multiple details/namespaces but is one vuln for our purposes.
		ids := seen[key]
		if ids == nil {
			ids = make(map[string]struct{})
			seen[key] = ids
		}
		if _, dup := ids[m.Vulnerability.ID]; dup {
			continue
		}
		ids[m.Vulnerability.ID] = struct{}{}

		var severity, dataSource string
		if m.Vulnerability.Metadata != nil {
			severity = m.Vulnerability.Metadata.Severity
			// the primary reference URL grype recorded for this ID (e.g. the NVD
			// or GHSA page); encoders use it to make the CVE/GHSA ID clickable.
			dataSource = m.Vulnerability.Metadata.DataSource
		}

		out[key] = append(out[key], dependency.Vulnerability{
			ID:         m.Vulnerability.ID,
			Severity:   severity,
			FixState:   string(m.Vulnerability.Fix.State),
			DataSource: dataSource,
		})
	}

	return out, nil
}
