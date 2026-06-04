package render

import (
	"sort"

	"github.com/anchore/chronicle/chronicle/dependency"
)

// ecosystemOrder is the canonical display order for the most common language
// ecosystems. Titles not listed here sort alphabetically after these.
var ecosystemOrder = []string{
	"Go", "JavaScript", "Python", "Java", "Ruby", "Rust", "PHP", ".NET",
	"C/C++", "Swift", "Dart", "Haskell", "Erlang/Elixir", "CocoaPods",
}

// ecosystemTitle is the grouping title for a change: the Ecosystem assigned at
// scan time from syft's package semantics, falling back to the raw syft type so
// a hand-built change (no Ecosystem) is never silently dropped from a group.
func ecosystemTitle(c dependency.PackageChange) string {
	if c.Ecosystem != "" {
		return c.Ecosystem
	}
	return c.Type
}

// EcosystemGroup is a set of changes that share an ecosystem title, used by
// encoders to render per-ecosystem subsections.
type EcosystemGroup struct {
	Title   string
	Changes []dependency.PackageChange
}

// GroupByEcosystem buckets changes by ecosystem title and returns the groups in
// canonical order (common languages first, then alphabetical). Within each group
// changes retain their incoming (type, name) order. The caller passes the
// changes it intends to enumerate (e.g. after VisibleChanges), so ecosystems
// with nothing to show never produce an empty group.
func GroupByEcosystem(changes []dependency.PackageChange) []EcosystemGroup {
	byTitle := make(map[string][]dependency.PackageChange)
	for _, c := range changes {
		title := ecosystemTitle(c)
		byTitle[title] = append(byTitle[title], c)
	}

	titles := make([]string, 0, len(byTitle))
	for title := range byTitle {
		titles = append(titles, title)
	}
	sort.Slice(titles, func(i, j int) bool {
		return ecosystemRank(titles[i]) < ecosystemRank(titles[j])
	})

	groups := make([]EcosystemGroup, 0, len(titles))
	for _, title := range titles {
		groups = append(groups, EcosystemGroup{Title: title, Changes: byTitle[title]})
	}
	return groups
}

// ecosystemRank yields a sort key that places canonical ecosystems first (in
// ecosystemOrder), then everything else alphabetically. The alphabetical tail
// is ordered by appending the title so equal-prefix ranks stay stable.
func ecosystemRank(title string) string {
	for i, t := range ecosystemOrder {
		if t == title {
			// pad the index so "10" sorts after "9"; canonical group sorts
			// before any alphabetical title (which is prefixed with "z").
			return "a" + string(rune('A'+i))
		}
	}
	return "z" + title
}
