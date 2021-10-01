package cmd

import (
	"fmt"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/presenter"
	"github.com/anchore/chronicle/chronicle/release/presenter/json"
	"github.com/anchore/chronicle/chronicle/release/presenter/markdown"
)

type presentationTask func(description release.Description) (release.Presenter, error)

func selectPresenter(format presenter.Format) (presentationTask, error) {
	switch format {
	case presenter.MarkdownFormat:
		return presentMarkdown, nil
	case presenter.JSONFormat:
		return presentJSON, nil
	default:
		return nil, fmt.Errorf("unsupported output format: %+v", format)
	}
}

func presentMarkdown(description release.Description) (release.Presenter, error) {
	return markdown.NewMarkdownPresenter(markdown.Config{
		Description: description,
		Title:       appConfig.Title,
	})
}

func presentJSON(description release.Description) (release.Presenter, error) {
	return json.NewJSONPresenter(description)
}
