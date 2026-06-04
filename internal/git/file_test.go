package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListFilesAtRef(t *testing.T) {
	// build a throwaway repo with go.mod changing between two tags, plus a nested module, so we can
	// exercise both content-at-ref reads and the recursive tree walk.
	repo := t.TempDir()
	runGit := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=tester", "GIT_AUTHOR_EMAIL=tester@example.com",
			"GIT_COMMITTER_NAME=tester", "GIT_COMMITTER_EMAIL=tester@example.com",
			// isolate from the developer's global/system git config (e.g. forced tag
			// signing or annotation), keeping the test hermetic and fast.
			"GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null",
		)
		out, err := cmd.CombinedOutput()
		require.NoErrorf(t, err, "git %s: %s", strings.Join(args, " "), out)
	}
	write := func(rel, content string) {
		t.Helper()
		full := filepath.Join(repo, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(full), 0o755))
		require.NoError(t, os.WriteFile(full, []byte(content), 0o644))
	}

	runGit("init")
	runGit("checkout", "-b", "main")

	write("go.mod", "module example.com/foo\n\ngo 1.21\n")
	write("tools/go.mod", "module example.com/foo/tools\n\ngo 1.21\n")
	runGit("add", ".")
	runGit("commit", "-m", "v1")
	runGit("tag", "v1")

	write("go.mod", "module example.com/foo\n\ngo 1.23\n")
	runGit("add", ".")
	runGit("commit", "-m", "v2")
	runGit("tag", "v2")

	goModOnly := func(p string) bool { return strings.HasSuffix(p, "go.mod") }

	t.Run("reads content at the since ref", func(t *testing.T) {
		files, err := ListFilesAtRef(repo, "v1", goModOnly)
		require.NoError(t, err)

		got := map[string]string{}
		for _, f := range files {
			got[f.Path] = string(f.Content)
		}
		want := map[string]string{
			"go.mod":       "module example.com/foo\n\ngo 1.21\n",
			"tools/go.mod": "module example.com/foo/tools\n\ngo 1.21\n",
		}
		assert.Equal(t, want, got)
	})

	t.Run("reads updated content at the until ref", func(t *testing.T) {
		files, err := ListFilesAtRef(repo, "v2", goModOnly)
		require.NoError(t, err)

		got := map[string]string{}
		for _, f := range files {
			got[f.Path] = string(f.Content)
		}
		assert.Equal(t, "module example.com/foo\n\ngo 1.23\n", got["go.mod"])
	})

	t.Run("nil matcher selects all files", func(t *testing.T) {
		files, err := ListFilesAtRef(repo, "v2", nil)
		require.NoError(t, err)
		assert.Len(t, files, 2) // go.mod + tools/go.mod
	})

	t.Run("unresolvable ref errors", func(t *testing.T) {
		_, err := ListFilesAtRef(repo, "does-not-exist", nil)
		assert.Error(t, err)
	})

	t.Run("clean worktree has no dirty paths", func(t *testing.T) {
		dirty, err := WorktreeDirtyPaths(repo)
		require.NoError(t, err)
		assert.Empty(t, dirty)
	})

	t.Run("uncommitted edit shows as dirty", func(t *testing.T) {
		write("go.mod", "module example.com/foo\n\ngo 1.24\n")
		t.Cleanup(func() { runGit("checkout", "--", "go.mod") })

		dirty, err := WorktreeDirtyPaths(repo)
		require.NoError(t, err)
		assert.Equal(t, []string{"go.mod"}, dirty)
	})
}
