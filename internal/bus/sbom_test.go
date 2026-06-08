package bus

import (
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/wagoodman/go-progress"

	"github.com/anchore/chronicle/chronicle/event"
)

func TestSBOMPackageProgressBinding(t *testing.T) {
	tr := event.NewTreeWithChildren("evidence", []event.LeafSpec{
		{Name: "source sbom", Children: []string{"since", "until"}},
	})
	sbom := tr.Leaf("source sbom")
	since := sbom.Child("since")
	until := sbom.Child("until")

	// the worker registers each ref's branch leaf; the scan then links the syft
	// source ID syft assigned to that ref.
	RegisterSBOMLeaf("since-ref", since)
	RegisterSBOMLeaf("until-ref", until)
	LinkSBOMSource("since-ref", "src-since")
	LinkSBOMSource("until-ref", "src-until")

	// syft publishes the top-level cataloging task (carrying the source ID),
	// then the package-cataloging task (the live count) right after. When the
	// package tasks arrive in the same order their top-level tasks were enqueued,
	// FIFO pairing attributes each count to the right branch. (This ordering is
	// best-effort across concurrent refs — see the package doc in sbom.go.)
	EnqueueSBOMBind("src-since")
	EnqueueSBOMBind("src-until")
	BindSBOMPackageProgress(stagedString("142 packages"))
	BindSBOMPackageProgress(stagedString("156 packages"))

	require.Equal(t, "142 packages", since.RunningDetail())
	require.Equal(t, "156 packages", until.RunningDetail())
}

func TestSBOMPackageProgressBinding_UnknownSourceIsIgnored(t *testing.T) {
	require.NotPanics(t, func() {
		// a top-level task for a source we never registered enqueues nothing, so
		// a following package task has no leaf to bind to and is a no-op.
		EnqueueSBOMBind("never-registered")
		BindSBOMPackageProgress(stagedString("1 packages"))
	})
}

func stagedString(stage string) progress.StagedProgressable {
	return &struct {
		progress.Stager
		progress.Progressable
	}{
		Stager:       progress.NewAtomicStage(stage),
		Progressable: progress.NewManual(-1),
	}
}
