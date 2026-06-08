package dependency

import (
	"context"
	"fmt"

	"github.com/anchore/chronicle/chronicle/dependency/source"
	"github.com/anchore/chronicle/internal/log"
	"github.com/anchore/go-sync"
)

// Config carries everything ComputeDiff needs beyond the Scanner: the
// collaborators it drives (Target, Comparer, optional Observer) and the run
// parameters. It is deliberately free of any cmd or UI type — the worker maps its
// options onto this and adapts the Observer callbacks onto UI elements.
type Config struct {
	// Target materializes each ref into a directory the Scanner can read.
	Target source.Target
	// Comparer classifies a version change as an update or a downgrade; injected
	// so the core stays free of grype's ecosystem-aware version logic.
	Comparer VersionComparer
	// Observer receives live per-ref progress. Optional; the zero value is a no-op.
	Observer Observer

	SinceRef string
	UntilRef string

	// SourceName names the scanned project so syft derives stable artifact IDs
	// across the tmpdir each ref is materialized into. The caller supplies any
	// fallback (e.g. the repo dir name) before handing it here.
	SourceName string

	// Annotate runs vulnerability annotation on the diff. The injected Scanner
	// must have been built to match for this to populate anything.
	Annotate    bool
	MinSeverity string
}

// ComputeDiff diffs the dependency graph between cfg.SinceRef and cfg.UntilRef.
// It materializes and scans both refs in parallel (sharing the scanner's
// vulnerability DB so the two sides never drift), compares the resulting
// snapshots, and — when cfg.Annotate is set — attributes the vulnerabilities each
// change remediated or introduced. It drives no UI directly: progress flows
// through cfg.Observer, which the caller adapts onto its own UI.
func ComputeDiff(ctx context.Context, scanner Scanner, cfg Config) (*Diff, error) {
	// run both refs end-to-end in parallel: each materializes, catalogs, and
	// (when annotating) matches against the scanner's shared DB. A bounded
	// executor drives the fan-out; sync.Collect joins the snapshots and errors.
	var since, until Snapshot
	jobs := []refJob{
		{ref: cfg.SinceRef, label: "since", out: &since},
		{ref: cfg.UntilRef, label: "until", out: &until},
	}
	ctx = sync.SetContextExecutor(ctx, sync.ExecutorDefault, sync.NewExecutor(len(jobs)))
	err := sync.Collect(&ctx, sync.ExecutorDefault, sync.ToSeq(jobs),
		func(j refJob) (Snapshot, error) {
			return scanRef(ctx, scanner, cfg, j.label, j.ref)
		},
		func(j refJob, snap Snapshot) {
			*j.out = snap
		})
	if err != nil {
		return nil, err
	}

	diff := Compare(since, until, cfg.Comparer)
	if cfg.Annotate {
		// Annotate is pure: it returns a new diff with each change's Vuln delta
		// populated and the rollup counts recomputed. Display-time concerns
		// (only-vulnerable filtering, grouping) live in render and the encoders,
		// so the diff handed back is always the complete result.
		diff = Annotate(diff, since, until, AnnotateConfig{MinSeverity: cfg.MinSeverity})
	}
	return &diff, nil
}

// refJob is one ref's unit of work for the parallel fan-out: which ref to scan,
// the label it reports progress under ("since"/"until"), and where to store the
// resulting snapshot.
type refJob struct {
	ref   string
	label string
	out   *Snapshot
}

// scanRef materializes a single ref and scans it, bridging the scanner's
// milestone hooks onto the ref-tagged Observer. A materialize or catalog failure
// is fatal for the ref; a vulnerability-match failure degrades to packages-only
// inside the Scanner and never reaches here as an error.
func scanRef(ctx context.Context, scanner Scanner, cfg Config, label, ref string) (Snapshot, error) {
	obs := cfg.Observer
	dir, cleanup, err := cfg.Target.Materialize(ctx, ref)
	if err != nil {
		obs.refCatalogFailed(label, err)
		return Snapshot{}, fmt.Errorf("unable to materialize ref %q: %w", ref, err)
	}
	defer func() {
		if cerr := cleanup(); cerr != nil {
			log.WithFields("error", cerr).Trace("unable to clean up materialized dependency source")
		}
	}()

	snap, err := scanner.Scan(ctx, dir, SourceInfo{Name: cfg.SourceName, Version: ref}, ScanHooks{
		OnSourceID:    func(id string) { obs.sourceResolved(label, id) },
		OnCataloged:   func(n int) { obs.refCataloged(label, n) },
		OnMatchStart:  func() { obs.matchStarted(label) },
		OnMatched:     func(n int) { obs.refMatched(label, n) },
		OnMatchFailed: func(e error) { obs.refMatchFailed(label, e) },
	})
	if err != nil {
		obs.refCatalogFailed(label, err)
		return Snapshot{}, fmt.Errorf("unable to scan ref %q: %w", ref, err)
	}
	return snap, nil
}
