package ui

import (
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/anchore/chronicle/chronicle/event"
)

// slot renders one line within a bracket group:
//
//	{bracket} {spinner-or-mark} {label}   {intent dim}{ → {resolved}}?
//
// the bracket prefix is supplied by the parent bracketGroup via SetPrefix so
// the same model works in any position. The spinner is a *singleton* owned by
// the top-level UI; slot only reads its current frame at render time.
type slot struct {
	sp     *spinner.Model
	data   *event.Slot
	prefix string

	labelWidth int
}

func newSlot(s *event.Slot, sp *spinner.Model) *slot {
	return &slot{sp: sp, data: s}
}

func (s *slot) SetPrefix(prefix string) { s.prefix = prefix }
func (s *slot) SetLabelWidth(width int) { s.labelWidth = width }

// Init returns nil — the shared spinner is initialized once by the top-level
// UI, not per-slot.
func (s *slot) Init() tea.Cmd { return nil }

// Update is a no-op for the same reason; the top-level UI owns spinner ticks.
// The pointer receiver is preserved to satisfy tea.Model.
func (s *slot) Update(_ tea.Msg) (tea.Model, tea.Cmd) { return s, nil }

// View is a pure read of the current slot/spinner state. Value receiver to
// signal it does not mutate.
func (s slot) View() string {
	var b strings.Builder
	b.WriteString(dimStyle.Render(s.prefix))
	b.WriteString(" ")
	b.WriteString(s.markView())
	b.WriteString(" ")
	b.WriteString(padRight(s.data.Label(), s.labelWidth))
	b.WriteString("   ")

	intent := dimStyle.Render(s.data.Intent())
	switch s.data.State() {
	case event.SlotResolved:
		b.WriteString(intent)
		if vals := s.data.Values(); len(vals) > 0 {
			b.WriteString(" ")
			b.WriteString(dimStyle.Render(arrow))
			b.WriteString(" ")
			b.WriteString(resolvedStyle.Render(strings.Join(vals, "  ")))
		}
	case event.SlotFailed:
		if err := s.data.Err(); err != nil {
			b.WriteString(dimStyle.Render(err.Error()))
		} else {
			b.WriteString(intent)
		}
	case event.SlotRunning:
		b.WriteString(intent)
		if cur := s.data.Stage.Current; cur != "" {
			b.WriteString(dimStyle.Render(" …" + cur))
		}
	default:
		// pending
		b.WriteString(intent)
	}
	return b.String()
}

// markView returns the mark column: shared spinner while running, ✔ on
// resolved, red ✘ on failure, dim dot while pending.
func (s slot) markView() string {
	switch s.data.State() {
	case event.SlotResolved:
		return okMarkStyle.Render(checkMark)
	case event.SlotFailed:
		return failStyle.Render(xMark)
	case event.SlotRunning:
		if s.sp != nil {
			return s.sp.View()
		}
	}
	return dimStyle.Render(dotMark)
}
