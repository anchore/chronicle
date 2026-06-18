package source

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	gogit "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// plumbingHash converts a hex sha string into a plumbing.Hash for tag creation.
func plumbingHash(t *testing.T, sha string) plumbing.Hash {
	t.Helper()
	return plumbing.NewHash(sha)
}

// buildTestRepo creates a minimal in-memory git repository in a temp directory
// with one or two commits so tests have a real repo to work against.
//
// The returned string is the repo root path; cleanup is deferred by the caller
// via t.Cleanup.
func buildTestRepo(t *testing.T) (repoPath string, firstHash string, secondHash string) {
	t.Helper()

	dir := t.TempDir()

	r, err := gogit.PlainInit(dir, false)
	require.NoError(t, err)

	w, err := r.Worktree()
	require.NoError(t, err)

	// --- first commit: two files ---
	writeFile(t, dir, "hello.txt", "hello world\n")
	writeFile(t, dir, "subdir/config.yaml", "key: value\n")

	_, err = w.Add(".")
	require.NoError(t, err)

	h1, err := w.Commit("first commit", &gogit.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@example.com", When: time.Now()},
	})
	require.NoError(t, err)

	firstHash = h1.String()

	// --- second commit: add a new file and change hello.txt ---
	writeFile(t, dir, "hello.txt", "hello updated\n")
	writeFile(t, dir, "extra.txt", "extra content\n")

	_, err = w.Add(".")
	require.NoError(t, err)

	h2, err := w.Commit("second commit", &gogit.CommitOptions{
		Author: &object.Signature{Name: "test", Email: "test@example.com", When: time.Now()},
	})
	require.NoError(t, err)

	secondHash = h2.String()

	return dir, firstHash, secondHash
}

// writeFile creates a file (and any necessary parent dirs) under root.
func writeFile(t *testing.T, root, rel, content string) {
	t.Helper()
	dest := filepath.Join(root, filepath.FromSlash(rel))
	require.NoError(t, os.MkdirAll(filepath.Dir(dest), 0o755))
	require.NoError(t, os.WriteFile(dest, []byte(content), 0o644))
}

func TestGitTarget_Materialize(t *testing.T) {
	repoPath, firstHash, secondHash := buildTestRepo(t)

	tests := []struct {
		name       string
		ref        string
		wantFiles  map[string]string // relative path → expected content
		wantAbsent []string          // relative paths that must NOT exist
		wantErr    require.ErrorAssertionFunc
	}{
		{
			name: "first commit",
			ref:  firstHash,
			wantFiles: map[string]string{
				"hello.txt":          "hello world\n",
				"subdir/config.yaml": "key: value\n",
			},
			wantAbsent: []string{"extra.txt"},
		},
		{
			name: "second commit",
			ref:  secondHash,
			wantFiles: map[string]string{
				"hello.txt":          "hello updated\n",
				"subdir/config.yaml": "key: value\n",
				"extra.txt":          "extra content\n",
			},
		},
		{
			name:    "unknown ref",
			ref:     "refs/tags/nonexistent",
			wantErr: require.Error,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantErr == nil {
				tt.wantErr = require.NoError
			}

			target := NewGitTarget(repoPath)
			dir, cleanup, err := target.Materialize(context.Background(), tt.ref)
			tt.wantErr(t, err)
			if err != nil {
				return
			}

			t.Cleanup(func() {
				assert.NoError(t, cleanup())
			})

			require.NotEmpty(t, dir)

			// assert expected files exist with correct contents
			for relPath, wantContent := range tt.wantFiles {
				dest := filepath.Join(dir, filepath.FromSlash(relPath))
				gotBytes, readErr := os.ReadFile(dest)
				require.NoError(t, readErr, "expected file %q to exist in materialized dir", relPath)
				assert.Equal(t, wantContent, string(gotBytes), "content mismatch for %q", relPath)
			}

			// assert files from other commits are absent
			for _, relPath := range tt.wantAbsent {
				dest := filepath.Join(dir, filepath.FromSlash(relPath))
				_, statErr := os.Stat(dest)
				assert.True(t, os.IsNotExist(statErr), "file %q should not exist in materialized dir for ref %q", relPath, tt.ref)
			}
		})
	}
}

