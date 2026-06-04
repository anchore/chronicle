package report

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/require"

	"github.com/anchore/chronicle/chronicle/dependency"
)

// TestRun_PackagesOnly is an end-to-end test of the dependency-diff feature in
// packages-only mode (no vulnerability annotation, so no DB/network). It builds
// a real git repo with a go.mod, bumps a dependency version across two commits,
// then asserts Run reports the bump as an Updated change. This exercises the
// full materialize → syft catalog → Compare pipeline with real syft.
func TestRun_PackagesOnly(t *testing.T) {
	const goModBase = `module example.com/testrepo

go 1.21

require github.com/google/uuid v1.3.0
`
	const goModBumped = `module example.com/testrepo

go 1.21

require github.com/google/uuid v1.6.0
`

	repoDir := t.TempDir()
	sinceSha, _ := buildGoModRepo(t, repoDir, goModBase, goModBumped)

	diff, err := Run(context.Background(), repoDir, sinceSha, "HEAD", Config{
		AnnotateVulnerabilities: false,
	}, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, diff)

	// find the change for the bumped dependency; assert it is an Updated change
	// with the expected from/to versions. We assert on the specific package
	// rather than the total count, since syft may also catalog the main module.
	var got *dependency.PackageChange
	changes := diff.Changes()
	for i := range changes {
		if changes[i].Name == "github.com/google/uuid" {
			got = &changes[i]
			break
		}
	}
	require.NotNil(t, got, "expected a change for github.com/google/uuid; got changes: %+v", changes)

	require.Equal(t, dependency.Updated, got.Kind)
	require.Equal(t, "v1.3.0", got.FromVersion)
	require.Equal(t, "v1.6.0", got.ToVersion)
}

// buildGoModRepo initializes a git repo at dir with two commits: the first
// writes baseGoMod, the second writes bumpedGoMod. It returns the first and
// second commit SHAs.
func buildGoModRepo(t *testing.T, dir, baseGoMod, bumpedGoMod string) (firstSha, secondSha string) {
	t.Helper()

	repo, err := git.PlainInit(dir, false)
	require.NoError(t, err)

	wt, err := repo.Worktree()
	require.NoError(t, err)

	commit := func(content string) string {
		require.NoError(t, os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0o600))
		_, err := wt.Add("go.mod")
		require.NoError(t, err)
		h, err := wt.Commit("update go.mod", &git.CommitOptions{
			Author: &object.Signature{Name: "test", Email: "test@example.com"},
		})
		require.NoError(t, err)
		return h.String()
	}

	firstSha = commit(baseGoMod)
	secondSha = commit(bumpedGoMod)
	return firstSha, secondSha
}
