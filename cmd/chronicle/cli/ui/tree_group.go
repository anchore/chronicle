package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/anchore/chronicle/chronicle/event"
)

// treeGroup renders a header line followed by N child leaf models stacked with
// tree prefixes (├── for non-last positions, └── for the last). Same expire
// protocol as bracketGroup.
type treeGroup struct {
	header  string
	leaves  []*leaf
	expired bool
}

// NewTreeGroup constructs the bubbletea model for an *event.Tree. The
// spinner is the shared singleton owned by the top-level UI.
func NewTreeGroup(t *event.Tree, sp *spinner.Model) tea.Model {
	if t == nil {
		return &treeGroup{}
	}
	names := t.Names()
	tg := &treeGroup{
		header: displayHeader(t.Header),
		leaves: make([]*leaf, 0, len(names)),
	}

	nameWidth := 0
	for _, n := range names {
		if l := len(n); l > nameWidth {
			nameWidth = l
		}
	}

	for i, n := range names {
		l := newLeaf(t.Leaf(n), sp)
		l.SetPrefix(treePrefix(i, len(names)))
		l.SetNameWidth(nameWidth)
		tg.leaves = append(tg.leaves, l)
	}
	return tg
}

func treePrefix(i, n int) string {
	if i == n-1 {
		return "└──"
	}
	return "├──"
}

func (t *treeGroup) Init() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(t.leaves))
	for _, l := range t.leaves {
		cmds = append(cmds, l.Init())
	}
	return tea.Batch(cmds...)
}

func (t *treeGroup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(ExpireGroupsMsg); ok {
		t.expired = true
	}

	cmds := make([]tea.Cmd, 0, len(t.leaves))
	for i, l := range t.leaves {
		m, cmd := l.Update(msg)
		if nl, ok := m.(*leaf); ok {
			t.leaves[i] = nl
		}
		cmds = append(cmds, cmd)
	}
	return t, tea.Batch(cmds...)
}

// IsAlive implements bubbly.TerminalElement.
func (t *treeGroup) IsAlive() bool { return !t.expired }

// View — see bracketGroup.View for the rationale on the trailing "\n".
func (t *treeGroup) View() string {
	lines := make([]string, 0, len(t.leaves)+1)
	if t.header != "" {
		lines = append(lines, t.header)
	}
	for _, l := range t.leaves {
		lines = append(lines, l.View())
	}
	return strings.Join(lines, "\n") + "\n"
}
