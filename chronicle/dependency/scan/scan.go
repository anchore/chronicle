package scan

// Package scan is the ONLY place in chronicle that imports syft and grype. It
// turns a materialized source directory into a dependency.Snapshot: a syft
// catalog of packages plus (optionally) a grype match of vulnerabilities. The
// pure diff/annotate logic in the parent package never touches these libraries.

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"

	"github.com/anchore/chronicle/chronicle/dependency"
	"github.com/anchore/chronicle/internal/log"
	"github.com/anchore/clio"
	"github.com/anchore/grype/grype"
	"github.com/anchore/grype/grype/db/v6/distribution"
	"github.com/anchore/grype/grype/db/v6/installation"
	grypeMatcher "github.com/anchore/grype/grype/matcher"
	grypePkg "github.com/anchore/grype/grype/pkg"
	"github.com/anchore/grype/grype/vulnerability"
	"github.com/anchore/syft/syft"
	"github.com/anchore/syft/syft/cataloging"
	"github.com/anchore/syft/syft/sbom"
	"github.com/anchore/syft/syft/source"
)

// Hooks lets the caller observe scan milestones. All fields are optional.
type Hooks struct {
	// OnSourceID fires once syft has resolved the source, before cataloging
	// begins, with the source's stable ID. The caller uses it to attribute the
	// live cataloging progress syft publishes on the bus to the right UI element.
	OnSourceID func(srcID string)
}

// SourceInfo names the materialized source so syft derives a stable artifact ID
// (and emits no "no explicit name and version" warning) instead of deriving one
// from the throwaway tmpdir path. Name is the project, Version is the ref.
type SourceInfo struct {
	Name    string
	Version string
}

// Catalog is the syft side of a scan: the resolved packages plus the opaque
// SBOM retained for a later vulnerability match. Cataloging (syft) and DB
// loading (grype) are independent, so the orchestrator runs them in parallel
// and only joins them in Match.
type Catalog struct {
	Packages []dependency.Package
	sb       *sbom.SBOM // opaque to callers; consumed by Match
}

// VulnDB is an opaque handle to a loaded grype vulnerability database, so the
// orchestrator can pass it between the load and match phases without importing
// grype itself (scan stays the only package that does).
type VulnDB struct {
	provider vulnerability.Provider
}

// Scanner turns a materialized directory into packages (Catalog) and, given a
// loaded DB, into vulnerability matches (Match). The two phases are split so
// they can be driven and rendered independently.
type Scanner interface {
	Catalog(ctx context.Context, dir string, info SourceInfo, hooks Hooks) (*Catalog, error)
	Match(ctx context.Context, db *VulnDB, cat *Catalog) (map[dependency.PackageKey][]dependency.Vulnerability, error)
}

// scanner is the syft+grype implementation.
type scanner struct {
	ecosystems   []string // syft cataloger selection expressions (e.g. "language", "go")
	excludePaths []string // syft exclude patterns (each must start with ./, */, or **/)
}

// getSourceMu serializes syft.GetSource across concurrent ref scans. GetSource
// builds the shared stereoscope provider registry, which appends to a package
// global without locking; without this two parallel scans race there. The heavy
// cataloging (CreateSBOM) still runs in parallel.
var getSourceMu sync.Mutex

// NewScanner builds a Scanner. ecosystems are syft cataloger selection
// expressions that scope cataloging (e.g. ["language"] or ["go"]); empty means
// syft's default directory catalogers. excludePaths are syft exclude patterns
// that prune directories from the scan (each must start with ./, */, or **/);
// empty means scan everything.
func NewScanner(ecosystems, excludePaths []string) Scanner {
	return &scanner{ecosystems: ecosystems, excludePaths: excludePaths}
}

// LoadDB curates the grype vulnerability DB, reusing grype's own cache location
// (id="grype" => ~/.cache/grype/db/...) so chronicle shares the grype CLI's DB
// rather than maintaining a separate one. Call once per run and reuse the
// returned handle for both the since and until matches so the two sides never
// drift against different DBs.
func LoadDB(autoUpdate bool) (*VulnDB, error) {
	distCfg := distribution.DefaultConfig()
	// seed installation config with grype's identity (NOT chronicle's) so the
	// cache dir resolves to grype's existing location.
	installCfg := installation.DefaultConfig(clio.Identification{Name: "grype"})

	provider, _, err := grype.LoadVulnerabilityDB(distCfg, installCfg, autoUpdate)
	if err != nil {
		return nil, fmt.Errorf("unable to load vulnerability DB: %w", err)
	}
	return &VulnDB{provider: provider}, nil
}

func (s *scanner) Catalog(ctx context.Context, dir string, info SourceInfo, hooks Hooks) (*Catalog, error) {
	if len(s.excludePaths) > 0 {
		// syft resolves symlinks when indexing the tree but derives exclusion
		// roots from filepath.Abs (no symlink resolution), so when the scan dir
		// sits behind a symlink (e.g. macOS /var → /private/var tmpdirs) the
		// patterns silently match nothing. Canonicalize the root up front so the
		// exclusions line up with the indexed paths. Best-effort: on error keep the
		// original path (exclusions may simply not apply).
		if resolved, err := filepath.EvalSymlinks(dir); err == nil {
			dir = resolved
		}
	}

	srcCfg := syft.DefaultGetSourceConfig()
	if info.Name != "" || info.Version != "" {
		// name the source so syft derives a stable artifact ID from the project
		// + ref rather than the throwaway tmpdir path (avoids the "no explicit
		// name and version provided for directory source" warning).
		srcCfg = srcCfg.WithAlias(source.Alias{Name: info.Name, Version: info.Version})
	}
	if len(s.excludePaths) > 0 {
		// prune the requested paths from the index before cataloging (e.g. vendored
		// or test trees). Patterns are relative to the scan root and resolved by
		// syft's directory source.
		srcCfg = srcCfg.WithExcludeConfig(source.ExcludeConfig{Paths: s.excludePaths})
	}

	getSourceMu.Lock()
	src, err := syft.GetSource(ctx, dir, srcCfg)
	getSourceMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("unable to create source from %q: %w", dir, err)
	}
	// surface the source ID before cataloging so the caller can attribute the
	// live progress syft is about to publish to the right UI element.
	if hooks.OnSourceID != nil {
		hooks.OnSourceID(string(src.ID()))
	}
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

	return &Catalog{Packages: mapPackages(sb), sb: sb}, nil
}

// Match runs grype over a previously produced catalog. A nil db or catalog
// yields no matches (packages-only).
func (s *scanner) Match(ctx context.Context, db *VulnDB, cat *Catalog) (map[dependency.PackageKey][]dependency.Vulnerability, error) {
	if db == nil || db.provider == nil || cat == nil {
		return nil, nil
	}
	return matchSBOM(ctx, db.provider, cat.sb)
}

// mapPackages folds syft's package collection into the pure dependency.Package
// model (identity is Type+Name).
func mapPackages(sb *sbom.SBOM) []dependency.Package {
	var out []dependency.Package
	for p := range sb.Artifacts.Packages.Enumerate() {
		out = append(out, dependency.Package{
			Name:      p.Name,
			Version:   p.Version,
			Type:      string(p.Type),
			Ecosystem: ecosystemTitle(p),
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
			Package:    m.Package.Name,
			FixState:   string(m.Vulnerability.Fix.State),
			DataSource: dataSource,
		})
	}

	return out, nil
}
