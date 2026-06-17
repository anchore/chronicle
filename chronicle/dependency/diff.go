package dependency

import (
	"context"
	"sort"

	"golang.org/x/sync/errgroup"
)

// ChangeKind classifies how a package changed between two scans.
type ChangeKind string

const (
	Added      ChangeKind = "added"
	Removed    ChangeKind = "removed"
	Updated    ChangeKind = "updated"    // version increased
	Downgraded ChangeKind = "downgraded" // version decreased
)

// DiffConfig carries everything ComputeDiff needs beyond the Scanner: the version
// Comparer it drives and the run parameters. It holds no UI, status, or bus type
// — progress is published by the scanner from deep inside, and the per-ref
// figures the UI needs come back on the returned Diff's scans. Whether to
// attribute vulnerabilities is inferred from the scanned scans, not a field
// here, so it can't disagree with how the scanner was built.
type DiffConfig struct {
	// Comparer classifies a version change as an update or a downgrade; injected
	// so the core stays free of grype's ecosystem-aware version logic.
	Comparer VersionComparer

	SinceRef string
	UntilRef string

	// MinSeverity filters which vulnerabilities annotation attributes to a change;
	// "" includes all. Ignored when neither ref was vuln-matched.
	MinSeverity string
}

// VersionComparer classifies the direction of a version change for a given
// package ecosystem. Implemented by scan.grypeVersionComparer in production;
// a trivial fake is used in tests.
type VersionComparer interface {
	// Compare returns <0 if a<b, 0 if equal, >0 if a>b, and ok=false when
	// the versions are not comparable in this ecosystem.
	Compare(pkgType, a, b string) (int, bool)
}

// Diff is the result of a Compare: the package changes and the derived rollups
// (per-kind totals and unique-vuln-ID counts), together with the two Scans
// that were compared. Build it through NewDiff (or annotate), which sorts the
// changes and computes the rollups from them, so a freshly built diff's totals
// and counts agree with its changes. The zero value is a valid empty diff.
// Display preferences live separately (render.Config), not here.
type Diff struct {
	Changes         []PackageChange
	Totals          ChangeTotals
	RemediatedCount int // unique vuln IDs across all changes' Remediated
	IntroducedCount int // unique vuln IDs across all changes' Introduced
	// RemainingCount is the unique vuln IDs present at BOTH since and until —
	// carried over, neither remediated (gone at until) nor introduced (new at
	// until) — across every package in the latest scan, not just the changed
	// ones. Unlike the two counts above it is not derivable from Changes (it
	// spans unchanged packages too), so annotate sets it from the scans rather
	// than NewDiff deriving it. Zero on an unannotated diff.
	RemainingCount int

	// Since and Until are the scans that were compared, retained so callers
	// can derive per-ref scan figures (package and vulnerability counts, match
	// success) straight from the raw data rather than from a precomputed summary.
	// Excluded from JSON: the serialized changelog artifact is the delta, not the
	// full per-ref catalogs.
	Since Scan `json:"-"`
	Until Scan `json:"-"`

	// Remaining is the carried-over vulnerabilities behind RemainingCount: the
	// standing vulnerability burden this release did not clear, per package,
	// across the whole latest scan. Populated by annotate. Excluded from JSON
	// for the same reason as Since/Until — it is derived from the full per-ref
	// catalogs, not part of the serialized delta; RemainingCount carries the
	// rollup figure instead.
	Remaining []PackageVulns `json:"-"`
}

// ChangeTotals is a per-kind count of package changes.
type ChangeTotals struct {
	Updated    int
	Downgraded int
	Added      int
	Removed    int
}

// PackageChange is one package's delta between the since and until scans,
// optionally annotated with the vulnerabilities the change remediated or
// introduced (Vuln, nil until annotate runs).
type PackageChange struct {
	Name        string
	Type        string // syft package type; render maps it to an ecosystem label
	FromVersion string // "" for Added
	ToVersion   string // "" for Removed
	Kind        ChangeKind
	Vuln        *VulnDelta // nil unless annotated
}

// ComputeDiff diffs the dependency graph between cfg.SinceRef and cfg.UntilRef.
// It scans both refs in parallel (the scanner materializes, catalogs, and — when
// matching — attributes each against its shared vulnerability DB so the two sides
// never drift), compares the resulting scans, and attributes the
// vulnerabilities each change remediated or introduced when either ref was
// vuln-matched. It drives no UI: the scanner publishes live progress to the bus
// itself, and the per-ref figures come back on the returned Diff's scans.
func ComputeDiff(ctx context.Context, scanner Scanner, cfg DiffConfig) (*Diff, error) {
	// scan both refs end-to-end in parallel: each materializes, catalogs, and
	// (when matching) attributes against the scanner's shared DB. errgroup cancels
	// the sibling on the first failure and joins the error.
	var since, until Scan
	g, gctx := errgroup.WithContext(ctx)
	g.Go(func() error {
		var err error
		since, err = scanner.Scan(gctx, cfg.SinceRef)
		return err
	})
	g.Go(func() error {
		var err error
		until, err = scanner.Scan(gctx, cfg.UntilRef)
		return err
	})
	if err := g.Wait(); err != nil {
		return nil, err
	}

	diff := compare(since, until, cfg.Comparer)

	// attribute vulnerabilities when either ref was actually vuln-matched. A
	// matched ref always carries a non-nil Vulns map (empty if it found none), so
	// the map's presence is the annotate signal — no separate flag is needed, and
	// it can't disagree with how the scanner was built.
	if diff.Since.Vulns != nil || diff.Until.Vulns != nil {
		diff = annotate(diff, annotateConfig{MinSeverity: cfg.MinSeverity})
	}

	return &diff, nil
}

