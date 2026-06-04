package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"

	"github.com/anchore/chronicle/chronicle/event"
)

// style palette per the spec's "Style palette" table. ANSI base colors only —
// no hex — so terminals with custom palettes look right.
var (
	dimStyle      = lipgloss.NewStyle().Faint(true)
	resolvedStyle = lipgloss.NewStyle()
	boldStyle     = lipgloss.NewStyle().Bold(true)
	failStyle     = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	okMarkStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
)

const (
	// heavy glyphs matching syft/grype/quill (via bubbly/taskprogress).
	checkMark = "✔" // U+2714 Heavy Check Mark
	xMark     = "✘" // U+2718 Heavy Ballot X
	dotMark   = "·"
	skipMark  = "⊘" // U+2298 Circled Division Slash — work intentionally skipped
	arrow     = "→"
)

// chronicleSpinnerFrames is the same braille spinner used by syft/grype/quill
// (via bubbly/taskprogress). Defined here once so slot/leaf can attach it to a
// vanilla bubbles/spinner without dragging in taskprogress.
var chronicleSpinnerFrames = strings.Split("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏", "")

func newChronicleSpinner() spinner.Model {
	s := spinner.New()
	s.Spinner = spinner.Spinner{
		Frames: chronicleSpinnerFrames,
		FPS:    150 * time.Millisecond,
	}
	s.Style = lipgloss.NewStyle().Foreground(lipgloss.Color("13"))
	return s
}

// staticMark renders the status glyph for a non-running slot/leaf state, shared
// by the live slot/leaf rows (which add a spinner for the running state) and the
// recap (which never sees a running state). Skipped only ever applies to a leaf;
// a slot can't reach it, so mapping it here is inert for slots.
func staticMark(s event.SlotState) string {
	switch s {
	case event.SlotResolved:
		return okMarkStyle.Render(checkMark)
	case event.SlotFailed:
		return failStyle.Render(xMark)
	case event.SlotSkipped:
		return dimStyle.Render(skipMark)
	}
	return dimStyle.Render(dotMark)
}

// headerRange is the group key whose section is rendered as the project title
// ("Project: OWNER/REPO") rather than a bolded header.
const headerRange = "range"

// displayHeader maps an internal cache key (e.g. "range", "evidence") to the
// label rendered in the live TUI. The "range" group is suppressed here — the
// handler injects "Project: OWNER/REPO" as the title instead. Other headers
// are title-cased and bolded so they stand out as section dividers.
func displayHeader(name string) string {
	if name == headerRange || name == "" {
		return ""
	}
	return boldStyle.Render(strings.ToUpper(name[:1]) + name[1:])
}
