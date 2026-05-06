package json

import (
	"encoding/json"
	"io"

	"github.com/anchore/chronicle/chronicle/release"
)

// ID is the registered name for this encoder.
const ID = "json"

type Encoder struct{}

func (e *Encoder) ID() string { return ID }

func (e *Encoder) Encode(w io.Writer, _ string, d release.Description) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(d)
}
