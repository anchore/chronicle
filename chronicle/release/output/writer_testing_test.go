package output

import "io"

// stdoutSink wraps an io.Writer so it can sit alongside fileSinks behind the
// same interface. Used by tests that want to capture the stdout stream
// directly into a buffer; production paths use publisherSink instead.
type stdoutSink struct {
	w io.Writer
}

func (s *stdoutSink) Write(p []byte) (int, error) { return s.w.Write(p) }
func (s *stdoutSink) Commit() error               { return nil }
func (s *stdoutSink) Abort() error                { return nil }

// newWithStdout is a test seam; production code calls New. The stdout sink
// writes directly to the supplied io.Writer rather than going through the bus
// so tests can assert byte-exact content via a bytes.Buffer.
func newWithStdout(specs []Spec, encs Encoders, stdout io.Writer) (Writer, error) {
	return newWithSink(specs, encs, func() sink { return &stdoutSink{w: stdout} })
}
