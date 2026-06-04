package report

// Package report orchestrates the dependency-diff feature end to end:
// materialize two source refs, scan each into a Snapshot, compare them, and
// (optionally) annotate the diff with vulnerability attribution. It lives in
// its own package rather than in `dependency` because it imports
// dependency/scan (which itself imports dependency) — keeping Run here avoids
// an import cycle while leaving the core dependency package library-free.

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/anchore/chronicle/chronicle/dependency"
	"github.com/anchore/chronicle/chronicle/dependency/scan"
	"github.com/anchore/chronicle/chronicle/dependency/source"
	"github.com/anchore/chronicle/chronicle/event"
	"github.com/anchore/chronicle/internal/bus"
	"github.com/anchore/chronicle/internal/log"
	"github.com/anchore/go-sync"
)

// Config controls a Run. It is deliberately decoupled from the cmd options
// type so the core feature does not import the CLI layer; the worker maps its
// options.Dependencies onto this.
type Config struct {
	Ecosystems              []string // syft cataloger selection expressions (e.g. ["language"], ["go"])
	ExcludePaths            []string // syft exclude patterns pruned from each scan (each must start with ./, */, or **/)
	AnnotateVulnerabilities bool
	AutoUpdateDB            bool
	MinSeverity             string

	// SourceName names the scanned source (the project, e.g. "anchore/chronicle")
	// so syft derives stable artifact IDs from the project + ref instead of the
	// throwaway tmpdir path. Falls back to the repo dir's base name when empty.
	SourceName string
}

// Run diffs the dependency graph between sinceRef and untilRef in the repo at
// repoPath. When cfg.AnnotateVulnerabilities is set it also loads the grype DB
// (once, shared across both scans so the two sides never drift) and annotates
// each package change with the vulnerabilities it remediated or introduced.
//
// sbomLeaf and vulnLeaf are sibling leaves of the evidence tree. sbomLeaf's
// "since"/"until" branches show each ref's live package count while cataloging;
// its parent line rolls up the package-change comparison. vulnLeaf (nil unless
// annotating) covers the independent vulnerability phase — its parent shows the
// DB update then the remediated/introduced rollup, its branches each ref's
// match. Cataloging and DB loading run in parallel; matching joins them. Both
// leaves may be nil (no UI).
func Run(ctx context.Context, repoPath, sinceRef, untilRef string, cfg Config, sbomLeaf, vulnLeaf *event.Leaf) (*dependency.Diff, error) {
	// name the syft source after the project so artifact IDs are stable across
	// the tmpdir each ref is materialized into.
	sourceName := cfg.SourceName
	if sourceName == "" {
		sourceName = filepath.Base(repoPath)
	}

	target := source.NewGitTarget(repoPath)
	scanner := scan.NewScanner(cfg.Ecosystems, cfg.ExcludePaths)

	// load the grype DB in parallel with cataloging — the two are independent;
	// a ref's match only needs both once that ref's catalog completes.
	loader := startDBLoad(cfg.AnnotateVulnerabilities, cfg.AutoUpdateDB, vulnLeaf)

	sbomLeaf.SetStage("cataloging…")
	sbomLeaf.Child("since").Start()
	sbomLeaf.Child("until").Start()

	// run both refs end-to-end in parallel: catalog (sbom branch) then, once the
	// DB is ready, match (vuln branch). A bounded executor in context drives the
	// fan-out; sync.Collect runs each job on it and joins the snapshots and errors.
	var since, until dependency.Snapshot
	jobs := []refJob{
		{ref: sinceRef, sbomBranch: sbomLeaf.Child("since"), vulnBranch: vulnLeaf.Child("since"), out: &since},
		{ref: untilRef, sbomBranch: sbomLeaf.Child("until"), vulnBranch: vulnLeaf.Child("until"), out: &until},
	}
	ctx = sync.SetContextExecutor(ctx, sync.ExecutorDefault, sync.NewExecutor(len(jobs)))
	err := sync.Collect(&ctx, sync.ExecutorDefault, sync.ToSeq(jobs),
		func(j refJob) (dependency.Snapshot, error) {
			snap, err := scanRef(ctx, target, scanner, sourceName, j.ref, j.sbomBranch, j.vulnBranch, loader, cfg.AnnotateVulnerabilities)
			if err != nil {
				return snap, fmt.Errorf("unable to scan ref %q: %w", j.ref, err)
			}
			return snap, nil
		},
		func(j refJob, snap dependency.Snapshot) {
			*j.out = snap
		})
	if err != nil {
		sbomLeaf.Fail(err)
		return nil, err
	}

	sbomLeaf.SetStage("comparing…")
	diff := dependency.Compare(since, until, scan.NewVersionComparer())

	if cfg.AnnotateVulnerabilities {
		// Annotate is pure: it returns a new diff with each change's Vuln delta
		// populated and the rollup counts recomputed. Display-time concerns
		// (only-vulnerable filtering, grouping) live in the render package and
		// the encoders, so the diff handed back is always the complete result.
		diff = dependency.Annotate(diff, since, until, dependency.AnnotateConfig{
			MinSeverity: cfg.MinSeverity,
		})
		if loader.err == nil {
			vulnLeaf.Resolve(vulnRollup(diff), "")
		}
	}
	sbomLeaf.Resolve(diffRollup(diff), "")

	return &diff, nil
}

