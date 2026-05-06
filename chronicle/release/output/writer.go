package output

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/anchore/chronicle/chronicle/release"
)

// Writer fans a release.Description out to N (encoder, sink) pairs.
type Writer interface {
	Write(title string, d release.Description) error
	io.Closer
}

// New constructs a Writer that will route a Description through each spec's
// encoder to that spec's destination. encs is the encoder set the caller
// supports; specs naming an unknown encoder are rejected. File sinks are
// opened up front so we fail fast on permission/path errors before doing any
// real work.
//
// On any subsequent Encode error, Close removes all temp files and returns
// the first encode error joined with any close errors. On success, Close
// renames each temp file into place.
func New(specs []Spec, encs Encoders) (Writer, error) {
	return newWithStdout(specs, encs, os.Stdout)
}

// newWithStdout exists for tests; production code calls New.
func newWithStdout(specs []Spec, encs Encoders, stdout io.Writer) (Writer, error) {
	if err := Validate(specs); err != nil {
		return nil, err
	}
	for _, s := range specs {
		if _, ok := encs.Lookup(s.Name); !ok {
			return nil, fmt.Errorf("unknown output format %q (known: %v)", s.Name, encs.Names())
		}
	}
	w := &multiWriter{}
	for _, s := range specs {
		enc, _ := encs.Lookup(s.Name) // checked above
		var sk sink
		if s.IsStdout() {
			sk = &stdoutSink{w: stdout}
		} else {
			fs, err := newFileSink(s.Path)
			if err != nil {
				// abort sinks already opened so we don't leak temp files
				_ = w.abortAll()
				return nil, err
			}
			sk = fs
		}
		w.pairs = append(w.pairs, pair{enc: enc, sk: sk, spec: s})
	}
	return w, nil
}

type pair struct {
	enc  Encoder
	sk   sink
	spec Spec
}

type multiWriter struct {
	pairs     []pair
	encodeErr error
	closed    bool
}

func (w *multiWriter) Write(title string, d release.Description) error {
	if w.closed {
		return errors.New("output writer is closed")
	}
	for _, p := range w.pairs {
		if err := p.enc.Encode(p.sk, title, d); err != nil {
			err = fmt.Errorf("encoding %s: %w", formatSpec(p.spec), err)
			w.encodeErr = err
			return err
		}
	}
	return nil
}

// Close commits each file sink (rename) on success, or aborts (remove temp)
// if any prior Write failed. Stdout sinks are no-ops.
func (w *multiWriter) Close() error {
	if w.closed {
		return nil
	}
	w.closed = true
	if w.encodeErr != nil {
		_ = w.abortAll()
		return nil
	}
	var errs []error
	for _, p := range w.pairs {
		if err := p.sk.Commit(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

func (w *multiWriter) abortAll() error {
	var errs []error
	for _, p := range w.pairs {
		if err := p.sk.Abort(); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}
