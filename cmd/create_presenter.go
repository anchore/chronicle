package cmd

import (
	"fmt"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/format"
	"github.com/anchore/chronicle/chronicle/release/format/json"
	"github.com/anchore/chronicle/chronicle/release/format/markdown"
	"github.com/wagoodman/go-presenter"
)

type presentationTask func(description release.Description) (presenter.Presenter, error)

func selectPresenter(f format.Format) (presentationTask, error) {
	switch f {
	case format.MarkdownFormat:
		return presentMarkdown, nil
	case format.JSONFormat:
		return presentJSON, nil
	default:
		return nil, fmt.Errorf("unsupported output format: %+v", f)
	}
}

func presentMarkdown(description release.Description) (presenter.Presenter, error) {
	return markdown.NewMarkdownPresenter(markdown.Config{
		Description: description,
		Title:       appConfig.Title,
	})
}

func presentJSON(description release.Description) (presenter.Presenter, error) {
	return json.NewJSONPresenter(description)
}
