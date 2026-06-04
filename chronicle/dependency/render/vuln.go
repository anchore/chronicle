package render

import (
	"sort"

	"github.com/anchore/chronicle/chronicle/dependency"
)

// VulnListing is one vulnerability aggregated across every change that shares
// its ID, for the flat vulnerabilities rollup at the top of the dependencies
// section. Packages are the affected package names (sorted, deduped) — the
// vuln-centric inverse of the per-package annotations shown further down.
type VulnListing struct {
	ID         string
	Severity   string
	DataSource string // grype's primary reference URL; "" if none
	Packages   []string
}

// RemediatedVulns returns every remediated vulnerability in the diff as a flat,
// ID-sorted listing: one entry per unique ID, with all affected packages
// collected. Its length matches Diff.RemediatedCount.
func RemediatedVulns(d dependency.Diff) []VulnListing {
	return aggregateVulns(d.Changes(), func(v *dependency.VulnDelta) []dependency.Vulnerability { return v.Remediated })
}

// IntroducedVulns is RemediatedVulns for introduced (and, on downgrades,
// reintroduced) vulnerabilities; its length matches Diff.IntroducedCount.
func IntroducedVulns(d dependency.Diff) []VulnListing {
	return aggregateVulns(d.Changes(), func(v *dependency.VulnDelta) []dependency.Vulnerability { return v.Introduced })
}

// aggregateVulns folds the picked vulnerabilities across all changes into per-ID
// listings, attributing each to the changed package it appeared on. The same ID
// across multiple packages collapses to a single listing that names them all.
func aggregateVulns(changes []dependency.PackageChange, pick func(*dependency.VulnDelta) []dependency.Vulnerability) []VulnListing {
	byID := make(map[string]*VulnListing)
	pkgs := make(map[string]map[string]struct{}) // id -> set of affected packages
	for _, c := range changes {
		if c.Vuln == nil {
			continue
		}
		for _, v := range pick(c.Vuln) {
			if _, ok := byID[v.ID]; !ok {
				byID[v.ID] = &VulnListing{ID: v.ID, Severity: v.Severity, DataSource: v.DataSource}
				pkgs[v.ID] = make(map[string]struct{})
			}
			pkgs[v.ID][c.Name] = struct{}{}
		}
	}

	out := make([]VulnListing, 0, len(byID))
	for id, l := range byID {
		for p := range pkgs[id] {
			l.Packages = append(l.Packages, p)
		}
		sort.Strings(l.Packages)
		out = append(out, *l)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
