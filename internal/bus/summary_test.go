package bus

import (
	"testing"

	"github.com/charmbracelet/lipgloss"
	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/require"

	"github.com/anchore/chronicle/chronicle/event"
	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
)

func init() {
	// disable color in summary snapshots so the rendered text is deterministic
	// across local terminals and CI.
	lipgloss.SetColorProfile(termenv.Ascii)
}

func newDescription() *release.Description {
	added := change.NewType("added", change.SemVerMinor)
	changed := change.NewType("changed", change.SemVerMinor)
	fixed := change.NewType("fixed", change.SemVerPatch)
	removed := change.NewType("removed", change.SemVerMajor)
	deprecated := change.NewType("deprecated", change.SemVerMinor)
	security := change.NewType("security", change.SemVerPatch)

	return &release.Description{
		Release: release.Release{Version: "v0.19.0"},
		SupportedChanges: []change.TypeTitle{
			{ChangeType: added, Title: "Added"},
			{ChangeType: changed, Title: "Changed"},
			{ChangeType: fixed, Title: "Fixed"},
			{ChangeType: removed, Title: "Removed"},
			{ChangeType: deprecated, Title: "Deprecated"},
			{ChangeType: security, Title: "Security"},
		},
		Changes: change.Changes{
			{ChangeTypes: []change.Type{added}, Text: "a1"},
			{ChangeTypes: []change.Type{added}, Text: "a2"},
			{ChangeTypes: []change.Type{added}, Text: "a3"},
			{ChangeTypes: []change.Type{changed}, Text: "c1"},
			{ChangeTypes: []change.Type{changed}, Text: "c2"},
			{ChangeTypes: []change.Type{changed}, Text: "c3"},
			{ChangeTypes: []change.Type{changed}, Text: "c4"},
			{ChangeTypes: []change.Type{changed}, Text: "c5"},
			{ChangeTypes: []change.Type{fixed}, Text: "f1"},
			{ChangeTypes: []change.Type{fixed}, Text: "f2"},
			{ChangeTypes: []change.Type{fixed}, Text: "f3"},
			{ChangeTypes: []change.Type{fixed}, Text: "f4"},
			{ChangeTypes: []change.Type{fixed}, Text: "f5"},
			{ChangeTypes: []change.Type{fixed}, Text: "f6"},
			{ChangeTypes: []change.Type{fixed}, Text: "f7"},
			{ChangeTypes: []change.Type{fixed}, Text: "f8"},
			{ChangeTypes: []change.Type{fixed}, Text: "f9"},
			{ChangeTypes: []change.Type{fixed}, Text: "f10"},
			{ChangeTypes: []change.Type{fixed}, Text: "f11"},
			{ChangeTypes: []change.Type{removed}, Text: "r1"},
		},
	}
}

func TestReportSummary_FullBlock(t *testing.T) {
	resetSummaryCache()
	t.Cleanup(resetSummaryCache)

	rng := PublishGroup("range", []event.GroupSlotInit{
		{Name: "since", Label: "since", Intent: "latest release"},
		{Name: "until", Label: "until", Intent: "HEAD"},
	})
	rng.Slot("since").Resolve("v0.14.0", "a3b4c5d", "Jan 15 2026")
	rng.Slot("until").Resolve("v0.18.0", "f1e2d3c", "May 4 2026")

	ev := PublishTree("evidence", []string{"commits", "issues", "pull requests"})
	ev.Leaf("commits").Resolve("47", "32 associated")
	ev.Leaf("issues").Resolve("73", "42 kept")
	ev.Leaf("pull requests").Resolve("164", "87 kept")

	out := buildSummary(SummaryOpts{
		Description:     newDescription(),
		PreviousVersion: "v0.18.0",
		NextVersion:     "v0.19.0",
		BumpKind:        change.SemVerMinor,
	})
	require.NotEmpty(t, out)
	snaps.MatchSnapshot(t, out)
}

func TestReportSummary_NoSpeculation(t *testing.T) {
	resetSummaryCache()
	t.Cleanup(resetSummaryCache)

	rng := PublishGroup("range", []event.GroupSlotInit{
		{Name: "since", Label: "since", Intent: "v0.14.0"},
		{Name: "until", Label: "until", Intent: "HEAD"},
	})
	rng.Slot("since").Resolve("v0.14.0", "a3b4c5d", "Jan 15 2026")
	rng.Slot("until").Resolve("v0.18.0", "f1e2d3c", "May 4 2026")

	ev := PublishTree("evidence", []string{"commits", "issues", "pull requests"})
	ev.Leaf("commits").Resolve("47", "32 associated")
	ev.Leaf("issues").Resolve("73", "42 kept")
	ev.Leaf("pull requests").Resolve("164", "87 kept")

	out := buildSummary(SummaryOpts{
		Description:     newDescription(),
		PreviousVersion: "v0.18.0",
		NextVersion:     "", // speculation off
		BumpKind:        change.SemVerUnknown,
	})
	require.NotEmpty(t, out)
	snaps.MatchSnapshot(t, out)
}

func TestReportSummary_NoBump(t *testing.T) {
	resetSummaryCache()
	t.Cleanup(resetSummaryCache)

	out := buildSummary(SummaryOpts{
		Description:     newDescription(),
		PreviousVersion: "v0.18.0",
		NextVersion:     "v0.18.0",
		BumpKind:        change.SemVerUnknown,
	})
	snaps.MatchSnapshot(t, out)
}

func TestReportSummary_MajorBump(t *testing.T) {
	resetSummaryCache()
	t.Cleanup(resetSummaryCache)

	out := buildSummary(SummaryOpts{
		Description:     newDescription(),
		PreviousVersion: "v0.18.0",
		NextVersion:     "v1.0.0",
		BumpKind:        change.SemVerMajor,
	})
	snaps.MatchSnapshot(t, out)
}

func TestReportSummary_PatchBump(t *testing.T) {
	resetSummaryCache()
	t.Cleanup(resetSummaryCache)

	out := buildSummary(SummaryOpts{
		Description:     newDescription(),
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
