package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/anchore/chronicle/chronicle/event"
)

// leaf renders one line within a tree group:
//
//	{tree-prefix} {spinner-or-mark} {name}   {count}{   ({note} dim)}?
//
// while running, the count column shows the running stage detail (e.g.
// "page 4 — 164 received") instead of the resolved count. Spinner is the
// shared singleton owned by the top-level UI.
type leaf struct {
	sp     *spinner.Model
	data   *event.Leaf
	prefix string

	nameWidth int
}

func newLeaf(l *event.Leaf, sp *spinner.Model) *leaf {
	return &leaf{sp: sp, data: l}
}

func (l *leaf) SetPrefix(prefix string) { l.prefix = prefix }
func (l *leaf) SetNameWidth(width int)  { l.nameWidth = width }

func (l *leaf) Init() tea.Cmd                         { return nil }
func (l *leaf) Update(_ tea.Msg) (tea.Model, tea.Cmd) { return l, nil }

// View is a pure read of the current leaf/spinner state.
func (l leaf) View() string {
	var b strings.Builder
	b.WriteString(dimStyle.Render(l.prefix))
	b.WriteString(" ")
	b.WriteString(l.markView())
	b.WriteString(" ")
	b.WriteString(padRight(l.data.Name(), l.nameWidth))
	b.WriteString("   ")

	switch l.data.State() {
	case event.SlotResolved:
		b.WriteString(resolvedStyle.Render(formatMetrics(l.data.Metrics())))
		if note := droppedText(l.data.Dropped()); note != "" {
			b.WriteString("   ")
			b.WriteString(dimStyle.Render("(" + note + ")"))
		}
	case event.SlotSkipped:
		// no count — the work was intentionally not performed
		b.WriteString(dimStyle.Render("skipped"))
	case event.SlotFailed:
		if err := l.data.Err(); err != nil {
			b.WriteString(dimStyle.Render(err.Error()))
		}
	case event.SlotRunning:
		if cur := l.data.RunningDetail(); cur != "" {
			b.WriteString(dimStyle.Render(cur))
		} else {
			b.WriteString(dimStyle.Render("waiting"))
		}
	default:
		b.WriteString(dimStyle.Render("waiting"))
	}
	return b.String()
}

func (l leaf) markView() string {
	if l.data.State() == event.SlotRunning && l.sp != nil {
		return l.sp.View()
	}
	return staticMark(l.data.State())
}
