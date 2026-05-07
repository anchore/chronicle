package ui

import (
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/lipgloss"
)

// style palette per the spec's "Style palette" table. ANSI base colors only —
// no hex — so terminals with custom palettes look right. The summary-block
// styles (bump, notify, mini-bar) live in internal/bus/summary.go alongside
// their renderer rather than here.
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

// displayHeader maps an internal cache key (e.g. "range", "evidence") to the
// label rendered in the live TUI. The "range" group is suppressed here — the
// handler injects "Project: OWNER/REPO" as the title instead. Other headers
// are title-cased and bolded so they stand out as section dividers.
func displayHeader(name string) string {
	if name == "range" || name == "" {
		return ""
	}
	return boldStyle.Render(strings.ToUpper(name[:1]) + name[1:])
}
