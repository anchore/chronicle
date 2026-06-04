package dependency

import (
	"sort"
	"strings"
)

// PackageVulns associates a package with a set of its vulnerabilities. It backs
// the "remaining" listing — carried-over vulnerabilities spanning every package
// in the latest scan, including packages whose version never changed (which the
// diff's Changes never mention) — so it cannot hang off a PackageChange the way
// VulnDelta does.
type PackageVulns struct {
	Package Package
	Vulns   []Vulnerability
}

// VulnDelta is the vulnerability attribution for one change: what the change
// remediated (gone at until) and introduced (new at until). It is populated by
// annotate; a nil *VulnDelta on a PackageChange means "not annotated", distinct
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

// annotateConfig controls which vulnerability annotations annotate applies.
type annotateConfig struct {
	// MinSeverity is the minimum severity required for a vulnerability to appear
	// in a change's Vuln delta. "" means no filtering (include all). Valid values
	// (case-insensitive): negligible, low, medium, high, critical.
	MinSeverity string
}

// annotate returns a copy of d with each change's Vuln delta populated from the
// diff's own since/until scan vulnerability maps (and the rollup counts
// recomputed). It is pure: the input diff is not modified, so it cannot be
// annotated twice by accident or left half-populated. A change's Vuln is always
// non-nil after annotation — empty when the change had no vulnerability impact —
// which distinguishes an annotated-but-clean change from an unannotated one
// (nil). The caller decides when to annotate (ComputeDiff does so only when a
// ref was actually vuln-matched).
func annotate(d Diff, cfg annotateConfig) Diff {
	minRank := severityRank(cfg.MinSeverity)

	annotated := make([]PackageChange, len(d.Changes))
	for i, ch := range d.Changes {
		key := ch.key()

		var sinceVulns, untilVulns []Vulnerability
		if d.Since.Vulns != nil {
			sinceVulns = d.Since.Vulns[key]
		}
		if d.Until.Vulns != nil {
			untilVulns = d.Until.Vulns[key]
		}

		var delta VulnDelta
		switch ch.Kind {
		case Removed:
			// all of the package's since vulns are remediated; none at until
			delta.Remediated = filterBySeverity(sinceVulns, minRank)
		case Added:
			// all of the package's until vulns are introduced; none at since
			delta.Introduced = filterBySeverity(untilVulns, minRank)
		default:
			// Updated / Downgraded: set difference
			delta.Remediated = filterBySeverity(setDifference(sinceVulns, untilVulns), minRank)
			delta.Introduced = filterBySeverity(setDifference(untilVulns, sinceVulns), minRank)
		}

		ch.Vuln = &delta
		annotated[i] = ch
	}

	// NewDiff re-derives the totals and unique-ID counts from the annotated
	// changes, keeping every rollup consistent with the per-change deltas; carry
	// the compared scans through to the rebuilt diff.
	out := NewDiff(annotated)
	out.Since = d.Since
	out.Until = d.Until
	// remaining spans the full latest scan (including unchanged packages), so it
	// is derived from the scans here rather than from the per-change deltas.
	out.Remaining = remainingVulns(out, minRank)
	out.RemainingCount = countUniqueIDsIn(out.Remaining)
	return out
}

// remainingVulns collects the carried-over vulnerabilities for every package in
// the latest scan: those present at both since and until (matched by ID per
// package), which are by construction neither remediated (gone at until) nor
// introduced (absent at since). Unlike the per-change deltas it spans packages
// whose version never moved, so it is the standing vulnerability burden the
// release did not clear. Severity-filtered like the per-change deltas, and
// sorted deterministically for stable output. Empty when until was not matched.
func remainingVulns(d Diff, minRank int) []PackageVulns {
	if d.Until.Vulns == nil {
		return nil
	}
	untilPkgs := indexPackages(d.Until.Packages)

	out := make([]PackageVulns, 0, len(d.Until.Vulns))
	for key, untilVulns := range d.Until.Vulns {
		var sinceVulns []Vulnerability
		if d.Since.Vulns != nil {
			sinceVulns = d.Since.Vulns[key]
		}
		carried := filterBySeverity(intersectByID(untilVulns, sinceVulns), minRank)
		if len(carried) == 0 {
			continue
		}
		out = append(out, PackageVulns{Package: untilPkgs[key], Vulns: carried})
	}

	sortPackageVulns(out)
	return out
}

// intersectByID returns the vulns from a whose ID also appears in b — the
// complement of setDifference. It picks a package's carried-over vulnerabilities
// (present in both scans) from the until side, so the records reflect the latest
// state.
func intersectByID(a, b []Vulnerability) []Vulnerability {
	bIDs := make(map[string]bool, len(b))
	for _, v := range b {
		bIDs[v.ID] = true
	}
	var result []Vulnerability
	for _, v := range a {
		if bIDs[v.ID] {
			result = append(result, v)
		}
	}
	return result
}

// sortPackageVulns orders the listing deterministically: packages by Type then
// Name (matching sortChanges), and each package's vulns by ID.
func sortPackageVulns(pvs []PackageVulns) {
	for i := range pvs {
		vs := pvs[i].Vulns
		sort.Slice(vs, func(a, b int) bool { return vs[a].ID < vs[b].ID })
	}
	sort.Slice(pvs, func(i, j int) bool {
		if pvs[i].Package.Type != pvs[j].Package.Type {
			return pvs[i].Package.Type < pvs[j].Package.Type
		}
		return pvs[i].Package.Name < pvs[j].Package.Name
	})
}

// countUniqueIDsIn counts distinct vulnerability IDs across a PackageVulns
// listing, so the same CVE on several packages counts once (mirroring how
// countUniqueIDs rolls up the per-change deltas).
func countUniqueIDsIn(pvs []PackageVulns) int {
	seen := make(map[string]bool)
	for _, pv := range pvs {
		for _, v := range pv.Vulns {
			seen[v.ID] = true
		}
	}
	return len(seen)
}

// setDifference returns vulns from a that are not present in b (by ID).
func setDifference(a, b []Vulnerability) []Vulnerability {
	bIDs := make(map[string]bool, len(b))
	for _, v := range b {
		bIDs[v.ID] = true
	}
	var result []Vulnerability
	for _, v := range a {
		if !bIDs[v.ID] {
			result = append(result, v)
		}
	}
	return result
}

// filterBySeverity retains only vulns whose severity is at or above minRank.
// When minRank is 0 (MinSeverity == ""), all vulns pass through.
func filterBySeverity(vulns []Vulnerability, minRank int) []Vulnerability {
	if minRank == 0 {
		return vulns
	}
	var out []Vulnerability
	for _, v := range vulns {
		if severityRank(v.Severity) >= minRank {
			out = append(out, v)
		}
	}
	return out
}

// ValidSeverity reports whether s is a recognized severity name
// (case-insensitive), or empty (meaning "no filter"). Callers validate a
// user-supplied MinSeverity with this so a typo fails loudly rather than being
// silently treated as "no filter" by filterBySeverity.
func ValidSeverity(s string) bool {
	return strings.TrimSpace(s) == "" || severityRank(s) > 0
}

// severityRank maps a severity string to a numeric rank for comparison. Unknown
// or empty severity sorts lowest (0). The scale is 1–5: negligible through critical.
func severityRank(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "negligible":
		return 1
	case "low":
		return 2
	case "medium":
		return 3
	case "high":
		return 4
	case "critical":
		return 5
	default:
		return 0
	}
}