// refJob is one ref's unit of work for the parallel fan-out: which ref to scan,
// the leaves that render its sbom/vuln progress, and where to store the result.
type refJob struct {
	ref        string
	sbomBranch *event.Leaf
	vulnBranch *event.Leaf
	out        *dependency.Snapshot
}

// dbLoad carries the shared grype DB (or its load error) from the background
// loader to each ref's match. The load runs on its own single-slot executor so
// cataloging overlaps with it; each ref calls wait before matching, blocking
// until the load completes.
type dbLoad struct {
	exec sync.Executor
	db   *scan.VulnDB
	err  error
}

// startDBLoad kicks off the grype DB load on a background executor (or leaves it
// empty when not annotating, so wait returns immediately) so cataloging and DB
// loading overlap.
func startDBLoad(annotate, autoUpdate bool, vulnLeaf *event.Leaf) *dbLoad {
	l := &dbLoad{exec: sync.NewExecutor(1)}
	if !annotate {
		return l
	}
	vulnLeaf.SetStage("updating db…")
	vulnLeaf.Child("since").Start()
	vulnLeaf.Child("until").Start()
	l.exec.Go(func() {
		l.db, l.err = scan.LoadDB(autoUpdate)
		if l.err != nil {
			vulnLeaf.Fail(l.err)
		} else {
			vulnLeaf.SetStage("matching…")
		}
	})
	return l
}

// wait blocks until the background DB load finishes (or ctx is cancelled), then
// returns the loaded DB and any load error. The result fields are read only once
// the load goroutine has completed — a clean wait happens-after that completion,
// so it is race-free; on cancellation it returns ctx.Err() without touching them.
func (l *dbLoad) wait(ctx context.Context) (*scan.VulnDB, error) {
	l.exec.Wait(ctx)
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return l.db, l.err
}

// scanRef materializes a single ref, catalogs it (driving sbomBranch), and —
// when annotating — waits for the shared DB and matches it (driving vulnBranch).
// A DB or match failure degrades to packages-only for this ref rather than
// failing the whole diff; the catalog still feeds the comparison.
func scanRef(ctx context.Context, target source.Target, scanner scan.Scanner, sourceName, ref string,
	sbomBranch, vulnBranch *event.Leaf, loader *dbLoad, annotate bool,
) (dependency.Snapshot, error) {
	dir, cleanup, err := target.Materialize(ctx, ref)
	if err != nil {
		sbomBranch.Fail(err)
		return dependency.Snapshot{}, fmt.Errorf("unable to materialize ref: %w", err)
	}
	defer func() {
		if cerr := cleanup(); cerr != nil {
			log.WithFields("error", cerr).Trace("unable to clean up materialized dependency source")
		}
	}()

	cat, err := scanner.Catalog(ctx, dir, scan.SourceInfo{Name: sourceName, Version: ref}, scan.Hooks{
		OnSourceID: func(srcID string) {
			bus.RegisterSBOMScanSource(srcID, sbomBranch)
		},
	})
	if err != nil {
		sbomBranch.Fail(err)
		return dependency.Snapshot{}, fmt.Errorf("unable to catalog ref: %w", err)
	}
	sbomBranch.Resolve(fmt.Sprintf("%d packages", len(cat.Packages)), "")
	snap := dependency.Snapshot{Packages: cat.Packages}

	if !annotate {
		return snap, nil
	}

	// matching needs both this ref's catalog (done) and the shared DB.
	db, err := loader.wait(ctx)
	if err != nil {
		vulnBranch.Fail(err)
		return snap, nil // packages-only for this ref; vuln annotation skipped
	}
	vulnBranch.SetStage("matching…")
	vulns, err := scanner.Match(ctx, db, cat)
	if err != nil {
		vulnBranch.Fail(err)
		return snap, nil
	}
	snap.Vulns = vulns
	vulnBranch.Resolve(vulnCountLabel(vulns), "")
	return snap, nil
}

// vulnRollup is the parent "vulnerabilities" resolved label: the net effect of
// the diff on vulnerabilities.
func vulnRollup(d dependency.Diff) string {
	return fmt.Sprintf("remediated=%d introduced=%d", d.RemediatedCount(), d.IntroducedCount())
}

// vulnCountLabel is a vuln branch's resolved label: the count of distinct
// vulnerabilities matched on that ref.
func vulnCountLabel(vulns map[dependency.PackageKey][]dependency.Vulnerability) string {
	ids := make(map[string]struct{})
	for _, vs := range vulns {
		for _, v := range vs {
			ids[v.ID] = struct{}{}
		}
	}
	if len(ids) == 1 {
		return "1 vulnerability"
	}
	return fmt.Sprintf("%d vulnerabilities", len(ids))
}

// diffRollup is the parent "source sbom" resolved label: a breakdown of the
// package changes by kind.
func diffRollup(d dependency.Diff) string {
	t := d.Totals()
	return fmt.Sprintf("added=%d removed=%d updated=%d downgraded=%d", t.Added, t.Removed, t.Updated, t.Downgraded)
}
