package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/anchore/chronicle/chronicle/event"
)

const (
	branchMid  = "├──" // a non-last item's branch glyph
	branchLast = "└──" // the last item's branch glyph
)

// treeGroup renders a header line followed by N child leaf models stacked with
// tree prefixes (├── for non-last positions, └── for the last). Same expire
// protocol as bracketGroup.
type treeGroup struct {
	header  string
	leaves  []*leaf
	expired bool
}

// NewTreeGroup constructs the bubbletea model for an *event.Tree, flattening
// leaves and their (one level of) children into a stack of rows with the right
// tree-drawing prefixes. The spinner is the shared singleton owned by the
// top-level UI.
func NewTreeGroup(t *event.Tree, sp *spinner.Model) tea.Model {
	if t == nil {
		return &treeGroup{}
	}
	names := t.Names()
	tg := &treeGroup{
		header: displayHeader(t.Header),
		leaves: make([]*leaf, 0, len(names)),
	}

	// top-level leaves align against each other; children align against their
	// own siblings (a deeper indent makes cross-level alignment meaningless).
	topWidth := 0
	for _, n := range names {
		if l := len(n); l > topWidth {
			topWidth = l
		}
	}

	for i, n := range names {
		parent := t.Leaf(n)
		parentLast := i == len(names)-1

		pl := newLeaf(parent, sp)
		pl.SetPrefix(treePrefix(i, len(names)))
		pl.SetNameWidth(topWidth)
		tg.leaves = append(tg.leaves, pl)

		children := parent.Children()
		if len(children) == 0 {
			continue
		}
		childWidth := 0
		for _, c := range children {
			if l := len(c.Name()); l > childWidth {
				childWidth = l
			}
		}
		for j, c := range children {
			cl := newLeaf(c, sp)
			cl.SetPrefix(childPrefix(parentLast, j, len(children)))
			cl.SetNameWidth(childWidth)
			tg.leaves = append(tg.leaves, cl)
		}
	}
	return tg
}

func treePrefix(i, n int) string {
	if i == n-1 {
		return branchLast
	}
	return branchMid
}

// childPrefix builds a nested child's prefix: a continuation column under the
// parent ("│   " when the parent has rows below it, else "    ") followed by
// the child's own branch glyph.
func childPrefix(parentLast bool, j, n int) string {
	cont := "│   "
	if parentLast {
		cont = "    "
	}
	branch := branchMid
	if j == n-1 {
		branch = branchLast
	}
	return cont + branch
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
