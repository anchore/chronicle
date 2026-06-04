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
// task in the same goroutine. We bridge the two here: the scan registers each
// ref's source ID against its leaf; the UI enqueues that leaf when it sees the
// top-level task, then binds the next package task's progress to it (FIFO).
//
// Best-effort by design: the since/until refs catalog concurrently, so if both
// top-level tasks happen to be published before either package task, the FIFO
// pairing can transiently swap which branch shows which count. Because the
// package task carries no source identity, there is no event-level handle to
// pair them precisely; only serializing cataloging or an upstream syft change
// could, and neither is worth it. This affects only the live count shown
// mid-scan — each branch is resolved with its own authoritative count once its
// catalog completes (see report.scanRef), so the final display is always correct.
var (
	sbomMu        sync.Mutex
	sbomLeafBySrc = map[string]*event.Leaf{}
	sbomBindQueue []*event.Leaf
)

// RegisterSBOMScanSource records that the syft source identified by srcID
// corresponds to the given branch leaf. Called by the scan once GetSource
// resolves, before cataloging begins.
func RegisterSBOMScanSource(srcID string, leaf *event.Leaf) {
	if leaf == nil || srcID == "" {
		return
	}
	sbomMu.Lock()
	defer sbomMu.Unlock()
	sbomLeafBySrc[srcID] = leaf
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
