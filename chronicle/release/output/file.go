package output

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/anchore/chronicle/internal/bus"
)

// fileSink writes to a temp file in the destination's parent directory and
// renames into place on Commit. On Abort it removes the temp file. This
// guarantees the final path is either fully written or untouched, so a failed
// run never leaves a half-written CHANGELOG on disk.
type fileSink struct {
	finalPath string
	tmp       *os.File
	committed bool
	aborted   bool
}

const (
	dirPerm  = 0o755
	filePerm = 0o644
)

func newFileSink(path string) (*fileSink, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, dirPerm); err != nil {
		return nil, fmt.Errorf("creating output directory %q: %w", dir, err)
	}
	// pattern keeps the artifact recognizable if a crash leaves it behind, and
	// uses the same dir so os.Rename is an atomic move on the same filesystem.
	pattern := ".chronicle-" + filepath.Base(path) + "-*.tmp"
	tmp, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, fmt.Errorf("creating temp file in %q: %w", dir, err)
	}
	return &fileSink{finalPath: path, tmp: tmp}, nil
}

func (s *fileSink) Write(p []byte) (int, error) {
	return s.tmp.Write(p)
}

// Commit flushes the temp file and renames it onto finalPath. Idempotent.
func (s *fileSink) Commit() error {
	if s.committed || s.aborted {
		return nil
	}
	s.committed = true
	if err := s.tmp.Sync(); err != nil {
		_ = s.tmp.Close()
		_ = os.Remove(s.tmp.Name())
		return fmt.Errorf("sync %q: %w", s.tmp.Name(), err)
	}
	if err := s.tmp.Close(); err != nil {
		_ = os.Remove(s.tmp.Name())
		return fmt.Errorf("close %q: %w", s.tmp.Name(), err)
	}
	if err := os.Chmod(s.tmp.Name(), filePerm); err != nil {
		_ = os.Remove(s.tmp.Name())
		return fmt.Errorf("chmod %q: %w", s.tmp.Name(), err)
	}
	if err := os.Rename(s.tmp.Name(), s.finalPath); err != nil {
		_ = os.Remove(s.tmp.Name())
		return fmt.Errorf("rename to %q: %w", s.finalPath, err)
	}
	return nil
}

// Abort closes and removes the temp file. Idempotent.
func (s *fileSink) Abort() error {
	if s.committed || s.aborted {
		return nil
	}
	s.aborted = true
	_ = s.tmp.Close()
	if err := os.Remove(s.tmp.Name()); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("removing temp file %q: %w", s.tmp.Name(), err)
	}
	return nil
}

// publisherSink buffers all encoder output until Commit, then emits it as a
// CLIReportType event via the bus. This keeps stdout sacred while a TUI is
// running on stderr — bytes only land on os.Stdout post-teardown via the UI's
// finalize path, where they cannot interleave with bubbletea repaints.
type publisherSink struct {
	buf bytes.Buffer
}

func (s *publisherSink) Write(p []byte) (int, error) { return s.buf.Write(p) }

// Commit publishes the buffered content. Idempotent — subsequent calls are no-ops.
func (s *publisherSink) Commit() error {
	if s.buf.Len() == 0 {
		return nil
	}
	bus.Report(s.buf.String())
	s.buf.Reset()
	return nil
}

func (s *publisherSink) Abort() error { return nil }

// sink is the destination side of an (encoder, destination) pair.
type sink interface {
	io.Writer
	Commit() error
	Abort() error
}
