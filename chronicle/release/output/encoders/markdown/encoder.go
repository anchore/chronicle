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

	var refs string
	for _, ref := range summary.References {
		switch {
		case ref.URL == "":
			refs += fmt.Sprintf(" %s", ref.Text)
		case strings.HasPrefix(ref.Text, "@") && strings.HasPrefix(ref.URL, "https://github.com/"):
			// the github release page automatically credits contributors as a footer; embedding the
			// handle in a link suppresses that, so we leave bare @-handles alone.
			refs += fmt.Sprintf(" %s", ref.Text)
		default:
			refs += fmt.Sprintf(" [%s](%s)", ref.Text, ref.URL)
		}
	}

	refs = strings.TrimSpace(refs)
	if refs != "" {
		result += fmt.Sprintf(" [%s]", refs)
	}

	return result + "\n"
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
