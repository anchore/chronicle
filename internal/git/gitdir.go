package git

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// gitDirs holds the resolved locations of a repository's git directory and its common dir. For a
// normal repo the two are identical; for a linked worktree gitDir holds per-worktree metadata
// (HEAD, index) while commonDir holds shared state (config, refs, objects).
type gitDirs struct {
	gitDir    string
	commonDir string
}

// resolveGitDirs locates the git dir and common dir for the repo rooted at p. It handles a normal
// repo (".git" is a directory) and the worktree/submodule layout (".git" is a file containing a
// "gitdir:" pointer to the real git dir). For a linked worktree the shared state lives in a
// separate common dir recorded in a "commondir" file; submodules have no commondir, so there the
// git dir is also the common dir.
func resolveGitDirs(p string) (gitDirs, error) {
	dotGit := filepath.Join(p, ".git")
	fi, err := os.Stat(dotGit)
	if err != nil {
		return gitDirs{}, fmt.Errorf("unable to stat %q: %w", dotGit, err)
	}

	// common case: .git is a directory that serves as both the git dir and the common dir
	if fi.IsDir() {
		return gitDirs{gitDir: dotGit, commonDir: dotGit}, nil
	}

	// worktree/submodule case: .git is a file pointing at the real git dir
	gitDir, err := readGitDirPointer(dotGit)
	if err != nil {
		return gitDirs{}, err
	}

	// worktrees record the shared common dir in a "commondir" file alongside the per-worktree git
	// dir (relative paths resolve against the git dir). When absent (e.g. submodules) the git dir
	// is itself the common dir.
	commonDir := gitDir
	if data, readErr := os.ReadFile(filepath.Join(gitDir, "commondir")); readErr == nil {
		common := strings.TrimSpace(string(data))
		if !filepath.IsAbs(common) {
			common = filepath.Join(gitDir, common)
		}
		commonDir = common
	}

	return gitDirs{gitDir: gitDir, commonDir: commonDir}, nil
}

// readGitDirPointer reads a ".git" file and returns the path it points at via its "gitdir:" line,
// resolving relative pointers against the directory containing the file.
func readGitDirPointer(dotGitFile string) (string, error) {
	data, err := os.ReadFile(dotGitFile)
	if err != nil {
		return "", fmt.Errorf("unable to read git dir pointer %q: %w", dotGitFile, err)
	}

	const prefix = "gitdir:"
	for line := range strings.SplitSeq(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, prefix) {
			continue
		}
		gitDir := strings.TrimSpace(strings.TrimPrefix(line, prefix))
		if !filepath.IsAbs(gitDir) {
			gitDir = filepath.Join(filepath.Dir(dotGitFile), gitDir)
		}
		return gitDir, nil
	}

	return "", fmt.Errorf("no 'gitdir:' pointer found in %q", dotGitFile)
}
