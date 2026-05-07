package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/anchore/chronicle/chronicle/event"
)

// bracketGroup renders a header line followed by N child slot models stacked
// with bracket prefixes (┌── for first, └── for last, ├── otherwise).
//
// Implements bubbly's TerminalElement protocol: on receipt of ExpireGroupsMsg
// the group flags itself, and the next frame.Update prunes it from the live
// area. UI emits ExpireGroupsMsg when the application exit signal arrives.
type bracketGroup struct {
	header  string
	slots   []*slot
	expired bool
}

// NewBracketGroup constructs the bubbletea model for an *event.Group. The
// spinner is the shared singleton owned by the top-level UI; bracket child
// slots all read its current frame at render time. title overrides the
// auto-derived header when non-empty (used by the "range" group to render
// "Project: OWNER/REPO" above the since/until tree).
func NewBracketGroup(g *event.Group, title string, sp *spinner.Model) tea.Model {
	if g == nil {
		return &bracketGroup{}
	}
	names := g.Names()
	header := displayHeader(g.Header)
	if title != "" {
		header = title
	}
	bg := &bracketGroup{
		header: header,
		slots:  make([]*slot, 0, len(names)),
	}

	// compute label width once (label text is fixed at construction time)
	labelWidth := 0
	for _, n := range names {
		if l := len(g.Slot(n).Label()); l > labelWidth {
			labelWidth = l
		}
	}

	for i, n := range names {
		s := newSlot(g.Slot(n), sp)
		s.SetPrefix(bracketPrefix(i, len(names)))
		s.SetLabelWidth(labelWidth)
		bg.slots = append(bg.slots, s)
	}
	return bg
}

// bracketPrefix returns the tree-prefix glyph for slot i of n. Matches the
// evidence tree's prefix shape — a `┌──` first row felt overly decorative
// once the section was made headerless and the repo became the title above.
func bracketPrefix(i, n int) string {
	if i == n-1 {
		return "└──"
	}
	return "├──"
}

func (b *bracketGroup) Init() tea.Cmd {
	cmds := make([]tea.Cmd, 0, len(b.slots))
	for _, s := range b.slots {
		cmds = append(cmds, s.Init())
	}
	return tea.Batch(cmds...)
}

func (b *bracketGroup) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(ExpireGroupsMsg); ok {
		b.expired = true
	}

	cmds := make([]tea.Cmd, 0, len(b.slots))
	for i, s := range b.slots {
		m, cmd := s.Update(msg)
		if ns, ok := m.(*slot); ok {
			b.slots[i] = ns
		}
		cmds = append(cmds, cmd)
	}
	return b, tea.Batch(cmds...)
}

// IsAlive implements bubbly.TerminalElement.
func (b *bracketGroup) IsAlive() bool { return !b.expired }

// View renders the bracket group; pointer receiver only because slots are
// pointers and the slice layout would otherwise force a copy at every call.
//
// Trailing newline is intentional: bubbly's frame joins child views with a
// single "\n", so a trailing "\n" here produces a blank line between this
// group and whatever section follows (matching the post-teardown summary's
// "\n\n" between sections).
func (b *bracketGroup) View() string {
	lines := make([]string, 0, len(b.slots)+1)
	if b.header != "" {
		lines = append(lines, b.header)
	}
	for _, s := range b.slots {
		lines = append(lines, s.View())
	}
	return strings.Join(lines, "\n") + "\n"
}

// padRight is small enough to colocate; reused by leaf.go via the same package.
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
