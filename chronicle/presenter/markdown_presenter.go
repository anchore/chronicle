package presenter

import (
	"fmt"
	"io"
	"sort"
	"text/template"

	"github.com/anchore/chronicle/chronicle/release"

	"github.com/anchore/chronicle/chronicle/release/change"
)

const (
	markdownHeaderTemplate = `# {{.Title}}

## [{{.Version}}]({{.VCSTagURL}}) ({{.Date}})

[Full Changelog]({{.VCSChangesURL}})

{{ formatChangeSections .Changes }}
`
)

var _ Presenter = (*MarkdownPresenter)(nil)

type MarkdownPresenter struct {
	config    MarkdownConfig
	templater *template.Template
}

type MarkdownConfig struct {
	release.Release
	release.Description
	Title string
}

func NewMarkdownPresenter(config MarkdownConfig) (*MarkdownPresenter, error) {
	funcMap := template.FuncMap{
		"formatChangeSections": formatChangeSections,
	}
	templater, err := template.New("markdown").Funcs(funcMap).Parse(markdownHeaderTemplate)
	if err != nil {
		return nil, fmt.Errorf("unable to parse markdown presenter template: %w", err)
	}
	return &MarkdownPresenter{
		config:    config,
		templater: templater,
	}, nil
}

func (m MarkdownPresenter) Present(writer io.Writer) error {
	return m.templater.Execute(writer, m.config)
}

func formatChangeSections(changes change.Summaries) string {
	sortedChangeTypes := make([]change.Type, len(change.AllChangeTypes))
	copy(sortedChangeTypes, change.AllChangeTypes)
	// TODO: move to change type definition
	sort.Slice(sortedChangeTypes, func(i, j int) bool {
		// TODO: allow for presentation config for this order
		return string(sortedChangeTypes[i]) < string(sortedChangeTypes[j])
	})

	var result string
	for _, changeType := range sortedChangeTypes {
		summaries := changes.ByChangeType(changeType)
		if len(summaries) > 0 {
			result += formatChangeSection(changeType, summaries) + "\n"
		}
	}
	return result
}

func formatChangeSection(change change.Type, summaries []change.Summary) string {
	result := fmt.Sprintf("### %s\n\n", string(change))
	for _, summary := range summaries {
		result += formatSummary(summary)
	}
	return result
}

func formatSummary(summary change.Summary) string {
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
