package git

import (
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

func HeadTagOrCommit(repoPath string) (string, error) {
	return headTag(repoPath, true)
}

func HeadTag(repoPath string) (string, error) {
	return headTag(repoPath, false)
}

func headTag(repoPath string, orCommit bool) (string, error) {
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
			// for lightweight tags
			tagName = t.Name().Short()
			return fmt.Errorf("found")
		}

		// a little extra work for annotated tags
		revHash, err := r.ResolveRevision(plumbing.Revision(t.Name()))
		if err != nil {
			return nil
		}

		if revHash == nil {
			return nil
		}

		if *revHash == ref.Hash() {
			tagName = t.Name().Short()
			return fmt.Errorf("found")
		}
		return nil
	})

	if tagName != "" {
		return tagName, nil
	}

	if orCommit {
		return ref.Hash().String(), nil
	}
	return "", nil
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
