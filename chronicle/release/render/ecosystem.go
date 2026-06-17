package render

import (
	"sort"

	"github.com/anchore/chronicle/chronicle/dependency"
)

// ecosystemOrder is the canonical display order for the most common ecosystems.
// Titles not listed here sort alphabetically after these.
var ecosystemOrder = []string{
	"Go", "JavaScript", "Python", "Java", "Ruby", "Rust", "PHP", ".NET",
	"C/C++", "Swift", "Dart", "Haskell", "Erlang/Elixir", "CocoaPods",
}

// typeTitle maps a raw syft package type to a human-friendly ecosystem label.
// Deriving a friendly grouping name from the type is a presentation concern, so
// it lives here rather than in the dependency core (which stays type-only).
// Several types intentionally collapse onto one label (e.g. the JVM and
// Erlang/Elixir families) so related packages group together; an unmapped type
// falls back to its raw string so nothing is dropped from a group.
var typeTitle = map[string]string{
	"go-module":              "Go",
	"npm":                    "JavaScript",
	"python":                 "Python",
	"java-archive":           "Java",
	"jenkins-plugin":         "Java",
	"graalvm-native-image":   "Java",
	"gem":                    "Ruby",
	"rust-crate":             "Rust",
	"php-composer":           "PHP",
	"dotnet":                 ".NET",
	"conan":                  "C/C++",
	"pod":                    "CocoaPods",
	"swift":                  "Swift",
	"dart-pub":               "Dart",
	"hackage":                "Haskell",
	"hex":                    "Erlang/Elixir",
	"erlang-otp":             "Erlang/Elixir",
	"lua-rocks":              "Lua",
	"swiplpack":              "SWI-Prolog",
	"terraform":              "Terraform",
	"wordpress-plugin":       "WordPress",
	"deb":                    "Debian",
	"apk":                    "Alpine",
	"alpm":                   "Arch Linux",
	"rpm":                    "RPM",
	"portage":                "Gentoo",
	"nix":                    "Nix",
	"github-action":          "GitHub Actions",
	"github-action-workflow": "GitHub Actions",
	"binary":                 "Binary",
}

// ecosystemTitle is the grouping label for a change, derived from its syft
// package type. Unmapped types fall back to the raw type string so a change is
// never silently dropped from a group.
func ecosystemTitle(c dependency.PackageChange) string {
	if t, ok := typeTitle[c.Type]; ok {
		return t
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