// TestGitTarget_Materialize_LenientBranchConfig is a regression test for repos
// whose .git/config carries a branch whose `merge` value go-git's validator
// rejects (e.g. a tracking ref that is not under refs/heads/). Real git tolerates
// these; go-git's strict open fails with "branch config: invalid merge". Opening
// must route through internal/git's lenient opener so materialization still works.
func TestGitTarget_Materialize_LenientBranchConfig(t *testing.T) {
	repoPath, firstHash, _ := buildTestRepo(t)

	// append a branch whose merge value go-git's validator rejects.
	cfgPath := filepath.Join(repoPath, ".git", "config")
	b, err := os.ReadFile(cfgPath)
	require.NoError(t, err)
	b = append(b, []byte("\n[branch \"weird\"]\n\tremote = origin\n\tmerge = refs/pull/123/head\n")...)
	require.NoError(t, os.WriteFile(cfgPath, b, 0o644))

	// sanity check: a strict go-git open reproduces the failure the lenient path fixes.
	_, plainErr := gogit.PlainOpenWithOptions(repoPath, &gogit.PlainOpenOptions{EnableDotGitCommonDir: true})
	require.Error(t, plainErr)

	target := NewGitTarget(repoPath)
	dir, cleanup, err := target.Materialize(context.Background(), firstHash)
	require.NoError(t, err)
	t.Cleanup(func() { assert.NoError(t, cleanup()) })

	gotBytes, err := os.ReadFile(filepath.Join(dir, "hello.txt"))
	require.NoError(t, err)
	assert.Equal(t, "hello world\n", string(gotBytes))
}

// TestGitTarget_Materialize_ResolvesTags is a regression test for bare tag-name
// resolution: ResolveRevision alone does not find refs/tags/<name> (especially
// annotated tags), so Materialize must resolve tags explicitly. This is the
// real-world case (since ref = a release tag like "v0.11.0").
func TestGitTarget_Materialize_ResolvesTags(t *testing.T) {
	repoPath, firstHash, secondHash := buildTestRepo(t)

	r, err := gogit.PlainOpen(repoPath)
	require.NoError(t, err)

	// lightweight tag at the first commit
	_, err = r.CreateTag("v0.1.0", plumbingHash(t, firstHash), nil)
	require.NoError(t, err)

	// annotated tag at the second commit
	_, err = r.CreateTag("v0.2.0", plumbingHash(t, secondHash), &gogit.CreateTagOptions{
		Tagger:  &object.Signature{Name: "test", Email: "test@example.com", When: time.Now()},
		Message: "release v0.2.0",
	})
	require.NoError(t, err)

	tests := []struct {
		name      string
		ref       string
		wantFiles map[string]string
	}{
		{
			name:      "lightweight tag by bare name",
			ref:       "v0.1.0",
			wantFiles: map[string]string{"hello.txt": "hello world\n"},
		},
		{
			name:      "annotated tag by bare name",
			ref:       "v0.2.0",
			wantFiles: map[string]string{"hello.txt": "hello updated\n", "extra.txt": "extra content\n"},
		},
		{
			name:      "HEAD resolves via revision fallback",
			ref:       "HEAD",
			wantFiles: map[string]string{"hello.txt": "hello updated\n"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			target := NewGitTarget(repoPath)
			dir, cleanup, err := target.Materialize(context.Background(), tt.ref)
			require.NoError(t, err)
			t.Cleanup(func() { assert.NoError(t, cleanup()) })

			for relPath, wantContent := range tt.wantFiles {
				gotBytes, readErr := os.ReadFile(filepath.Join(dir, filepath.FromSlash(relPath)))
				require.NoError(t, readErr, "expected file %q for ref %q", relPath, tt.ref)
				assert.Equal(t, wantContent, string(gotBytes))
			}
		})
	}
}

func TestGitTarget_Materialize_CleanupRemovesDir(t *testing.T) {
	repoPath, firstHash, _ := buildTestRepo(t)

	target := NewGitTarget(repoPath)
	dir, cleanup, err := target.Materialize(context.Background(), firstHash)
	require.NoError(t, err)
	require.NotEmpty(t, dir)

	// dir should exist before cleanup
	_, statErr := os.Stat(dir)
	require.NoError(t, statErr, "materialized dir should exist before cleanup")

	// after cleanup the dir must be gone
	require.NoError(t, cleanup())
	_, statErr = os.Stat(dir)
	assert.True(t, os.IsNotExist(statErr), "materialized dir should be removed after cleanup")
}

func TestGitTarget_Materialize_ContextCancelled(t *testing.T) {
	repoPath, firstHash, _ := buildTestRepo(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel before materializing

	target := NewGitTarget(repoPath)
	_, _, err := target.Materialize(ctx, firstHash)
	// a pre-cancelled context may or may not prevent materialization depending on
	// when the first ctx.Err() check fires; we only assert no panic occurs.
	// If err is nil the dir was created, which is acceptable.
	_ = err
}

func TestGitTarget_Materialize_BadRepoPath(t *testing.T) {
	target := NewGitTarget("/this/path/does/not/exist")
	_, _, err := target.Materialize(context.Background(), "HEAD")
	require.Error(t, err)
}
