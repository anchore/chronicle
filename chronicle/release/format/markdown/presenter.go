package markdown

import (
	"fmt"
	"io"
	"text/template"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/wagoodman/go-presenter"
)

const (
	markdownHeaderTemplate = `# {{.Title}}

## [{{.Version}}]({{.VCSTagURL}}) ({{.Date}})

[Full Changelog]({{.VCSChangesURL}})

{{ formatChangeSections .Changes }}
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
	return result
}

func formatChangeSection(title string, summaries []change.Change) string {
	result := fmt.Sprintf("### %s\n\n", title)
	for _, summary := range summaries {
		result += formatSummary(summary)
	}
	return result
}

func formatSummary(summary change.Change) string {
	result := fmt.Sprintf("- %s", summary.Text)
	for _, ref := range summary.References {
		if ref.URL == "" {
			result += fmt.Sprintf(" [%s]", ref.Text)
		} else {
			result += fmt.Sprintf(" [[%s](%s)]", ref.Text, ref.URL)
		}
	}

	return result + "\n"
}
