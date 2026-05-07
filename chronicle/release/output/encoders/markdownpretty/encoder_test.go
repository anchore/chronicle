package markdownpretty

import (
	"bytes"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
)

func sampleDescription() release.Description {
	return release.Description{
		Release:       release.Release{Version: "v1.2.3"},
		VCSChangesURL: "https://example.com/compare/v1.2.2...v1.2.3",
		SupportedChanges: []change.TypeTitle{
			{ChangeType: change.NewType("bug", change.SemVerPatch), Title: "Bug Fixes"},
		},
		Changes: []change.Change{
			{
				ChangeTypes: []change.Type{change.NewType("bug", change.SemVerPatch)},
				Text:        "fix something important",
				References: []change.Reference{
					{Text: "#4708", URL: "https://github.com/example/repo/issues/4708"},
				},
			},
		},
	}
}

func TestEncoder_NonTTY_FallsBackToPlainMarkdown(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, (&Encoder{IsTTY: false}).Encode(&buf, "My Title", sampleDescription()))

	out := buf.String()

	// plain markdown structure should be present
	require.Contains(t, out, "# My Title")
	require.Contains(t, out, "### Bug Fixes")
	require.Contains(t, out, "fix something important")

	// no ANSI escape sequences should appear in the fallback path
	require.False(t, strings.Contains(out, "\x1b["), "fallback output should not contain ANSI escapes")
}

func TestEncoder_TTY_ProducesANSI(t *testing.T) {
	// glamour's `auto` style still emits ANSI codes in non-tty contexts when we
	// ask for them; we don't need a real TTY for the test, just to flip the flag.
	var buf bytes.Buffer
	require.NoError(t, (&Encoder{IsTTY: true}).Encode(&buf, "My Title", sampleDescription()))

	out := buf.String()
	require.Contains(t, out, "fix something important", "rendered text should still contain the body")
	// some indicator of glamour rendering — either ANSI escapes or a styled
	// header transformation. The plain output begins with `# My Title`; glamour
	// turns that into `# My Title` with surrounding styling and paddings.
	// Easiest stable assertion: the byte length differs from plain markdown.
	var plain bytes.Buffer
	require.NoError(t, (&Encoder{IsTTY: false}).Encode(&plain, "My Title", sampleDescription()))
	require.NotEqual(t, plain.String(), out, "TTY output should differ from plain-markdown fallback")
}

func TestEncoder_StdoutOnly(t *testing.T) {
	require.True(t, (&Encoder{}).StdoutOnly())
}

func TestEncoder_TTY_EmitsOSC8Hyperlinks(t *testing.T) {
	t.Setenv("GLAMOUR_STYLE", "pink") // pin the active theme so SGR codes are predictable
	var buf bytes.Buffer
	require.NoError(t, (&Encoder{IsTTY: true}).Encode(&buf, "Title", sampleDescription()))
	out := buf.String()

	// reference link should appear as a clickable OSC 8 hyperlink wrapping
	// just the text "#4708" with theme link_text styling (bold under pink).
	require.Contains(t, out, "\x1b]8;;https://github.com/example/repo/issues/4708\x07\x1b[1m#4708\x1b[22m\x1b]8;;\x07",
		"expected OSC 8 wrapping bold #4708 with the reference URL")

	// no placeholder fragments should leak into the rendered output.
	require.NotContains(t, out, openPrefix, "placeholder open marker leaked into output")
	require.NotContains(t, out, closePrefix, "placeholder close marker leaked into output")
}

func TestLinkTextSGR(t *testing.T) {
	tests := []struct {
		style   string
		wantOn  string
		wantOff string
	}{
		// pink and dark/light all bold link_text; dark/light additionally color
		// it but linkTextSGR intentionally skips color (terminals add their own).
		{style: "pink", wantOn: "\x1b[1m", wantOff: "\x1b[22m"},
		{style: "dark", wantOn: "\x1b[1m", wantOff: "\x1b[22m"},
		{style: "light", wantOn: "\x1b[1m", wantOff: "\x1b[22m"},
		// dracula uses color only — no Bold/Italic/Underline → empty SGR.
		{style: "dracula", wantOn: "", wantOff: ""},
		// unknown styles fall back to the default (pink).
		{style: "no-such-style", wantOn: "\x1b[1m", wantOff: "\x1b[22m"},
	}
	for _, tt := range tests {
		t.Run(tt.style, func(t *testing.T) {
			on, off := linkTextSGR(tt.style)
			require.Equal(t, tt.wantOn, on)
			require.Equal(t, tt.wantOff, off)
		})
	}
}

func TestEncoder_NonTTY_HasNoOSC8Hyperlinks(t *testing.T) {
	var buf bytes.Buffer
	require.NoError(t, (&Encoder{IsTTY: false}).Encode(&buf, "Title", sampleDescription()))
	require.NotContains(t, buf.String(), "\x1b]8;;", "OSC 8 escapes must not appear in plain markdown fallback")
}

func TestResolveStyle(t *testing.T) {
	tests := []struct {
		name string
		env  string
		want string
	}{
		{name: "unset uses pink default", env: "", want: "pink"},
		{name: "GLAMOUR_STYLE wins", env: "dracula", want: "dracula"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("GLAMOUR_STYLE", tt.env)
			require.Equal(t, tt.want, resolveStyle())
		})
	}
}

func TestSubstituteAndReplace_RoundTrip(t *testing.T) {
	in := "see [#1](https://example.com/1) and [#2](https://example.com/2) for details"

	src, urls := substituteLinksWithPlaceholders(in)
	require.Equal(t, []string{"https://example.com/1", "https://example.com/2"}, urls)
	require.NotContains(t, src, "[#1](", "markdown link syntax should be gone after substitution")
	require.NotContains(t, src, "[#2](", "markdown link syntax should be gone after substitution")
	require.NotContains(t, src, "https://example.com", "URL bytes should not appear in markdown source (avoids autolink)")

	// after the substitution → glamour → replacement round-trip, the OSC 8
	// escape sequences should wrap each link's text and the URL should be
	// embedded in the escape, not displayed inline. Empty SGR codes here
	// keep the round-trip assertion focused on the OSC 8 framing itself.
	got := replacePlaceholdersWithOSC8(src, urls, "", "")
	require.Contains(t, got, "\x1b]8;;https://example.com/1\x07#1\x1b]8;;\x07")
	require.Contains(t, got, "\x1b]8;;https://example.com/2\x07#2\x1b]8;;\x07")

	// non-link content survives untouched
	require.Contains(t, got, "see ")
	require.Contains(t, got, " and ")
	require.Contains(t, got, " for details")
}
