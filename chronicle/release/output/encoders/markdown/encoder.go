package markdown

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/leodido/go-conventionalcommits"
	cc "github.com/leodido/go-conventionalcommits/parser"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
)

// ID is the registered name for this encoder.
const ID = "md"

const headerTemplate = `{{if .Title }}# {{.Title}}

{{ end }}{{if .Changes }}{{ formatChangeSections .Changes }}

{{ end }}**[(Full Changelog)]({{.VCSChangesURL}})**
`

type Encoder struct{}

func (e *Encoder) ID() string { return ID }

func (e *Encoder) Encode(w io.Writer, title string, d release.Description) error {
	// title supports templating against the description (e.g. `{{ .Version }}`),
	// so it must be rendered before the body template runs.
	resolvedTitle, err := renderTitle(title, d)
	if err != nil {
		return err
	}

	view := struct {
		release.Description
		Title string
	}{Description: d, Title: resolvedTitle}

	funcMap := template.FuncMap{
		"formatChangeSections": func(changes change.Changes) string {
			return formatChangeSections(d.SupportedChanges, changes)
		},
	}

	tmpl, err := template.New("markdown").Funcs(funcMap).Parse(headerTemplate)
	if err != nil {
		return fmt.Errorf("parsing markdown template: %w", err)
	}
	return tmpl.Execute(w, view)
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

func formatChangeSections(sections []change.TypeTitle, changes change.Changes) string {
	var result string
	for _, section := range sections {
		summaries := changes.ByChangeType(section.ChangeType)
		if len(summaries) > 0 {
			result += formatChangeSection(section.Title, summaries) + "\n"
		}
	}
	return strings.TrimRight(result, "\n")
}

func formatChangeSection(title string, summaries []change.Change) string {
	result := fmt.Sprintf("### %s\n\n", title)
	for _, summary := range summaries {
		result += formatSummary(summary)
	}
	return result
}

func formatSummary(summary change.Change) string {
	result := removeConventionalCommitPrefix(strings.TrimSpace(summary.Text))
	result = fmt.Sprintf("- %s", result)
	if endsWithPunctuation(result) {
		result = result[:len(result)-1]
	}

	return result + formatReferences(summary.References) + "\n"
}

// formatReferences groups references by kind and renders them as space-prefixed
// bracketed groups: `[Issue ...] [PR ... @handles] [other]`. See package docs
// for the full bundling rules.
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

// renderRef renders a single reference to its markdown fragment, preserving
// the bare-text behavior for @-handles linked to a github user page (so the
// github release page can still auto-credit contributors).
func renderRef(ref change.Reference) string {
	switch {
	case ref.URL == "":
		return ref.Text
	case strings.HasPrefix(ref.Text, "@") && strings.HasPrefix(ref.URL, "https://github.com/"):
		return ref.Text
	default:
		return fmt.Sprintf("[%s](%s)", ref.Text, ref.URL)
	}
}

func endsWithPunctuation(s string) bool {
	if len(s) == 0 {
		return false
	}
	return strings.Contains("!.?", s[len(s)-1:]) //nolint:gocritic
}

func removeConventionalCommitPrefix(s string) string {
	res, err := cc.NewMachine(cc.WithTypes(conventionalcommits.TypesConventional)).Parse([]byte(s))
	if err != nil || res == nil || (res != nil && !res.Ok()) {
		// probably not a conventional commit
		return s
	}

	// conventional commits always have a prefix and the message starts after the first ":"
	fields := strings.SplitN(s, ":", 2)
	if len(fields) == 2 {
		return strings.TrimSpace(fields[1])
	}

	return s
}
