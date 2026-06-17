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
	return aggregateVulns(d.Changes, func(v *dependency.VulnDelta) []dependency.Vulnerability { return v.Remediated })
}

// IntroducedVulns is RemediatedVulns for introduced (and, on downgrades,
// reintroduced) vulnerabilities; its length matches Diff.IntroducedCount.
func IntroducedVulns(d dependency.Diff) []VulnListing {
	return aggregateVulns(d.Changes, func(v *dependency.VulnDelta) []dependency.Vulnerability { return v.Introduced })
}

// RemainingVulns returns every carried-over (remaining) vulnerability — present
// at both since and until, across all packages in the latest scan — as a flat,
// ID-sorted listing, one entry per unique ID with its affected packages. Unlike
// the two above it reads Diff.Remaining (which the core attributes from the full
// scans, spanning unchanged packages) rather than the per-change deltas. Its
// length matches Diff.RemainingCount.
func RemainingVulns(d dependency.Diff) []VulnListing {
	acc := newVulnAccumulator()
	for _, pv := range d.Remaining {
		for _, v := range pv.Vulns {
			acc.add(v, pv.Package.Name)
		}
	}
	return acc.listings()
}

// aggregateVulns folds the picked vulnerabilities across all changes into per-ID
// listings, attributing each to the changed package it appeared on. The same ID
// across multiple packages collapses to a single listing that names them all.
func aggregateVulns(changes []dependency.PackageChange, pick func(*dependency.VulnDelta) []dependency.Vulnerability) []VulnListing {
	acc := newVulnAccumulator()
	for _, c := range changes {
		if c.Vuln == nil {
			continue
		}
		for _, v := range pick(c.Vuln) {
			acc.add(v, c.Name)
		}
	}
	return acc.listings()
}

// vulnAccumulator folds vulnerabilities into per-ID listings, collecting the
// distinct package names each ID affects. Both the change-based rollups
// (remediated/introduced) and the remaining rollup build on it, so the three
// share one definition of "collapse by ID, name the packages, sort".
type vulnAccumulator struct {
	byID map[string]*VulnListing
	pkgs map[string]map[string]struct{} // id -> set of affected packages
}

func newVulnAccumulator() *vulnAccumulator {
	return &vulnAccumulator{
		byID: make(map[string]*VulnListing),
		pkgs: make(map[string]map[string]struct{}),
	}
}

func (a *vulnAccumulator) add(v dependency.Vulnerability, pkg string) {
	if _, ok := a.byID[v.ID]; !ok {
		a.byID[v.ID] = &VulnListing{ID: v.ID, Severity: v.Severity, DataSource: v.DataSource}
		a.pkgs[v.ID] = make(map[string]struct{})
	}
	a.pkgs[v.ID][pkg] = struct{}{}
}

func (a *vulnAccumulator) listings() []VulnListing {
	out := make([]VulnListing, 0, len(a.byID))
	for id, l := range a.byID {
		for p := range a.pkgs[id] {
			l.Packages = append(l.Packages, p)
		}
		sort.Strings(l.Packages)
		out = append(out, *l)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}
