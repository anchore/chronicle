package render

import (
	"sort"

	"github.com/anchore/chronicle/chronicle/dependency"
)

// nonLanguageTitle maps raw syft package types that are NOT language ecosystems
// (OS distros, infra, build artifacts) to a friendly grouping label. Language
// ecosystems are handled by dependency.Ecosystem instead, so this table holds
// only what the enum deliberately leaves out. An unmapped type falls back to its
// raw string so a change is never dropped from a group.
var nonLanguageTitle = map[string]string{
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
// package type: a known language ecosystem resolves through dependency.Ecosystem
// (the single source of truth for those labels), otherwise a non-language type
// falls back to its friendly label, and finally to the raw type so a change is
// never silently dropped from a group.
func ecosystemTitle(c dependency.PackageChange) string {
	if e, ok := dependency.ParseEcosystem(c.Type); ok {
		return e.Label()
	}
	if t, ok := nonLanguageTitle[c.Type]; ok {
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

// languageRank is the canonical display position of each language-ecosystem
// label, derived once from dependency.Ecosystems so the order has a single
// source of truth.
var languageRank = func() map[string]int {
	m := make(map[string]int)
	for i, e := range dependency.Ecosystems() {
		m[e.Label()] = i
	}
	return m
}()

// ecosystemRank yields a sort key that places canonical language ecosystems
// first (in dependency.Ecosystems order), then everything else alphabetically.
// The alphabetical tail is ordered by appending the title so equal-prefix ranks
// stay stable.
func ecosystemRank(title string) string {
	if i, ok := languageRank[title]; ok {
		// pad the index so "10" sorts after "9"; a canonical group sorts before
		// any alphabetical title (which is prefixed with "z").
		return "a" + string(rune('A'+i))
	}
	return "z" + title
}
