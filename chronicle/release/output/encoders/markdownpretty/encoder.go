// Package markdownpretty renders chronicle's markdown changelog as ANSI-styled
// terminal output via charmbracelet/glamour. It is intentionally stdout-only
// and falls back to plain markdown when the destination is not a TTY (so that
// piped/redirected output stays useful).
//
// =============================================================================
// HACK WARNING — please read before changing anything below.
// =============================================================================
//
// Chronicle's reference links normally show up as `text URL` side-by-side in
// glamour's default rendering, which is noisy and not clickable. The "right"
// fix is upstream: charmbracelet/glamour#531 adds a `WithHiddenLinks()` option
// that tells glamour to skip the URL display entirely so the link text on its
// own becomes a clickable OSC 8 hyperlink (glamour v2 already emits those
// natively per PR #411). When that PR merges and we move to glamour v2:
//
//  1. delete substituteLinksWithPlaceholders / replacePlaceholdersWithOSC8
//  2. delete the linkTextSGR / resolveStyleConfig helpers
//  3. delete openOSC8 / closeOSC8 and the openPrefix/closePrefix constants
//  4. add `glamour.WithHiddenLinks()` to NewTermRenderer
//  5. drop the muesli/termenv and golang.org/x/term direct imports
//
// Until then, we run a two-phase substitution:
//
//  1. Pre-process: every `[text](url)` in the markdown source is replaced
//     with alphanumeric placeholder strings (XCHRONICLELINKOPEN0X / CLOSE).
//     We can't inject OSC 8 escape sequences directly into the source
//     because glamour hardcodes goldmark's GFM autolinker; bare URL bytes
//     get captured into a LinkElement that glamour then re-renders with its
//     own styling, fragmenting our escape across SGR boundaries.
//  2. Glamour renders the placeholders as plain text — no autolink trigger,
//     no LinkElement, so the link text stays atomic.
//  3. Post-process: each placeholder pair becomes an OSC 8 hyperlink escape.
//     We also emit SGR codes matching the active theme's `link_text` style
//     (Bold/Italic/Underline) inside the escape, because glamour's `link_text`
//     styling never got applied to a real link element.
//
// This is fragile in the usual ways: tied to glamour internals, depends on
// placeholder strings not appearing in user content, and duplicates style
// resolution that glamour already does internally. The whole file is a
// stand-in. Replace with #531 as soon as possible.
// =============================================================================
package markdownpretty

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/glamour/ansi"
	"github.com/charmbracelet/glamour/styles"
	"github.com/muesli/termenv"
	"golang.org/x/term"

	"github.com/anchore/chronicle/chronicle/release"
	mdenc "github.com/anchore/chronicle/chronicle/release/output/encoders/markdown"
)

// ID is the registered name for this encoder.
const ID = "md-pretty"

// defaultStyle is the fallback glamour style when GLAMOUR_STYLE is unset.
// "pink" is bright and visible across most terminal backgrounds.
const defaultStyle = "pink"

// the placeholder strings live in the markdown source between the two phases.
// They are alphanumeric only so goldmark's autolinker / parser leaves them
// alone and glamour's text wrapper preserves them verbatim. The "X" delimiters
// give us a stable boundary for the post-process regex even when glamour
// splits surrounding text into multiple SGR-wrapped tokens.
const (
	openPrefix  = "XCHRONICLELINKOPEN"
	closePrefix = "XCHRONICLELINKCLOSE"
	suffix      = "X"
)

var (
	// mdLinkPattern matches a standard markdown inline link `[text](url)`.
	// The text class forbids both `[` and `]` so we don't accidentally swallow
	// the outer brackets that chronicle's markdown wraps reference lists in
	// (e.g. `[[#1234](url)]` — only the inner `[#1234](url)` should match).
	mdLinkPattern = regexp.MustCompile(`\[([^][]+)]\(([^)]+)\)`)

	openPlaceholder  = regexp.MustCompile(openPrefix + `(\d+)` + suffix)
	closePlaceholder = regexp.MustCompile(closePrefix + `(\d+)` + suffix)
)

// Encoder renders the markdown changelog through glamour for ANSI styling.
//
// IsTTY is set by the caller (typically the cmd layer, once at startup) and
// indicates whether the destination is a terminal capable of rendering ANSI.
// When false, Encode emits plain markdown so piped or redirected output stays
// machine-parseable.
type Encoder struct {
	IsTTY bool
}

func (e *Encoder) ID() string { return ID }

// StdoutOnly tells the writer to reject specs like `md-pretty=path` — ANSI
// escape sequences in a file are rarely what anyone wants.
func (e *Encoder) StdoutOnly() bool { return true }

func (e *Encoder) Encode(w io.Writer, title string, d release.Description) error {
	// always render plain markdown first; if we're not on a TTY, that's the
	// final output. Otherwise it becomes glamour's input.
	var raw bytes.Buffer
	if err := (&mdenc.Encoder{}).Encode(&raw, title, d); err != nil {
		return err
	}

	if !e.IsTTY {
		_, err := w.Write(raw.Bytes())
		return err
	}

	src, urls := substituteLinksWithPlaceholders(raw.String())
	style := resolveStyle()
	sgrOpen, sgrClose := linkTextSGR(style)

	// WordWrap=0 disables glamour's default 80-column wrap so long reference
	// lists stay on one line. resolveStyle honors GLAMOUR_STYLE (shared
	// convention with glow, gh, etc.) and falls back to defaultStyle.
	r, err := glamour.NewTermRenderer(
		glamour.WithStylePath(style),
		glamour.WithWordWrap(0),
	)
	if err != nil {
		return fmt.Errorf("constructing glamour renderer: %w", err)
	}
	defer func() { _ = r.Close() }()

	rendered, err := r.Render(src)
	if err != nil {
		return fmt.Errorf("rendering markdown with glamour: %w", err)
	}

	_, err = io.WriteString(w, replacePlaceholdersWithOSC8(rendered, urls, sgrOpen, sgrClose))
	return err
}

