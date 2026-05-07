package ui

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/muesli/termenv"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/go-partybus"
	"github.com/wagoodman/go-progress"

	"github.com/anchore/bubbly"
	"github.com/anchore/chronicle/chronicle/event"
)

func init() {
	// disable color so rendered snapshots are deterministic across local
	// terminals and CI.
	lipgloss.SetColorProfile(termenv.Ascii)
}

// compile-time assertion (mirrors the one in handler.go for test-time
// enforcement).
var _ bubbly.EventHandler = (*Handler)(nil)

func TestNew_InitializesState(t *testing.T) {
	h := New()
	assert.NotNil(t, h.state)
	assert.NotNil(t, h.state.Running)
}

func TestHandler_RespondsToTaskTypes(t *testing.T) {
	h := New()
	got := h.RespondsTo()
	assert.Contains(t, got, event.TaskType)
	assert.Contains(t, got, event.GroupTaskType)
	assert.Contains(t, got, event.TreeTaskType)
}

func TestHandler_HandleTask(t *testing.T) {
	h := New()
	prog := event.ManualStagedProgress{}
	value := progress.StagedProgressable(&struct {
		progress.Stager
		progress.Progressable
	}{
		Stager:       &prog.Stage,
		Progressable: &prog.Manual,
	})
	models, _ := h.Handle(partybus.Event{
		Type: event.TaskType,
		Source: event.Task{
			Title:   event.Title{Default: "doing a thing"},
			Context: "ctx",
		},
		Value: value,
	})
	require.Len(t, models, 1)
	assert.NotNil(t, models[0])
}

func TestHandler_HandleGroupTask(t *testing.T) {
	h := New()
	g := event.NewGroup("range", []event.GroupSlotInit{
		{Name: "since", Label: "since", Intent: "latest release"},
		{Name: "until", Label: "until", Intent: "HEAD"},
	})
	models, _ := h.Handle(partybus.Event{
		Type:   event.GroupTaskType,
		Source: "range",
		Value:  g,
	})
	require.Len(t, models, 1)
	assert.NotNil(t, models[0])

	// the "range" group is rendered headerless — bracket pair speaks for
	// itself. Verify the slot rows are present instead.
	view := models[0].View()
	assert.Contains(t, view, "since")
	assert.Contains(t, view, "until")
	assert.NotContains(t, view, "range")
}

func TestHandler_HandleTreeTask(t *testing.T) {
	h := New()
	tr := event.NewTree("evidence", []string{"commits", "issues", "pull requests"})
	models, _ := h.Handle(partybus.Event{
		Type:   event.TreeTaskType,
		Source: "evidence",
		Value:  tr,
	})
	require.Len(t, models, 1)
	view := models[0].View()
	// header is title-cased at render time; cache key stays lowercase.
	assert.Contains(t, view, "Evidence")
}

func TestHandler_BadEventDoesNotPanic(t *testing.T) {
	h := New()
	assert.NotPanics(t, func() {
		_, _ = h.Handle(partybus.Event{Type: event.GroupTaskType, Value: "not a group"})
		_, _ = h.Handle(partybus.Event{Type: event.TreeTaskType, Value: "not a tree"})
		_, _ = h.Handle(partybus.Event{Type: event.TaskType, Source: "no good"})
	})
}

// snapshot tests for bracketGroup and treeGroup rendering.

func TestBracketGroup_Snapshot_Resolving(t *testing.T) {
	g := event.NewGroup("range", []event.GroupSlotInit{
		{Name: "since", Label: "since", Intent: "latest release"},
		{Name: "until", Label: "until", Intent: "HEAD"},
	})
	g.Slot("since").Start()
	g.Slot("since").SetStage("looking up tag")

	bg := buildBracketGroup(g)
	snaps.MatchSnapshot(t, bg.View())
}

func TestBracketGroup_Snapshot_Resolved(t *testing.T) {
	g := event.NewGroup("range", []event.GroupSlotInit{
		{Name: "since", Label: "since", Intent: "latest release"},
		{Name: "until", Label: "until", Intent: "HEAD"},
	})
	g.Slot("since").Resolve("v0.14.0", "a3b4c5d", "Jan 15 2026")
	g.Slot("until").Resolve("v0.18.0", "f1e2d3c", "May 4 2026")

	bg := buildBracketGroup(g)
	snaps.MatchSnapshot(t, bg.View())
}

func TestTreeGroup_Snapshot_Resolving(t *testing.T) {
	tr := event.NewTree("evidence", []string{"commits", "issues", "pull requests"})
	tr.Leaf("issues").SetStage("page 2 — 73 received")
	tr.Leaf("pull requests").SetStage("page 4 — 164 received")

	tg := buildTreeGroup(tr)
	snaps.MatchSnapshot(t, tg.View())
}

func TestTreeGroup_Snapshot_Resolved(t *testing.T) {
	tr := event.NewTree("evidence", []string{"commits", "issues", "pull requests"})
	tr.Leaf("commits").Resolve("47", "32 associated")
	tr.Leaf("issues").Resolve("73", "42 kept")
	tr.Leaf("pull requests").Resolve("164", "87 kept")

	tg := buildTreeGroup(tr)
	snaps.MatchSnapshot(t, tg.View())
}

func buildBracketGroup(g *event.Group) tea.Model {
	sp := newChronicleSpinner()
	return NewBracketGroup(g, "", &sp)
}

func buildTreeGroup(t *event.Tree) tea.Model {
	sp := newChronicleSpinner()
	return NewTreeGroup(t, &sp)
}
