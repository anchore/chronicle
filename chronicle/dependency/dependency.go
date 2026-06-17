package dependency

// Package dependency holds the pure data types and algorithms for the
// dependency-diff feature. It deliberately imports no syft/grype code so the
// compare/annotate logic stays unit-testable with fixtures. The syft+grype
// integration lives in the dependency/scan subpackage; presentation (summary
// prose, version/vuln formatting, ecosystem grouping) lives with the output
// layer in release/render — not here.
//
// This file holds only the atomic vocabulary shared across the whole package:
// a Package, its identity (PackageKey), and a Vulnerability. The diff result
// model lives in diff.go, the Scanner contract and its Scan output in
// scanner.go, and vulnerability annotation in annotate.go.

// Package is a single resolved dependency at a point in time. Identity is the
// (Type, Name) pair; Version distinguishes points in time. Type is the raw syft
// package type string (e.g. "go-module"); turning that into a human-friendly
// ecosystem label is a presentation concern and lives in release/render.
type Package struct {
	Name    string // identity (with Type)
	Version string
	Type    string // syft package type string, e.g. "go-module", "npm"
}

// key returns the PackageKey identity for a package.
func (p Package) key() PackageKey {
	return PackageKey{Type: p.Type, Name: p.Name}
}

// PackageKey is the identity used to index packages and vulnerabilities across
// the two scans. It is deliberately version-less: identity is (Type, Name),
// and Version is the axis the diff moves along — so the same package at two refs
// shares a key and is matched as an update rather than a remove+add.
type PackageKey struct{ Type, Name string }

// Vulnerability is a single match against a package at a point in time. It is
// always reached through the package it affects (the Scan.Vulns map key, or
// a PackageChange), so it carries no back-reference to that package.
type Vulnerability struct {
	ID         string // CVE-…, GHSA-…
	Severity   string
	FixState   string // fixed / not-fixed / unknown
	DataSource string // grype's primary reference URL for the ID; "" if none. Encoders use it to make the ID clickable.
}
