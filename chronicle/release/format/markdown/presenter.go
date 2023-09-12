package markdown

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/wagoodman/go-presenter"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
)

const (
	markdownHeaderTemplate = `{{if .Title }}# {{.Title}}

{{ end }}{{if .Changes }}{{ formatChangeSections .Changes }}

{{ end }}**[(Full Changelog)]({{.VCSChangesURL}})**
`
)

var _ presenter.Presenter = (*Presenter)(nil)

type Presenter struct {
	config    Config
	templater *template.Template
}

type ChangeSection struct {
	ChangeType change.Type
	Title      string
}

type Sections []ChangeSection

type Config struct {
	release.Description
	Title string
}

func NewMarkdownPresenter(config Config) (*Presenter, error) {
	p := Presenter{
		config: config,
	}

	funcMap := template.FuncMap{
		"formatChangeSections": p.formatChangeSections,
	}
	templater, err := template.New("markdown").Funcs(funcMap).Parse(markdownHeaderTemplate)
	if err != nil {
		return nil, fmt.Errorf("unable to parse markdown presenter template: %w", err)
	}

	titleTemplater, err := template.New("title").Funcs(funcMap).Parse(config.Title)
	if err != nil {
		return nil, fmt.Errorf("unable to parse markdown presenter title template: %w", err)
	}

	buf := bytes.Buffer{}
	if err := titleTemplater.Execute(&buf, config); err != nil {
		return nil, fmt.Errorf("unable to template title: %w", err)
	}
	p.config.Title = buf.String()

	p.templater = templater

	return &p, nil
}

func (m Presenter) Present(writer io.Writer) error {
	return m.templater.Execute(writer, m.config)
}

func (m Presenter) formatChangeSections(changes change.Changes) string {
	var result string
	for _, section := range m.config.SupportedChanges {
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
	result := fmt.Sprintf("- %s", strings.TrimSpace(summary.Text))
	if !endsWithPunctuation(result) {
		switch result[len(result)-1:] {
		case "!", ".", "?":
			// pass
		default:
			result += "."
		}
	}

	var refs string
	for _, ref := range summary.References {
		switch {
		case ref.URL == "":
			refs += fmt.Sprintf(" %s", ref.Text)
		case strings.HasPrefix(ref.Text, "@") && strings.HasPrefix(ref.URL, "https://github.com/"):
			// the github release page will automatically show all contributors as a footer. However, if you
			// embed the contributor's github handle in a link, then this feature will not work.
			refs += fmt.Sprintf(" %s", ref.Text)
		default:
			refs += fmt.Sprintf(" [%s](%s)", ref.Text, ref.URL)
		}
	}

	refs = strings.TrimSpace(refs)
	if refs != "" {
		result += fmt.Sprintf(" _(%s)", refs)
	}

	return result + "\n"
}

func endsWithPunctuation(s string) bool {
	if len(s) == 0 {
		return false
	}
	return strings.Contains("!.?", s[len(s)-1:]) //nolint:gocritic
}