// compare builds a Diff from two scans. Packages present only in since are
// Removed; only in until are Added; present in both with a differing version are
// Updated or Downgraded (determined by cmp). Equal versions are omitted. The
// Changes slice is sorted deterministically (by Type then Name) so output is
// stable for tests and rendering. The compared scans are retained on the
// returned diff so callers can read the raw per-ref data behind it.
func compare(since, until Scan, cmp VersionComparer) Diff {
	sinceIdx := indexPackages(since.Packages)
	untilIdx := indexPackages(until.Packages)

	var changes []PackageChange

	// packages present in since — either Removed or changed
	for key, sincePkg := range sinceIdx {
		untilPkg, inUntil := untilIdx[key]
		if !inUntil {
			changes = append(changes, PackageChange{
				Name:        sincePkg.Name,
				Type:        sincePkg.Type,
				FromVersion: sincePkg.Version,
				ToVersion:   "",
				Kind:        Removed,
			})
			continue
		}

		if sincePkg.Version == untilPkg.Version {
			// no change; omit
			continue
		}

		kind := classifyVersionChange(sincePkg.Type, sincePkg.Version, untilPkg.Version, cmp)
		changes = append(changes, PackageChange{
			Name:        sincePkg.Name,
			Type:        sincePkg.Type,
			FromVersion: sincePkg.Version,
			ToVersion:   untilPkg.Version,
			Kind:        kind,
		})
	}

	// packages only in until are Added
	for key, untilPkg := range untilIdx {
		if _, inSince := sinceIdx[key]; !inSince {
			changes = append(changes, PackageChange{
				Name:        untilPkg.Name,
				Type:        untilPkg.Type,
				FromVersion: "",
				ToVersion:   untilPkg.Version,
				Kind:        Added,
			})
		}
	}

	// NewDiff sorts and tallies, so the rollups can't drift from the changes.
	d := NewDiff(changes)
	d.Since = since
	d.Until = until
	return d
}

// sortChanges orders changes deterministically by Type then Name so output is
// stable for tests and rendering. NewDiff calls it for every diff it builds.
func sortChanges(changes []PackageChange) {
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Type != changes[j].Type {
			return changes[i].Type < changes[j].Type
		}
		return changes[i].Name < changes[j].Name
	})
}

// indexPackages returns a map from PackageKey to Package for fast lookup.
func indexPackages(pkgs []Package) map[PackageKey]Package {
	idx := make(map[PackageKey]Package, len(pkgs))
	for _, p := range pkgs {
		idx[p.key()] = p
	}
	return idx
}

// classifyVersionChange uses cmp to decide Updated vs Downgraded. When cmp
// reports ok=false (versions not comparable), we classify as Updated (changed
// but direction unknown).
func classifyVersionChange(pkgType, from, to string, cmp VersionComparer) ChangeKind {
	result, ok := cmp.Compare(pkgType, from, to)
	if !ok {
		// unknown direction — treat as updated
		return Updated
	}
	if result > 0 {
		// Compare(from, to) > 0 means from > to: the version decreased
		return Downgraded
	}
	// from < to (version increased), or string-differs-but-equal — treat as an update
	return Updated
}

// key returns the PackageKey identity for a change, mirroring Package.key so
// annotation can index a change straight into a Scan's vuln map.
func (c PackageChange) key() PackageKey {
	return PackageKey{Type: c.Type, Name: c.Name}
}

// NewDiff freezes a set of changes into a Diff: it sorts them deterministically
// (by Type then Name) and derives the per-kind totals and unique-vuln-ID counts
// from the changes themselves. annotate routes through NewDiff too, so an
// annotated diff's counts reflect the per-change Vuln deltas.
func NewDiff(changes []PackageChange) Diff {
	sortChanges(changes)
	return Diff{
		Changes:         changes,
		Totals:          tallyKinds(changes),
		RemediatedCount: countUniqueIDs(changes, remediatedOf),
		IntroducedCount: countUniqueIDs(changes, introducedOf),
	}
}

// Total is the sum across all kinds.
func (t ChangeTotals) Total() int {
	return t.Updated + t.Downgraded + t.Added + t.Removed
}

// tallyKinds counts changes by kind.
func tallyKinds(changes []PackageChange) ChangeTotals {
	var t ChangeTotals
	for _, c := range changes {
		switch c.Kind {
		case Updated:
			t.Updated++
		case Downgraded:
			t.Downgraded++
		case Added:
			t.Added++
		case Removed:
			t.Removed++
		}
	}
	return t
}

// remediatedOf and introducedOf adapt a change's Vuln delta for countUniqueIDs;
// both are nil-safe for unannotated changes.
func remediatedOf(c PackageChange) []Vulnerability {
	if c.Vuln == nil {
		return nil
	}
	return c.Vuln.Remediated
}

func introducedOf(c PackageChange) []Vulnerability {
	if c.Vuln == nil {
		return nil
	}
	return c.Vuln.Introduced
}

// countUniqueIDs counts distinct vulnerability IDs across all changes using the
// provided accessor, so the rollup counts are not inflated by the same CVE
// appearing in multiple packages.
func countUniqueIDs(changes []PackageChange, accessor func(PackageChange) []Vulnerability) int {
	seen := make(map[string]bool)
	for _, ch := range changes {
		for _, v := range accessor(ch) {
			seen[v.ID] = true
		}
	}
	return len(seen)
}
