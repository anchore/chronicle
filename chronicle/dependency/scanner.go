package dependency

// this file defines the seam between the pure dependency-diff core and the
// syft+grype machinery: the Scanner contract ComputeDiff drives. The concrete
// scanner lives in dependency/scan and is injected, mirroring how release injects
// a Summarizer, so the core never imports syft, grype, git, or any UI package.
// Materializing a ref onto disk is the scanner's own concern (it owns the source
// target), and progress is too — it publishes to the bus from deep inside, not
// back through this interface.

import "context"

// Scanner turns a source ref into a Scan: the resolved packages and, when the
// scanner was built to annotate, the vulnerabilities matched against them. The
// scanner owns everything behind the ref — materializing it onto disk, cataloging
// with syft, and matching with grype. The implementation lives in dependency/scan;
// ComputeDiff drives two of these in parallel and never imports either library.
type Scanner interface {
	// Scan materializes ref, catalogs it into packages, and (when the scanner was
	// built to match) attributes vulnerabilities, returning the per-ref Scan.
	// A non-nil error is fatal for the ref (it could not be materialized or
	// cataloged); a vulnerability-match failure instead degrades to a
	// packages-only Scan (nil Vulns). A successful match always yields a
	// non-nil Vulns map (empty if none were found), so the map's presence tells
	// ComputeDiff whether to attribute — no separate intent flag is needed.
	Scan(ctx context.Context, ref string) (Scan, error)
}

// Scan is the Scanner's output, consumed by Compare. Defined here so the
// core package owns the contract; the scan subpackage produces it.
type Scan struct {
	Packages []Package
	Vulns    map[PackageKey][]Vulnerability // nil/empty when annotation disabled
}

// DistinctPackages counts the distinct packages by identity (PackageKey, i.e.
// Type+Name) — distinct because syft can catalog the same package more than once
// (e.g. found via multiple paths). This matches how Compare treats packages,
// which index by key, so the count never over-reports versus the actual diff.
func (s Scan) DistinctPackages() int {
	keys := make(map[PackageKey]struct{}, len(s.Packages))
	for _, p := range s.Packages {
		keys[p.key()] = struct{}{}
	}
	return len(keys)
}

// DistinctVulns counts the distinct vulnerability IDs matched across the
// scan's packages — distinct because a single CVE can match several
// packages. Returns 0 when the scan was not matched.
func (s Scan) DistinctVulns() int {
	ids := make(map[string]struct{})
	for _, vs := range s.Vulns {
		for _, v := range vs {
			ids[v.ID] = struct{}{}
		}
	}
	return len(ids)
}
