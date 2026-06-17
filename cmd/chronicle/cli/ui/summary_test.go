package ui

import (
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/require"

	"github.com/anchore/chronicle/chronicle/event"
	"github.com/anchore/chronicle/chronicle/release/change"
)

// sampleChanges mirrors a realistic change tally: major/minor/patch populated
// and two zero-count types that must be dropped from the rendered tiers.
func sampleChanges() []event.SummaryChange {
	return []event.SummaryChange{
		{Name: "added", Kind: change.SemVerMinor, Count: 3},
		{Name: "changed", Kind: change.SemVerMinor, Count: 5},
		{Name: "fixed", Kind: change.SemVerPatch, Count: 11},
		{Name: "removed", Kind: change.SemVerMajor, Count: 1},
		{Name: "deprecated", Kind: change.SemVerMinor, Count: 0},
		{Name: "security", Kind: change.SemVerPatch, Count: 0},
	}
}

func sampleRange() *event.Group {
	g := event.NewGroup("range", []event.GroupSlotInit{
		{Name: "since", Label: "since", Intent: "latest release"},
		{Name: "until", Label: "until", Intent: "HEAD"},
	})
	g.Slot("since").Resolve(event.Text("v0.14.0"), event.SHA("a3b4c5d"), event.Date(time.Date(2026, time.January, 15, 0, 0, 0, 0, time.UTC)))
	g.Slot("until").Resolve(event.Text("v0.18.0"), event.SHA("f1e2d3c"), event.Date(time.Date(2026, time.May, 4, 0, 0, 0, 0, time.UTC)))
	return g
}

func sampleEvidence() *event.Tree {
	tr := event.NewTree("evidence", []string{"commits", "issues", "pull requests"})
	c := tr.Leaf("commits")
	c.Resolve(event.Num(47))
	c.SetDropped(15)
	i := tr.Leaf("issues")
	i.Resolve(event.Num(73))
	i.SetDropped(31)
	p := tr.Leaf("pull requests")
	p.Resolve(event.Num(164))
	p.SetDropped(77)
	return tr
}

func TestRenderSummary_FullBlock(t *testing.T) {
	out := RenderSummary(
		[]*event.Group{sampleRange()},
		[]*event.Tree{sampleEvidence()},
		event.Summary{
			Repo:            "anchore/chronicle",
			Changes:         sampleChanges(),
			PreviousVersion: "v0.18.0",
			NextVersion:     "v0.19.0",
			BumpKind:        change.SemVerMinor,
		})
	require.NotEmpty(t, out)
	snaps.MatchSnapshot(t, out)
}

func TestRenderSummary_NoSpeculation(t *testing.T) {
	out := RenderSummary(
		[]*event.Group{sampleRange()},
		[]*event.Tree{sampleEvidence()},
		event.Summary{
			Changes:         sampleChanges(),
			PreviousVersion: "v0.18.0",
			NextVersion:     "", // speculation off → no version-transition line
			BumpKind:        change.SemVerUnknown,
		})
	require.NotEmpty(t, out)
	snaps.MatchSnapshot(t, out)
}

func TestRenderSummary_SkippedEvidence(t *testing.T) {
	// HEAD sits exactly on the previous release: zero commits in scope, so the
	// issue and PR fetches were skipped. They should render as "skipped" rather
	// than a resolved zero count.
	rng := event.NewGroup("range", []event.GroupSlotInit{
		{Name: "since", Label: "since", Intent: "latest release"},
		{Name: "until", Label: "until", Intent: "HEAD"},
	})
	rng.Slot("since").Resolve(event.Text("v1.45.0"), event.SHA("9673f86"), event.Date(time.Date(2026, time.June, 2, 0, 0, 0, 0, time.UTC)))
	rng.Slot("until").Resolve(event.Text("v1.45.0"))

	ev := event.NewTree("evidence", []string{"commits", "issues", "pull requests"})
	ev.Leaf("commits").Resolve(event.Num(0))
	ev.Leaf("issues").Skip()
	ev.Leaf("pull requests").Skip()

	out := RenderSummary(
		[]*event.Group{rng},
		[]*event.Tree{ev},
		event.Summary{
			Changes:         sampleChanges(),
			PreviousVersion: "v1.45.0",
			NextVersion:     "", // nothing changed, no speculation
			BumpKind:        change.SemVerUnknown,
		})
	require.NotEmpty(t, out)
	snaps.MatchSnapshot(t, out)
}

func TestRenderSummary_NoBump(t *testing.T) {
	out := RenderSummary(nil, nil, event.Summary{
		Changes:         sampleChanges(),
		PreviousVersion: "v0.18.0",
		NextVersion:     "v0.18.0",
		BumpKind:        change.SemVerUnknown,
	})
	snaps.MatchSnapshot(t, out)
}

func TestRenderSummary_MajorBump(t *testing.T) {
	out := RenderSummary(nil, nil, event.Summary{
		Changes:         sampleChanges(),
		PreviousVersion: "v0.18.0",
		NextVersion:     "v1.0.0",
		BumpKind:        change.SemVerMajor,
	})
	snaps.MatchSnapshot(t, out)
}

func TestRenderSummary_PatchBump(t *testing.T) {
	out := RenderSummary(nil, nil, event.Summary{
		Changes:         sampleChanges(),
		PreviousVersion: "v0.18.0",
		NextVersion:     "v0.18.1",
		BumpKind:        change.SemVerPatch,
	})
	snaps.MatchSnapshot(t, out)
}

func TestHighlightBumpedElement_NotSemver(t *testing.T) {
	got := highlightBumpedElement("not", "semver", change.SemVerMinor)
	// must not panic; returns the next version (possibly styled).
	require.Equal(t, "semver", got)
}
