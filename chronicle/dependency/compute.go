package dependency

import (
	"context"
	"fmt"

	"github.com/anchore/chronicle/chronicle/dependency/source"
	"github.com/anchore/chronicle/internal/log"
	"github.com/anchore/go-sync"
)

// Config carries everything ComputeDiff needs beyond the Scanner: the
// collaborators it drives (Target, Comparer) and the run parameters. It holds no
// UI, status, or bus type — progress is published by the scanner from deep
// inside, and the per-ref figures the UI needs come back on the Result.
type Config struct {
	// Target materializes each ref into a directory the Scanner can read.
	Target source.Target
	// Comparer classifies a version change as an update or a downgrade; injected
	// so the core stays free of grype's ecosystem-aware version logic.
	Comparer VersionComparer

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

// Result is the outcome of a ComputeDiff: the diff itself plus each ref's raw
// catalog/match figures, returned as data so the caller can render them (resolve
// its UI leaves) without ComputeDiff knowing anything about UI.
type Result struct {
	Diff  Diff
	Since RefStat
	Until RefStat
}

// RefStat is one ref's raw scan figures: how many packages it cataloged, how
// many distinct vulnerabilities matched, and whether annotation was requested
// but matching did not complete for it.
type RefStat struct {
	Packages    int
	Vulns       int
	VulnsFailed bool
}

// ComputeDiff diffs the dependency graph between cfg.SinceRef and cfg.UntilRef.
// It materializes and scans both refs in parallel (sharing the scanner's
// vulnerability DB so the two sides never drift), compares the resulting
// snapshots, and — when cfg.Annotate is set — attributes the vulnerabilities each
// change remediated or introduced. It drives no UI: the scanner publishes live
// progress to the bus itself, and the per-ref figures come back on the Result.
func ComputeDiff(ctx context.Context, scanner Scanner, cfg Config) (*Result, error) {
	// run both refs end-to-end in parallel: each materializes, catalogs, and
	// (when annotating) matches against the scanner's shared DB. A bounded
	// executor drives the fan-out; sync.Collect joins the snapshots and errors.
	var since, until Snapshot
	jobs := []refJob{
		{ref: cfg.SinceRef, out: &since},
		{ref: cfg.UntilRef, out: &until},
	}
	ctx = sync.SetContextExecutor(ctx, sync.ExecutorDefault, sync.NewExecutor(len(jobs)))
	err := sync.Collect(&ctx, sync.ExecutorDefault, sync.ToSeq(jobs),
		func(j refJob) (Snapshot, error) {
			return scanRef(ctx, scanner, cfg, j.ref)
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

	return &Result{
		Diff:  diff,
		Since: refStat(since, cfg.Annotate),
		Until: refStat(until, cfg.Annotate),
	}, nil
}

// refStat derives a ref's display figures from its snapshot. When annotation was
// requested but the snapshot has no vuln map, matching did not complete for that
// ref (the scanner logs the cause and degrades to packages-only).
func refStat(s Snapshot, annotate bool) RefStat {
	return RefStat{
		Packages:    len(s.Packages),
		Vulns:       s.DistinctVulns(),
		VulnsFailed: annotate && s.Vulns == nil,
	}
}

// refJob is one ref's unit of work for the parallel fan-out: which ref to scan
// and where to store the resulting snapshot.
type refJob struct {
	ref string
	out *Snapshot
}

// scanRef materializes a single ref and scans it. A materialize or catalog
// failure is fatal for the ref; a vulnerability-match failure degrades to
// packages-only inside the Scanner and never reaches here as an error.
func scanRef(ctx context.Context, scanner Scanner, cfg Config, ref string) (Snapshot, error) {
	dir, cleanup, err := cfg.Target.Materialize(ctx, ref)
	if err != nil {
		return Snapshot{}, fmt.Errorf("unable to materialize ref %q: %w", ref, err)
	}
	defer func() {
		if cerr := cleanup(); cerr != nil {
			log.WithFields("error", cerr).Trace("unable to clean up materialized dependency source")
		}
	}()

	snap, err := scanner.Scan(ctx, dir, SourceInfo{Name: cfg.SourceName, Version: ref})
	if err != nil {
		return Snapshot{}, fmt.Errorf("unable to scan ref %q: %w", ref, err)
	}
	return snap, nil
}
