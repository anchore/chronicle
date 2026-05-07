package output

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/anchore/chronicle/chronicle/release"
)

// recordingEncoder writes "<id>:<title>:<version>" so we can assert the same
// Description reaches every sink without dragging real encoders in.
type recordingEncoder struct{ id string }

func (r *recordingEncoder) ID() string { return r.id }
func (r *recordingEncoder) Encode(w io.Writer, title string, d release.Description) error {
	_, err := fmt.Fprintf(w, "%s:%s:%s", r.id, title, d.Version)
	return err
}

type erroringEncoder struct{ id string }

func (e *erroringEncoder) ID() string { return e.id }
func (e *erroringEncoder) Encode(io.Writer, string, release.Description) error {
	return errors.New("boom")
}

func TestWriter_FanOut(t *testing.T) {
	encs := NewEncoders(&recordingEncoder{id: "rec-md"}, &recordingEncoder{id: "rec-json"})

	dir := t.TempDir()
	filePath := filepath.Join(dir, "out.json")

	specs := []Spec{
		{Name: "rec-md"},                   // stdout
		{Name: "rec-json", Path: filePath}, // file
	}

	stdout := &bytes.Buffer{}
	w, err := newWithStdout(specs, encs, stdout)
	require.NoError(t, err)

	desc := release.Description{Release: release.Release{Version: "v1.2.3"}}

	require.NoError(t, w.Write("My Title", desc))
	require.NoError(t, w.Close())

	require.Equal(t, "rec-md:My Title:v1.2.3", stdout.String())

	got, err := os.ReadFile(filePath)
	require.NoError(t, err)
	require.Equal(t, "rec-json:My Title:v1.2.3", string(got))
}

func TestWriter_AbortsTempFilesOnEncodeError(t *testing.T) {
	encs := NewEncoders(&erroringEncoder{id: "rec-err"})

	dir := t.TempDir()
	filePath := filepath.Join(dir, "out.txt")

	w, err := newWithStdout([]Spec{{Name: "rec-err", Path: filePath}}, encs, io.Discard)
	require.NoError(t, err)

	require.Error(t, w.Write("", release.Description{}))
	require.NoError(t, w.Close())

	// no final file
	_, err = os.Stat(filePath)
	require.True(t, os.IsNotExist(err), "expected no final file, got err=%v", err)

	// no leftover temp files
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries)
}

func TestWriter_AbortsAllSinksWhenLaterEncoderFails(t *testing.T) {
	// the first encoder succeeds and writes bytes to its temp file; the
	// second encoder fails. Close must abort *both* sinks so neither final
	// file appears and no temp files leak.
	encs := NewEncoders(&recordingEncoder{id: "ok"}, &erroringEncoder{id: "bad"})

	dir := t.TempDir()
	okPath := filepath.Join(dir, "ok.txt")
	badPath := filepath.Join(dir, "bad.txt")

	specs := []Spec{
		{Name: "ok", Path: okPath},
		{Name: "bad", Path: badPath},
	}
	w, err := newWithStdout(specs, encs, io.Discard)
	require.NoError(t, err)

	require.Error(t, w.Write("t", release.Description{Release: release.Release{Version: "v1"}}))
	require.NoError(t, w.Close())

	for _, p := range []string{okPath, badPath} {
		_, statErr := os.Stat(p)
		require.True(t, os.IsNotExist(statErr), "expected %q to not exist, got err=%v", p, statErr)
	}

	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries, "expected no leftover temp files")
}

func TestNewEncoders_PanicsOnDuplicateID(t *testing.T) {
	require.Panics(t, func() {
		NewEncoders(&recordingEncoder{id: "dup"}, &recordingEncoder{id: "dup"})
	})
}

func TestWriter_RejectsTwoStdouts(t *testing.T) {
	encs := NewEncoders(&recordingEncoder{id: "rec-a"}, &recordingEncoder{id: "rec-b"})
	_, err := newWithStdout([]Spec{{Name: "rec-a"}, {Name: "rec-b"}}, encs, io.Discard)
	require.Error(t, err)
}

func TestWriter_RejectsUnknownEncoder(t *testing.T) {
	encs := NewEncoders(&recordingEncoder{id: "rec-a"})
	_, err := newWithStdout([]Spec{{Name: "rec-a"}, {Name: "rec-missing"}}, encs, io.Discard)
	require.Error(t, err)
}

// stdoutOnlyEncoder is a recordingEncoder that declares StdoutOnly() true,
// so the writer should reject any spec that gives it a file path.
type stdoutOnlyEncoder struct{ recordingEncoder }

func (s *stdoutOnlyEncoder) StdoutOnly() bool { return true }

func TestWriter_RejectsStdoutOnlyEncoderToFile(t *testing.T) {
	encs := NewEncoders(&stdoutOnlyEncoder{recordingEncoder{id: "rec-tty"}})

	dir := t.TempDir()
	_, err := newWithStdout([]Spec{{Name: "rec-tty", Path: filepath.Join(dir, "out.txt")}}, encs, io.Discard)
	require.Error(t, err)
	require.Contains(t, err.Error(), "stdout")
}

func TestWriter_AllowsStdoutOnlyEncoderToStdout(t *testing.T) {
	encs := NewEncoders(&stdoutOnlyEncoder{recordingEncoder{id: "rec-tty"}})
	w, err := newWithStdout([]Spec{{Name: "rec-tty"}}, encs, io.Discard)
	require.NoError(t, err)
	require.NoError(t, w.Close())
}
