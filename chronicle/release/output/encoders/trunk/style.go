package trunk

import (
	"fmt"
	"io"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/savioxavier/termlink"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
)

// styles groups all the lipgloss styles used by the trunk renderer.
// They are constructed once per Encode call via newStyles so that the
// renderer's color profile (TTY vs ASCII) is set correctly for the output.
type styles struct {
	dim    lipgloss.Style
	normal lipgloss.Style
	// semver category styles. unknown reuses dim.
	major lipgloss.Style
	minor lipgloss.Style
	patch lipgloss.Style
	// trunkLine is the vertical connector drawn between rows.
	trunkLine string
	// link wraps text in an OSC 8 hyperlink escape when the destination is a
	// TTY and a non-empty URL is provided; otherwise it returns text unchanged.
	link func(text, url string) string
}

// newStyles builds a styles value whose color profile matches isTTY.
// When isTTY is false the renderer uses termenv.Ascii so all styles become no-ops
// and no ANSI escape codes are emitted.
func newStyles(isTTY bool) styles {
	r := lipgloss.NewRenderer(nil)
	if !isTTY {
		r.SetColorProfile(termenv.Ascii)
	}

	link := func(text, _ string) string { return text }
	if isTTY {
		link = func(text, url string) string {
			if url == "" {
				return text
			}
			return termlink.Link(text, url)
		}
	}

	return styles{
		dim:    r.NewStyle().Faint(true),
		normal: r.NewStyle(),
		// 256-color palette indices that look reasonable on both light and dark
		// terminal backgrounds. Major = red (breaking), minor = green (feature),
		// patch = cyan (fix). Unknown falls through to dim.
		major:     r.NewStyle().Foreground(lipgloss.Color("9")),
		minor:     r.NewStyle().Foreground(lipgloss.Color("10")),
		patch:     r.NewStyle().Foreground(lipgloss.Color("14")),
		trunkLine: "│",
		link:      link,
	}
}

// styleForKind returns the foreground style for a semver category.
// Unknown returns the dim style so unknown types render grey.
func (s styles) styleForKind(k change.SemVerKind) lipgloss.Style {
	switch k {
	case change.SemVerMajor:
		return s.major
	case change.SemVerMinor:
		return s.minor
	case change.SemVerPatch:
		return s.patch
	}
	return s.dim
}

// highestKind returns the most significant semver kind across the given
// change types. Used to pick a single color per row when there are multiple
// types attached.
func highestKind(types []change.Type) change.SemVerKind {
	out := change.SemVerUnknown
	for _, t := range types {
		if t.Kind > out {
			out = t.Kind
		}
	}
	return out
}

// writeTopAnchor writes the top version anchor line.
// Uses ◇ (open diamond) when speculated, ◆ (filled diamond) otherwise.
// The date (and optional "(speculated)") is rendered dim.
func writeTopAnchor(w io.Writer, d release.Description, st styles) error {
	glyph := "◆"
	if d.Speculated {
		glyph = "◇"
	}

	date := d.Date.Format("2006-01-02")
	dateText := date
	if d.Speculated {
		dateText = date + " (speculated)"
	}

	line := fmt.Sprintf("%s  %s  %s",
		glyph,
		d.Version,
		st.dim.Render("‹"+dateText+"›"),
	)
	_, err := fmt.Fprintln(w, line)
	return err
}

// writeBottomAnchor writes the bottom version anchor for PreviousRelease.
// The previous release is always a real tag so we always use ◆.
func writeBottomAnchor(w io.Writer, prev *release.Release, st styles) error {
	date := prev.Date.Format("2006-01-02")
	line := fmt.Sprintf("%s  %s  %s",
		"◆",
		prev.Version,
		st.dim.Render("‹"+date+"›"),
	)
	_, err := fmt.Fprintln(w, line)
	return err
}

// shortHash returns the first 7 characters of a commit hash.
func shortHash(hash string) string {
	if len(hash) <= 7 {
		return hash
	}
	return hash[:7]
}

// joinChangeTypes returns a comma-joined list of change type names.
func joinChangeTypes(types []change.Type) string {
	names := make([]string, 0, len(types))
	for _, t := range types {
		names = append(names, t.Name)
	}
	return strings.Join(names, ", ")
}

// padToVisibleWidth pads content with trailing spaces so that the *visible*
// (non-escape-code) width totals targetWidth. visibleWidth is the caller-known
// printed width of content; this is needed because content may contain OSC 8
// or SGR escape sequences that don't print but do increase len().
func padToVisibleWidth(content string, visibleWidth, targetWidth int) string {
	if visibleWidth >= targetWidth {
		return content
	}
	return content + strings.Repeat(" ", targetWidth-visibleWidth)
}

// closeRef is one entry in the closes column: the visible label (e.g. "#450")
// and the URL it should link to.
type closeRef struct {
	text string
	url  string
}

// closeRefsVisibleWidth returns the visible width of the comma-joined refs.
func closeRefsVisibleWidth(refs []closeRef) int {
	if len(refs) == 0 {
		return 0
	}
	n := 0
	for _, r := range refs {
		n += len(r.text)
	}
	n += (len(refs) - 1) * 2 // ", " separators
	return n
}

// renderCloseRefs returns the comma-joined string with each label hyperlinked
// (when its url is non-empty and the styles support links).
func renderCloseRefs(st styles, refs []closeRef) string {
	if len(refs) == 0 {
		return ""
	}
	parts := make([]string, len(refs))
	for i, r := range refs {
		parts[i] = st.link(r.text, r.url)
	}
	return strings.Join(parts, ", ")
}
