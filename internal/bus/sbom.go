package bus

import (
	"sync"

	"github.com/wagoodman/go-progress"

	"github.com/anchore/chronicle/chronicle/event"
)

// The source-sbom branch leaves ("since"/"until") show a live package count
// while syft catalogs each ref. syft publishes that count on a "package-
// cataloging" task that carries no source identity, so it can't be attributed
// to a branch on its own. The top-level "cataloging" task, however, does carry
// the source ID in its context and is published immediately before its package
// task in the same goroutine. We bridge the two here in two steps, so neither
// the scan nor the UI has to hold the other's handle:
//
//   - the UI worker, which owns the leaf tree, registers each ref's branch leaf
//     against the ref string (RegisterSBOMLeaf);
//   - the scan, deep in the syft integration, reports the source ID syft
//     assigned to that ref (LinkSBOMSource) — identifiers only, no leaf.
//
// LinkSBOMSource joins the two into a source-ID → leaf map. The UI then enqueues
// that leaf when it sees the top-level task and binds the next package task's
// progress to it (FIFO).
//
// Best-effort by design: the since/until refs catalog concurrently, so if both
// top-level tasks happen to be published before either package task, the FIFO
// pairing can transiently swap which branch shows which count. Because the
// package task carries no source identity, there is no event-level handle to
// pair them precisely; only serializing cataloging or an upstream syft change
// could, and neither is worth it. This affects only the live count shown
// mid-scan — each branch is resolved with its own authoritative count once its
// catalog completes (the worker resolves it from the returned diff), so the
// final display is always correct.
var (
	sbomMu        sync.Mutex
	sbomLeafByRef = map[string]*event.Leaf{} // ref (SourceInfo.Version) -> branch leaf
	sbomLeafBySrc = map[string]*event.Leaf{} // syft source ID -> branch leaf
	sbomBindQueue []*event.Leaf
)

// RegisterSBOMLeaf records the branch leaf that should show the live cataloging
// count for the given scan ref (the SourceInfo.Version handed to the scanner).
// Called by the UI worker that owns the leaf tree, before scanning begins.
func RegisterSBOMLeaf(ref string, leaf *event.Leaf) {
	if leaf == nil || ref == "" {
		return
	}
	sbomMu.Lock()
	defer sbomMu.Unlock()
	sbomLeafByRef[ref] = leaf
}

// LinkSBOMSource records that syft resolved the given ref to srcID, linking the
// source ID to whatever branch leaf was registered for that ref. Called by the
// scan once GetSource resolves, before cataloging begins — it carries only
// identifiers, so the scan stays free of any UI handle.
func LinkSBOMSource(ref, srcID string) {
	if ref == "" || srcID == "" {
		return
	}
	sbomMu.Lock()
	defer sbomMu.Unlock()
	sbomLeafBySrc[srcID] = sbomLeafByRef[ref]
}

// EnqueueSBOMBind queues the leaf for the source identified by srcID to receive
// the next package-cataloging progress. Called by the UI when it observes the
// top-level cataloging task (which carries srcID in its context).
func EnqueueSBOMBind(srcID string) {
	sbomMu.Lock()
	defer sbomMu.Unlock()
	if leaf := sbomLeafBySrc[srcID]; leaf != nil {
		sbomBindQueue = append(sbomBindQueue, leaf)
	}
}

// BindSBOMPackageProgress binds a package-cataloging progress source to the
// next queued branch leaf (FIFO), so that branch's line shows the live package
// count syft reports as it catalogs. Each enqueued leaf is bound at most once.
func BindSBOMPackageProgress(p progress.StagedProgressable) {
	sbomMu.Lock()
	defer sbomMu.Unlock()
	if len(sbomBindQueue) == 0 {
		return
	}
	leaf := sbomBindQueue[0]
	sbomBindQueue = sbomBindQueue[1:]
	leaf.BindLive(p)
}
