package git

import (
	"fmt"
	"sort"

	gogit "github.com/go-git/go-git/v5"
)

// WorktreeDirtyPaths returns the repo-relative, slash-separated paths that differ from HEAD in the
// working tree (modified, added, deleted, renamed, or untracked). A clean tree returns nil.
func WorktreeDirtyPaths(repoPath string) ([]string, error) {
	r, err := openRepo(repoPath)
	if err != nil {
		return nil, fmt.Errorf("unable to open repo %q: %w", repoPath, err)
	}

	wt, err := r.Worktree()
	if err != nil {
		return nil, fmt.Errorf("unable to open worktree: %w", err)
	}

	status, err := wt.Status()
	if err != nil {
		return nil, fmt.Errorf("unable to compute worktree status: %w", err)
	}

	var dirty []string
	for path, st := range status {
		if st.Staging == gogit.Unmodified && st.Worktree == gogit.Unmodified {
			continue
		}
		dirty = append(dirty, path)
	}
	sort.Strings(dirty)
	return dirty, nil
}
