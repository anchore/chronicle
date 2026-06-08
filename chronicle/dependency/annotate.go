package dependency

import "strings"

// AnnotateConfig controls which vulnerability annotations Annotate applies.
type AnnotateConfig struct {
	// MinSeverity is the minimum severity required for a vulnerability to appear
	// in a change's Vuln delta. "" means no filtering (include all). Valid values
	// (case-insensitive): negligible, low, medium, high, critical.
	MinSeverity string
}

// Annotate returns a copy of d with each change's Vuln delta populated from the
// since/until vulnerability maps (and the rollup counts recomputed). It is pure:
// the input diff is not modified, so it cannot be annotated twice by accident or
// left half-populated. A change's Vuln is always non-nil after annotation —
// empty when the change had no vulnerability impact — which distinguishes an
// annotated-but-clean change from an unannotated one (nil).
func Annotate(d Diff, since, until Snapshot, cfg AnnotateConfig) Diff {
	minRank := severityRank(cfg.MinSeverity)

	annotated := make([]PackageChange, len(d.changes))
	for i, ch := range d.changes {
		key := ch.Key()

		var sinceVulns, untilVulns []Vulnerability
		if since.Vulns != nil {
			sinceVulns = since.Vulns[key]
		}
		if until.Vulns != nil {
			untilVulns = until.Vulns[key]
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
	// changes, keeping every rollup consistent with the per-change deltas.
	return NewDiff(annotated)
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