func resolveStyle() string {
	if s := os.Getenv("GLAMOUR_STYLE"); s != "" {
		return s
	}
	return defaultStyle
}

// substituteLinksWithPlaceholders replaces every `[text](url)` in the source
// with `<OPEN_N>text<CLOSE_N>` and returns the parallel URL slice. The
// placeholders pass through goldmark and glamour as plain text — no autolink
// triggering, no link element rendering — so the link text stays atomic.
func substituteLinksWithPlaceholders(md string) (string, []string) {
	var urls []string
	out := mdLinkPattern.ReplaceAllStringFunc(md, func(match string) string {
		m := mdLinkPattern.FindStringSubmatch(match)
		text, url := m[1], m[2]
		idx := len(urls)
		urls = append(urls, url)
		return fmt.Sprintf("%s%d%s%s%s%d%s", openPrefix, idx, suffix, text, closePrefix, idx, suffix)
	})
	return out, urls
}

// replacePlaceholdersWithOSC8 swaps each placeholder pair in the rendered
// output for the corresponding OSC 8 hyperlink escape. We do this *after*
// glamour has applied its SGR styling so the link text picks up whatever
// foreground color the active style chose for plain text — and the surrounding
// SGR codes inside the OSC 8 region are harmless: terminals process SGR and
// OSC 8 independently.
func replacePlaceholdersWithOSC8(rendered string, urls []string, sgrOpen, sgrClose string) string {
	rendered = openPlaceholder.ReplaceAllStringFunc(rendered, func(m string) string {
		idx := mustParseIndex(openPlaceholder.FindStringSubmatch(m)[1])
		if idx < 0 || idx >= len(urls) {
			return m
		}
		return openOSC8(urls[idx], sgrOpen)
	})
	rendered = closePlaceholder.ReplaceAllStringFunc(rendered, func(m string) string {
		idx := mustParseIndex(closePlaceholder.FindStringSubmatch(m)[1])
		if idx < 0 || idx >= len(urls) {
			return m
		}
		return closeOSC8(sgrClose)
	})
	return rendered
}

// openOSC8 / closeOSC8 emit the OSC 8 hyperlink framing plus SGR styling that
// matches the active theme's `link_text` primitive — without that styling our
// placeholder substitution would lose the visual cue glamour normally applies
// to link text (pink/dark/light all bold it; dracula colors it).
//
// OSC 8 format: ESC ]8;;URL BEL <text> ESC ]8;; BEL
//
// We use BEL (0x07) as the terminator rather than ST (ESC \). The OSC 8 spec
// accepts both; every terminal that supports OSC 8 also accepts BEL; and the
// ST form's backslash gets eaten by markdown escape parsing if it ever leaks
// back into a markdown context. Terminals that don't support OSC 8 typically
// swallow the escape silently, leaving plain link text behind.
func openOSC8(url, sgrOpen string) string {
	return "\x1b]8;;" + url + "\x07" + sgrOpen
}

func closeOSC8(sgrClose string) string {
	return sgrClose + "\x1b]8;;\x07"
}

// linkTextSGR returns SGR open/close codes that replicate the active theme's
// `link_text` styling for Bold, Italic, and Underline. Color isn't replicated
// because terminals typically apply their own coloring to OSC 8 hyperlinks
// and we don't want to fight them on it.
func linkTextSGR(styleName string) (sgrOpen, sgrClose string) {
	cfg := resolveStyleConfig(styleName)
	if cfg == nil {
		return "", ""
	}
	p := cfg.LinkText

	var on, off []string
	if p.Bold != nil && *p.Bold {
		on = append(on, "1")
		off = append(off, "22")
	}
	if p.Italic != nil && *p.Italic {
		on = append(on, "3")
		off = append(off, "23")
	}
	if p.Underline != nil && *p.Underline {
		on = append(on, "4")
		off = append(off, "24")
	}
	if len(on) == 0 {
		return "", ""
	}
	return "\x1b[" + strings.Join(on, ";") + "m", "\x1b[" + strings.Join(off, ";") + "m"
}

// resolveStyleConfig mirrors glamour's own style resolution so we can read
// the same StyleConfig glamour will apply. "auto" maps to dark/light based
// on the terminal background; unknown styles fall back to the default.
func resolveStyleConfig(name string) *ansi.StyleConfig {
	if name == "auto" {
		if !term.IsTerminal(int(os.Stdout.Fd())) {
			return &styles.NoTTYStyleConfig
		}
		if termenv.HasDarkBackground() {
			return &styles.DarkStyleConfig
		}
		return &styles.LightStyleConfig
	}
	if cfg, ok := styles.DefaultStyles[name]; ok {
		return cfg
	}
	return styles.DefaultStyles[defaultStyle]
}

func mustParseIndex(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return -1
		}
		n = n*10 + int(c-'0')
	}
	return n
}
