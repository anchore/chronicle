package render

import (
	"regexp"
	"sort"
	"strings"

	"github.com/anchore/chronicle/chronicle/dependency"
)

// pseudoVersionTail matches the "<14-digit timestamp>-<12-char hash>" tail of a
// Go pseudo-version (e.g. the "20211214055906-6f57359322fd" in
// "v0.0.0-20211214055906-6f57359322fd").
var pseudoVersionTail = regexp.MustCompile(`\d{14}-([0-9a-f]{12})`)

// ShortenVersion compacts long version strings for display. Go pseudo-versions
// embed a 14-digit timestamp and a 12-char commit hash; those are collapsed to
// a 7-char short hash (keeping the base, e.g. "v0.0.0-6f57359"). Other versions
// are returned unchanged.
func ShortenVersion(v string) string {
	return pseudoVersionTail.ReplaceAllStringFunc(v, func(m string) string {
		hash := m[strings.LastIndex(m, "-")+1:]
		return hash[:7]
	})
}

// Backtick wraps a string in inline-code backticks. Pass it as the code-token
// renderer to VersionTransitionWith so each version is styled while the → arrow
// stays plain; markdown and slack share the same backtick syntax.
func Backtick(s string) string { return "`" + s + "`" }

// VersionTransitionWith renders a change's version movement for display:
// shortened "from → to" for updates/downgrades, or a single shortened version
// for additions (the new version) and removals (the version that left). Each
// (shortened) version token is rendered through code, so an encoder can wrap the
// versions in inline-code markup while the → arrow stays outside it (e.g.
// `v1.0.1` → `v1.0.2` rather than `v1.0.1 → v1.0.2`). A nil code yields bare
// version text.
func VersionTransitionWith(c dependency.PackageChange, code func(string) string) string {
	tok := func(v string) string {
		s := ShortenVersion(v)
		if code != nil {
			return code(s)
		}
		return s
	}
	switch c.Kind {
	case dependency.Updated, dependency.Downgraded:
		return tok(c.FromVersion) + " → " + tok(c.ToVersion)
	case dependency.Added:
		return tok(c.ToVersion)
	case dependency.Removed:
		return tok(c.FromVersion)
	}
	return ""
}

// VulnLinker renders a single vulnerability as display text. Encoders supply
// one to wrap the ID in their format's link syntax (using v.DataSource); a nil
// linker yields the bare ID.
type VulnLinker func(v dependency.Vulnerability) string

// VulnNoteWith summarizes a change's vulnerability effect as a short phrase (no
// surrounding markup): 🟢-flagged remediated IDs and any 🔴-flagged
// (re)introduced IDs. Returns "" when the change has no vulnerability impact (or
// was not annotated). Each vulnerability ID is rendered through link, so encoders
// can hyperlink CVE/GHSA IDs to their data source; a nil link yields bare IDs.
func VulnNoteWith(c dependency.PackageChange, link VulnLinker) string {
	if c.Vuln == nil {
		return ""
	}
	var parts []string
	if len(c.Vuln.Remediated) > 0 {
		// 🟢 = remediated (good), 🔴 = (re)introduced (bad).
		parts = append(parts, "🟢 remediated "+joinVulnIDs(c.Vuln.Remediated, link))
	}
	switch {
	case c.Kind == dependency.Downgraded && len(c.Vuln.Introduced) > 0:
		parts = append(parts, "🔴 reintroduces "+joinVulnIDs(c.Vuln.Introduced, link))
	case len(c.Vuln.Introduced) > 0:
		parts = append(parts, "🔴 introduces "+joinVulnIDs(c.Vuln.Introduced, link))
	}
	return strings.Join(parts, "; ")
}

// joinVulnIDs returns the vulnerabilities sorted by ID and comma-joined, each
// rendered through link when non-nil (else the bare ID). Sorting is by ID so
// ordering stays stable regardless of how the linker wraps the text.
func joinVulnIDs(vulns []dependency.Vulnerability, link VulnLinker) string {
	sorted := make([]dependency.Vulnerability, len(vulns))
	copy(sorted, vulns)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	ids := make([]string, len(sorted))
	for i, v := range sorted {
		if link != nil {
			ids[i] = link(v)
		} else {
			ids[i] = v.ID
		}
	}
	return strings.Join(ids, ", ")
}
