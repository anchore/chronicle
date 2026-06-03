// Package slack encodes a release description as Slack "mrkdwn" text suitable
// for the `text` field of a Slack webhook payload. It mirrors the markdown
// encoder but swaps in Slack's flavor: `<url|text>` links, `*bold*` section
// labels (Slack has no real headers in text), and `•` bullets.
package slack

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
)

// ID is the registered name for this encoder.
const ID = "slack"

type Encoder struct{}

func (e *Encoder) ID() string { return ID }

func (e *Encoder) Encode(w io.Writer, title string, d release.Description) error {
	// title supports templating against the description (e.g. `{{ .Version }}`),
	// so it must be rendered before the body is assembled.
	resolvedTitle, err := renderTitle(title, d)
	if err != nil {
		return err
	}

	var out strings.Builder
	if resolvedTitle != "" {
		fmt.Fprintf(&out, "*%s*\n\n", escapeMrkdwn(resolvedTitle))
	}

	if sections := formatChangeSections(d.SupportedChanges, d.Changes, d.ConventionalCommitTypes); sections != "" {
		out.WriteString(sections)
		out.WriteString("\n\n")
	}

	fmt.Fprintf(&out, "*<%s|Full Changelog>*\n", d.VCSChangesURL)

	_, err = io.WriteString(w, out.String())
	return err
}

func renderTitle(raw string, d release.Description) (string, error) {
	t, err := template.New("title").Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parsing title template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, d); err != nil {
		return "", fmt.Errorf("executing title template: %w", err)
	}
	return buf.String(), nil
}

func formatChangeSections(sections []change.TypeTitle, changes change.Changes, recognizedTypes []string) string {
	var result string
	for _, section := range sections {
		summaries := changes.ByChangeType(section.ChangeType)
		if len(summaries) > 0 {
			result += formatChangeSection(section.Title, summaries, recognizedTypes) + "\n"
		}
	}
	return strings.TrimRight(result, "\n")
}

func formatChangeSection(title string, summaries []change.Change, recognizedTypes []string) string {
	result := fmt.Sprintf("*%s*\n", title)
	for _, summary := range summaries {
		result += formatSummary(summary, recognizedTypes)
	}
	return result
}

func formatSummary(summary change.Change, recognizedTypes []string) string {
	text := change.TrimConventionalCommitPrefix(strings.TrimSpace(summary.Text), recognizedTypes...)
	if endsWithPunctuation(text) {
		text = text[:len(text)-1]
	}
	// escape only after trimming punctuation, otherwise the trim could lop the
	// trailing ";" off an escaped entity (e.g. "&gt;").
	result := fmt.Sprintf("• %s", escapeMrkdwn(text))

	return result + formatReferences(summary.References) + "\n"
}

// formatReferences groups references by kind and renders them as space-prefixed
// bracketed groups: `[Issue ...] [PR ... @handles] [other]`, matching the
// markdown encoder's bundling rules but with Slack link syntax.
func formatReferences(refs []change.Reference) string {
	if len(refs) == 0 {
		return ""
	}

	var issues, prs, handles, others []string
	for _, ref := range refs {
		frag := renderRef(ref)
		switch {
		case strings.HasPrefix(ref.Text, "@"):
			handles = append(handles, frag)
		case strings.Contains(ref.URL, "/issues/"):
			issues = append(issues, frag)
		case strings.Contains(ref.URL, "/pull/"):
			prs = append(prs, frag)
		default:
			others = append(others, frag)
		}
	}

	// bundle handles into the PR group if present, else the Issue group, else standalone.
	switch {
	case len(prs) > 0:
		prs = append(prs, handles...)
		handles = nil
	case len(issues) > 0:
		issues = append(issues, handles...)
		handles = nil
	}

	var out strings.Builder
	if len(issues) > 0 {
		fmt.Fprintf(&out, " [Issue %s]", strings.Join(issues, " "))
	}
	if len(prs) > 0 {
		fmt.Fprintf(&out, " [PR %s]", strings.Join(prs, " "))
	}
	if len(handles) > 0 {
		fmt.Fprintf(&out, " [%s]", strings.Join(handles, " "))
	}
	if len(others) > 0 {
		fmt.Fprintf(&out, " [%s]", strings.Join(others, " "))
	}
	return out.String()
}

// renderRef renders a single reference to its Slack fragment. Unlike the
// markdown encoder, @-handles are rendered as backticked plain text rather
// than links: Slack only auto-credits contributors when it recognizes a real
// mention (`<@USER_ID>`), which we don't have from a github user URL, so a
// linked handle adds nothing. Backticks set the handle apart visually instead.
func renderRef(ref change.Reference) string {
	switch {
	case strings.HasPrefix(ref.Text, "@"):
		return fmt.Sprintf("`%s`", escapeMrkdwn(ref.Text))
	case ref.URL == "":
		return escapeMrkdwn(ref.Text)
	default:
		return fmt.Sprintf("<%s|%s>", ref.URL, escapeMrkdwn(ref.Text))
	}
}

// mrkdwnEscaper escapes the only three characters Slack reserves in mrkdwn
// text. & must be listed first so its replacement isn't re-escaped; NewReplacer
// scans left-to-right without re-scanning, so this is single-pass safe.
var mrkdwnEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// escapeMrkdwn escapes change/title display text so characters like `<` and `&`
// aren't mistaken for Slack link or mention syntax. It must never be applied to
// the `<url|text>` link wrappers we build, only to the text inside them.
func escapeMrkdwn(s string) string {
	return mrkdwnEscaper.Replace(s)
}

func endsWithPunctuation(s string) bool {
	if len(s) == 0 {
		return false
	}
	return strings.Contains("!.?", s[len(s)-1:]) //nolint:gocritic
}
