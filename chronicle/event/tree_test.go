package event

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/go-progress"
)

func TestNewTreeWithChildren(t *testing.T) {
	tr := NewTreeWithChildren("evidence", []LeafSpec{
		{Name: "commits"},
		{Name: "source sbom", Children: []string{"since", "until"}},
	})

	assert.Equal(t, []string{"commits", "source sbom"}, tr.Names())
	assert.Empty(t, tr.Leaf("commits").Children())

	sbom := tr.Leaf("source sbom")
	children := sbom.Children()
	require.Len(t, children, 2)
	assert.Equal(t, "since", children[0].Name())
	assert.Equal(t, "until", children[1].Name())
	assert.Same(t, children[0], sbom.Child("since"))
	assert.Nil(t, sbom.Child("missing"))
}

func TestLeaf_RunningDetail_LiveTakesPrecedence(t *testing.T) {
	tr := NewTreeWithChildren("evidence", []LeafSpec{
		{Name: "source sbom", Children: []string{"since"}},
	})
	since := tr.Leaf("source sbom").Child("since")

	// with only a manual stage set, that stage is the running detail.
	since.SetStage("waiting on materialize")
	assert.Equal(t, "waiting on materialize", since.RunningDetail())
	assert.Equal(t, SlotRunning, since.State())

	// once a live source is bound, its current stage wins (the live syft count).
	since.BindLive(stagedString("142 packages"))
	assert.Equal(t, "142 packages", since.RunningDetail())

	// after resolving, the resolved metric is authoritative and live is ignored.
	since.Resolve(Count("package", 142))
	assert.Equal(t, SlotResolved, since.State())
	assert.Equal(t, []Metric{{Name: "package", Count: 142}}, since.Metrics())
}

func TestTree_Close_ResolvesChildren(t *testing.T) {
	tr := NewTreeWithChildren("evidence", []LeafSpec{
		{Name: "source sbom", Children: []string{"since", "until"}},
	})
	sbom := tr.Leaf("source sbom")
	sbom.Child("since").Start()

	tr.Close()

	assert.Equal(t, SlotResolved, sbom.State())
	assert.Equal(t, SlotResolved, sbom.Child("since").State())
	assert.Equal(t, SlotResolved, sbom.Child("until").State())
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
