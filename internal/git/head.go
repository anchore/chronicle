package git

import (
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

func HeadTagOrCommit(repoPath string) (string, error) {
	r, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("unable to open repo: %w", err)
	}
	ref, err := r.Head()
	if err != nil {
		return "", fmt.Errorf("unable fetch head: %w", err)
	}
	tagRefs, _ := r.Tags()
	var tagName string

	_ = tagRefs.ForEach(func(t *plumbing.Reference) error {
		if t.Hash().String() == ref.Hash().String() {
			tagName = t.Name().Short()
			return fmt.Errorf("found")
		}
		return nil
	})

	if tagName != "" {
		return tagName, nil
	}

	return ref.Hash().String(), nil
}

func HeadTag(repoPath string) (string, error) {
	r, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("unable to open repo: %w", err)
	}
	ref, err := r.Head()
	if err != nil {
		return "", fmt.Errorf("unable fetch head: %w", err)
	}
	tagRefs, _ := r.Tags()
	var tagName string

	_ = tagRefs.ForEach(func(t *plumbing.Reference) error {
		if t.Hash().String() == ref.Hash().String() {
			tagName = t.Name().Short()
			return fmt.Errorf("found")
		}
		return nil
	})

	// note: if there is no tag, then an empty value is returned
	return tagName, nil
}

func HeadCommit(repoPath string) (string, error) {
	r, err := git.PlainOpen(repoPath)
	if err != nil {
		return "", fmt.Errorf("unable to open repo: %w", err)
	}
	ref, err := r.Head()
	if err != nil {
		return "", fmt.Errorf("unable fetch head: %w", err)
	}
	return ref.Hash().String(), nil
}
