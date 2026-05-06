package version

import (
	"errors"
	"fmt"
	"io"

	"github.com/anchore/chronicle/chronicle/release"
)

// ID is the registered name for this encoder.
const ID = "version"

// Encoder writes only the resolved version string. It exists so callers can
// produce a `VERSION` file alongside (or instead of) the changelog without a
// special-case CLI flag.
type Encoder struct{}

func (e *Encoder) ID() string { return ID }

func (e *Encoder) Encode(w io.Writer, _ string, d release.Description) error {
	if d.Version == "" {
		return errors.New("version is empty (was --speculate-next-version expected?)")
	}
	_, err := fmt.Fprintln(w, d.Version)
	return err
}
