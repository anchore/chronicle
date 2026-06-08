package dependency

// this file defines the seam between the pure dependency-diff core and the
// syft+grype machinery: the Scanner contract ComputeDiff drives, the inputs it
// passes, and the progress callbacks it emits. The concrete scanner lives in
// dependency/scan and is injected, mirroring how release injects a Summarizer,
// so the core never imports syft, grype, or any UI package.

import "context"

// Scanner turns a materialized source directory into a Snapshot: the resolved
// packages and, when the scanner was built to annotate, the vulnerabilities
// matched against them. The syft+grype implementation lives in dependency/scan;
// ComputeDiff drives two of these in parallel and never imports either library.
type Scanner interface {
	// Scan catalogs dir into packages and (when annotating) matches
	// vulnerabilities, returning the per-ref Snapshot. It reports its own
	// milestones through hooks. A non-nil error is fatal for the ref (the dir
	// could not be cataloged); a vulnerability-match failure instead degrades to
	// a packages-only Snapshot and is surfaced via hooks.OnMatchFailed.
	Scan(ctx context.Context, dir string, info SourceInfo, hooks ScanHooks) (Snapshot, error)
}

// SourceInfo names the materialized source so the scanner can derive a stable
// artifact ID from the project + ref rather than the throwaway tmpdir path. Name
// is the project (e.g. "anchore/chronicle"); Version is the ref.
type SourceInfo struct {
	Name    string
	Version string
}

// ScanHooks lets a Scanner report its milestones as they happen, so a caller can
// render live progress without the scanner knowing anything about UI. All fields
// are optional; the scanner nil-checks before calling.
type ScanHooks struct {
	// OnSourceID fires once the source is resolved, before cataloging begins,
	// with its stable ID — the caller uses it to attribute the live cataloging
	// progress to the right UI element.
	OnSourceID func(srcID string)
	// OnCataloged fires when cataloging completes, with the package count.
	OnCataloged func(packages int)
	// OnMatchStart fires when vulnerability matching begins (the shared DB is
	// ready). Only fires when annotating.
	OnMatchStart func()
	// OnMatched fires when matching completes, with the distinct-vulnerability count.
	OnMatched func(vulns int)
	// OnMatchFailed fires when DB load or matching fails; the scan degrades to
	// packages-only rather than failing the ref.
	OnMatchFailed func(err error)
}

// Observer receives per-ref progress during ComputeDiff so a caller can drive UI
// without ComputeDiff importing any UI package. ref is the diff endpoint the
// callback concerns ("since" or "until"). The zero value is a valid no-op; every
// field is optional and nil-checked before use.
type Observer struct {
	SourceResolved   func(ref, srcID string)
	RefCataloged     func(ref string, packages int)
	RefCatalogFailed func(ref string, err error)
	MatchStarted     func(ref string)
	RefMatched       func(ref string, vulns int)
	RefMatchFailed   func(ref string, err error)
}

func (o Observer) sourceResolved(ref, srcID string) {
	if o.SourceResolved != nil {
		o.SourceResolved(ref, srcID)
	}
}

func (o Observer) refCataloged(ref string, packages int) {
	if o.RefCataloged != nil {
		o.RefCataloged(ref, packages)
	}
}

func (o Observer) refCatalogFailed(ref string, err error) {
	if o.RefCatalogFailed != nil {
		o.RefCatalogFailed(ref, err)
	}
}

func (o Observer) matchStarted(ref string) {
	if o.MatchStarted != nil {
		o.MatchStarted(ref)
	}
}

func (o Observer) refMatched(ref string, vulns int) {
	if o.RefMatched != nil {
		o.RefMatched(ref, vulns)
	}
}

func (o Observer) refMatchFailed(ref string, err error) {
	if o.RefMatchFailed != nil {
		o.RefMatchFailed(ref, err)
	}
}
