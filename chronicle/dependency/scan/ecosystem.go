package scan

import "github.com/anchore/syft/syft/pkg"

// ecosystemTitle derives the human-friendly grouping title for a catalogued
// package, used to bucket the dependency diff into per-ecosystem subsections.
//
// syft is the single source of truth: the package's Language (assigned by the
// cataloger) drives the title for language ecosystems, so new package types
// within a language (e.g. another Java build format) flow through with no change
// here. A handful of ecosystems key off Type instead — either because they have
// no language (OS distros, CI, binaries) or because their syft Language is the
// implementation language rather than the ecosystem we group by (terraform→go,
// wordpress→php, cocoapods→swift). Anything unrecognized falls back to the raw
// syft string, so a newly added type/language is grouped (just un-prettified)
// rather than silently dropped.
func ecosystemTitle(p pkg.Package) string {
	if t, ok := typeTitle[p.Type]; ok {
		return t
	}
	if t, ok := languageTitle[p.Language]; ok {
		return t
	}
	if p.Language != pkg.UnknownLanguage {
		return string(p.Language)
	}
	return string(p.Type)
}

// typeTitle overrides the grouping title by package type. It takes precedence
// over the package's syft Language, both to supply titles for the language-less
// ecosystems (OS distros, CI, binaries) and to override the few whose Language
// is the implementation language rather than the ecosystem we want to group by.
var typeTitle = map[pkg.Type]string{
	pkg.TerraformPkg:            "Terraform",
	pkg.WordpressPluginPkg:      "WordPress",
	pkg.CocoapodsPkg:            "CocoaPods",
	pkg.DebPkg:                  "Debian",
	pkg.ApkPkg:                  "Alpine",
	pkg.AlpmPkg:                 "Arch Linux",
	pkg.RpmPkg:                  "RPM",
	pkg.PortagePkg:              "Gentoo",
	pkg.NixPkg:                  "Nix",
	pkg.GithubActionPkg:         "GitHub Actions",
	pkg.GithubActionWorkflowPkg: "GitHub Actions",
	pkg.BinaryPkg:               "Binary",
}

// languageTitle maps syft's language enum to its display title. Languages absent
// here fall back to the raw language string, so a newly added syft language is
// still grouped, just un-prettified.
var languageTitle = map[pkg.Language]string{
	pkg.Go:         "Go",
	pkg.JavaScript: "JavaScript",
	pkg.Python:     "Python",
	pkg.Java:       "Java",
	pkg.Ruby:       "Ruby",
	pkg.Rust:       "Rust",
	pkg.PHP:        "PHP",
	pkg.Dotnet:     ".NET",
	pkg.CPP:        "C/C++",
	pkg.Swift:      "Swift",
	pkg.Dart:       "Dart",
	pkg.Haskell:    "Haskell",
	pkg.Elixir:     "Erlang/Elixir",
	pkg.Erlang:     "Erlang/Elixir",
	pkg.OCaml:      "OCaml",
	pkg.Lua:        "Lua",
	pkg.R:          "R",
	pkg.Swipl:      "SWI-Prolog",
}
