package json

import (
	"encoding/json"
	"io"

	"github.com/anchore/chronicle/chronicle/release"
)

type Presenter struct {
	description release.Description
}

func NewJSONPresenter(description release.Description) (*Presenter, error) {
	return &Presenter{
		description: description,
	}, nil
}

func (m Presenter) Present(writer io.Writer) error {
	enc := json.NewEncoder(writer)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(m.description)
}
