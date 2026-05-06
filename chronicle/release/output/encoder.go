package output

import (
	"io"
	"sort"

	"github.com/anchore/chronicle/chronicle/release"
)

// Encoder turns a release.Description into bytes for one named format.
// Title is threaded through because the markdown encoder needs it; encoders
// that don't care are free to ignore it.
type Encoder interface {
	ID() string
	Encode(w io.Writer, title string, d release.Description) error
}

// Encoders is a name-keyed set of available encoders. Callers (typically the
// cmd layer) construct this once with the encoders the command supports and
// pass it into New.
type Encoders map[string]Encoder

// NewEncoders builds an Encoders set from a slice, keyed by each encoder's ID.
// Duplicate IDs are not allowed and panic — that's a programmer error.
func NewEncoders(encs ...Encoder) Encoders {
	out := make(Encoders, len(encs))
	for _, e := range encs {
		if _, exists := out[e.ID()]; exists {
			panic("output: duplicate encoder ID " + e.ID())
		}
		out[e.ID()] = e
	}
	return out
}

func (e Encoders) Lookup(id string) (Encoder, bool) {
	enc, ok := e[id]
	return enc, ok
}

func (e Encoders) Names() []string {
	names := make([]string, 0, len(e))
	for k := range e {
		names = append(names, k)
	}
	sort.Strings(names)
	return names
}
