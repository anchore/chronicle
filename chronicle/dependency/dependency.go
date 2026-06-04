package dependency

// Package dependency holds the pure data types and algorithms for the
// dependency-diff feature. It deliberately imports no syft/grype code so the
// compare/annotate logic stays unit-testable with fixtures. The syft+grype
// integration lives in the dependency/scan subpackage; presentation (summary
// prose, version/vuln formatting, ecosystem grouping) lives in
// dependency/render.

import "encoding/json"

// Package is a single resolved dependency at a point in time. Identity is the
// (Type, Name) pair; Version distinguishes points in time.
type Package struct {
	Name    string // identity (with Type)
	Version string
	Type    string // syft package type string, e.g. "go-module", "npm"
	// Ecosystem is the human-friendly grouping title derived by the scan
	// subpackage from syft's own package semantics (Language/Type). Kept here as
	// a plain string so the core stays syft-free; empty for hand-built packages.
	Ecosystem string
}

// ChangeKind classifies how a package changed between two snapshots.
type ChangeKind string

const (
	Added      ChangeKind = "added"
	Removed    ChangeKind = "removed"
	Updated    ChangeKind = "updated"    // version increased
	Downgraded ChangeKind = "downgraded" // version decreased
)

// Vulnerability is a single match against a package at a point in time.
type Vulnerability struct {
	ID         string // CVE-…, GHSA-…
	Severity   string
	Package    string // package the match was on
	FixState   string // fixed / not-fixed / unknown
	DataSource string // grype's primary reference URL for the ID; "" if none. Encoders use it to make the ID clickable.
}

// VulnDelta is the vulnerability attribution for one change: what the change
// remediated (gone at until) and introduced (new at until). It is populated by
// Annotate; a nil *VulnDelta on a PackageChange means "not annotated", distinct
// from an empty delta meaning "annotated, no vulnerability impact".
type VulnDelta struct {
	Remediated []Vulnerability // present at `since` for this pkg, gone at `until`
	Introduced []Vulnerability // present at `until`, absent at `since`
}

// HasImpact reports whether the change remediated or introduced any
// vulnerability. Nil-safe: an unannotated (nil) delta has no impact.
func (d *VulnDelta) HasImpact() bool {
	return d != nil && (len(d.Remediated) > 0 || len(d.Introduced) > 0)
}

// PackageChange is one package's delta between the since and until snapshots,
// optionally annotated with the vulnerabilities the change remediated or
// introduced (Vuln, nil until Annotate runs).
type PackageChange struct {
	Name        string
	Type        string
	Ecosystem   string // grouping title; see Package.Ecosystem
	FromVersion string // "" for Added
	ToVersion   string // "" for Removed
	Kind        ChangeKind
	Vuln        *VulnDelta // nil unless annotated
}

// Key returns the PackageKey identity for a change, mirroring Package.Key so
// annotation can index a change straight into a Snapshot's vuln map.
func (c PackageChange) Key() PackageKey {
	return PackageKey{Type: c.Type, Name: c.Name}
}

// Diff is the full, immutable result of a Compare: the package changes plus the
// derived rollups (per-kind totals and unique-vuln-ID counts). It is built only
// through NewDiff (or Annotate), which compute every rollup from the changes in
// lockstep, so the totals and counts can never drift from the changes. The zero
// value is a valid empty diff. Read it through the accessors; encoders never
// mutate it. Display preferences live separately (render.Config), not here.
type Diff struct {
	changes         []PackageChange
	totals          ChangeTotals
	remediatedCount int // unique vuln IDs across all changes' Remediated
	introducedCount int // unique vuln IDs across all changes' Introduced
}

// NewDiff freezes a set of changes into a Diff: it sorts them deterministically
// (by Type then Name) and derives the per-kind totals and unique-vuln-ID counts
// from the changes themselves. Because every rollup is computed here, they can
// never disagree with the changes. Annotate routes through NewDiff too, so an
// annotated diff's counts reflect the per-change Vuln deltas.
func NewDiff(changes []PackageChange) Diff {
	sortChanges(changes)
	return Diff{
		changes:         changes,
		totals:          tallyKinds(changes),
		remediatedCount: countUniqueIDs(changes, remediatedOf),
		introducedCount: countUniqueIDs(changes, introducedOf),
	}
}

// Changes returns the diff's changes for read-only iteration. It hands back a
// shallow copy of the slice so callers cannot append to or reorder the diff's
// backing array.
func (d Diff) Changes() []PackageChange {
	out := make([]PackageChange, len(d.changes))
	copy(out, d.changes)
	return out
}

// Totals returns the per-kind change totals.
func (d Diff) Totals() ChangeTotals { return d.totals }

// RemediatedCount returns the number of unique vulnerability IDs remediated
// across all changes (0 when the diff was never annotated).
func (d Diff) RemediatedCount() int { return d.remediatedCount }

// IntroducedCount returns the number of unique vulnerability IDs introduced
// across all changes (0 when the diff was never annotated).
func (d Diff) IntroducedCount() int { return d.introducedCount }

// diffJSON is the serialized shape of a Diff. The rollups are emitted for
// external consumers but are authoritative only as derived from Changes —
// UnmarshalJSON recomputes them so a round-trip can't smuggle in inconsistent
// totals.
type diffJSON struct {
	Changes         []PackageChange
	RemediatedCount int
	IntroducedCount int
	Totals          ChangeTotals
}

func (d Diff) MarshalJSON() ([]byte, error) {
	return json.Marshal(diffJSON{
		Changes:         d.changes,
		RemediatedCount: d.remediatedCount,
		IntroducedCount: d.introducedCount,
		Totals:          d.totals,
	})
}

func (d *Diff) UnmarshalJSON(b []byte) error {
	var x diffJSON
	if err := json.Unmarshal(b, &x); err != nil {
		return err
	}
	// rebuild through NewDiff so totals/counts derive from the changes rather
	// than trusting the serialized rollups.
	*d = NewDiff(x.Changes)
	return nil
}

// ChangeTotals is a per-kind count of package changes.
type ChangeTotals struct {
	Updated    int
	Downgraded int
	Added      int
	Removed    int
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

// Snapshot is the scan subpackage's output, consumed by Compare. Defined here
// so the core package owns the contract; scan produces it.
type Snapshot struct {
	Packages []Package
	Vulns    map[PackageKey][]Vulnerability // nil/empty when annotation disabled
}

// PackageKey is the identity used to index packages and vulnerabilities across
// the two snapshots.
type PackageKey struct{ Type, Name string }

// Key returns the PackageKey identity for a package.
func (p Package) Key() PackageKey {
	return PackageKey{Type: p.Type, Name: p.Name}
}
