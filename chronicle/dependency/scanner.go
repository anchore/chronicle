package dependency

// this file defines the seam between the pure dependency-diff core and the
// syft+grype machinery: the Scanner contract ComputeDiff drives and the inputs
// it passes. The concrete scanner lives in dependency/scan and is injected,
// mirroring how release injects a Summarizer, so the core never imports syft,
// grype, or any UI package. Progress is the scanner's own concern — it publishes
// to the bus from deep inside, not back through this interface.

import "context"

// Scanner turns a materialized source directory into a Snapshot: the resolved
// packages and, when the scanner was built to annotate, the vulnerabilities
// matched against them. The syft+grype implementation lives in dependency/scan;
// ComputeDiff drives two of these in parallel and never imports either library.
type Scanner interface {
	// Scan catalogs dir into packages and (when annotating) matches
	// vulnerabilities, returning the per-ref Snapshot. A non-nil error is fatal
	// for the ref (the dir could not be cataloged); a vulnerability-match failure
	// instead degrades to a packages-only Snapshot (nil Vulns).
	Scan(ctx context.Context, dir string, info SourceInfo) (Snapshot, error)
}

// SourceInfo names the materialized source so the scanner can derive a stable
// artifact ID from the project + ref rather than the throwaway tmpdir path. Name
// is the project (e.g. "anchore/chronicle"); Version is the ref. The scanner
// also uses Version to attribute its live cataloging progress to the right UI
// element on the bus.
type SourceInfo struct {
	Name    string
	Version string
}
