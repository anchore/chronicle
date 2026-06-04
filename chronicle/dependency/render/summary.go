package render

import (
	"fmt"
	"strings"

	"github.com/anchore/chronicle/chronicle/dependency"
)

// SummaryLine is the rollup sentence above the grouped changes. It reports the
// full per-kind change totals — whole even when only a vulnerable subset is
// enumerated below (VisibleChanges filters at render time without touching the
// diff) — followed by the vulnerability remediated/introduced counts when
// present.
func SummaryLine(d dependency.Diff) string {
	t := d.Totals()

	noun := "dependency changes"
	if t.Total() == 1 {
		noun = "dependency change"
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%d %s", t.Total(), noun)
	if bd := kindBreakdown(t); bd != "" {
		fmt.Fprintf(&sb, " (%s)", bd)
	}
	sb.WriteString(".")

	if v := vulnSentence(d.RemediatedCount(), d.IntroducedCount()); v != "" {
		sb.WriteString(" " + v)
	}
	return sb.String()
}

// kindBreakdown renders the non-zero per-kind counts in canonical order.
func kindBreakdown(t dependency.ChangeTotals) string {
	var parts []string
	if t.Updated > 0 {
		parts = append(parts, fmt.Sprintf("%d updated", t.Updated))
	}
	if t.Downgraded > 0 {
		parts = append(parts, fmt.Sprintf("%d downgraded", t.Downgraded))
	}
	if t.Added > 0 {
		parts = append(parts, fmt.Sprintf("%d added", t.Added))
	}
	if t.Removed > 0 {
		parts = append(parts, fmt.Sprintf("%d removed", t.Removed))
	}
	return strings.Join(parts, ", ")
}

// vulnSentence renders the remediated/introduced clause, or "" when neither.
func vulnSentence(remediated, introduced int) string {
	switch {
	case remediated > 0 && introduced > 0:
		return fmt.Sprintf("%s remediated and %s introduced.", pluralVulns(remediated), pluralVulns(introduced))
	case remediated > 0:
		return fmt.Sprintf("%s remediated.", pluralVulns(remediated))
	case introduced > 0:
		return fmt.Sprintf("%s introduced.", pluralVulns(introduced))
	default:
		return ""
	}
}

// pluralVulns formats a vuln count with the correct singular/plural noun.
func pluralVulns(n int) string {
	if n == 1 {
		return "1 vulnerability"
	}
	return fmt.Sprintf("%d vulnerabilities", n)
}

// PackageCountLabel formats a per-kind package count for a section header. When
// the diff is being rendered as only-vulnerable, it clarifies that the count is
// the vuln-affected subset (distinct from the full totals in SummaryLine).
func (c *Config) PackageCountLabel(n int) string {
	base := pluralPackages(n)
	if c != nil && c.OnlyVulnerable {
		return base + " with vulnerability changes"
	}
	return base
}

// pluralPackages formats a package count with the correct singular/plural noun.
func pluralPackages(n int) string {
	if n == 1 {
		return "1 package"
	}
	return fmt.Sprintf("%d packages", n)
}
