package output

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFileSink_Commit(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "nested", "out.txt")

	s, err := newFileSink(target)
	require.NoError(t, err)

	_, err = s.Write([]byte("hello"))
	require.NoError(t, err)

	require.NoError(t, s.Commit())

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "hello", string(got))

	// no leftover temp files in the parent dir
	entries, err := os.ReadDir(filepath.Dir(target))
	require.NoError(t, err)
	require.Len(t, entries, 1, "expected only the final file in %q", filepath.Dir(target))
}

func TestFileSink_Abort(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")

	s, err := newFileSink(target)
	require.NoError(t, err)

	_, err = s.Write([]byte("partial"))
	require.NoError(t, err)

	require.NoError(t, s.Abort())

	// final file should not exist
	_, err = os.Stat(target)
	require.True(t, os.IsNotExist(err), "expected no final file, got err=%v", err)

	// no leftover temp files
	entries, err := os.ReadDir(dir)
	require.NoError(t, err)
	require.Empty(t, entries, "expected dir to be empty after abort")
}

func TestFileSink_MkdirAllParent(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "a", "b", "c", "out.txt")

	s, err := newFileSink(target)
	require.NoError(t, err)
	_, err = s.Write([]byte("x"))
	require.NoError(t, err)
	require.NoError(t, s.Commit())

	got, err := os.ReadFile(target)
	require.NoError(t, err)
	require.Equal(t, "x", string(got))
}

func TestFileSink_CommitPermissions(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "out.txt")

	s, err := newFileSink(target)
	require.NoError(t, err)
	_, err = s.Write([]byte("x"))
	require.NoError(t, err)
	require.NoError(t, s.Commit())

	info, err := os.Stat(target)
	require.NoError(t, err)

	if runtime.GOOS == "windows" {
		// Go's os.Chmod on Windows only manages the readonly attribute. 0o644 has
		// the owner-write bit set, so the file must NOT be readonly; os.Stat
		// reports a writable file as 0o666 (readonly would be 0o444). Also
		// verify we can actually open it for writing.
		require.Equal(t, os.FileMode(0o666), info.Mode().Perm(), "expected writable file on Windows, got %o", info.Mode().Perm())
		f, err := os.OpenFile(target, os.O_WRONLY, 0)
		require.NoError(t, err)
		require.NoError(t, f.Close())
		return
	}

	require.Equal(t, os.FileMode(filePerm), info.Mode().Perm())
}
