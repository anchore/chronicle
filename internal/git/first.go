package git

import (
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing/object"
)

func FirstCommit(repoPath string) (string, error) {
	r, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("unable to open repo: %w", err)
	}

	iter, err := r.Log(&git.LogOptions{})
	if err != nil {
		return "", fmt.Errorf("unable to log commits: %w", err)
	}

	// the iterator works just like "git log", which is in reverse chronological order. That means the
	// first commit in the repo is the last item in the iterator.
	var last string
	err = iter.ForEach(func(c *object.Commit) error {
		if c != nil {
			last = c.Hash.String()
		}
		return nil
	})
	return last, err
}
