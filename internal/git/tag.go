package git

import (
	"fmt"
	"io"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

type Tag struct {
	Name      string
	Timestamp time.Time
	Commit    string
}

// TODO: put under test
func SearchForTag(repoPath, tagRef string) (*Tag, error) {
	r, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, err
	}

	// TODO: only supports tags, should support commits and other tree-ish things
	ref, err := r.Reference(plumbing.NewTagReferenceName(tagRef), false)
	if err != nil {
		return nil, fmt.Errorf("unable to find git ref=%q: %w", tagRef, err)
	}
	if ref == nil {
		return nil, fmt.Errorf("unable to find git ref=%q", tagRef)
	}

	commit, err := r.CommitObject(ref.Hash())
	if err != nil {
		return nil, err
	}

	return &Tag{
		Name:      tagRef,
		Timestamp: commit.Committer.When,
		Commit:    commit.String(),
	}, nil
}

func TagsFromLocal(repoPath string) ([]Tag, error) {
	r, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, err
	}

	tagrefs, err := r.Tags()
	if err != nil {
		return nil, err
	}

	var tags []Tag
	for {
		t, err := tagrefs.Next()
		if err == io.EOF || t == nil {
			break
		} else if err != nil {
			return nil, err
		}

		c, err := r.CommitObject(t.Hash())
		if err != nil {
			return nil, fmt.Errorf("unable to get tag info from commit=%q: %w", t.Hash().String(), err)
		}

		tags = append(tags, Tag{
			Name:      t.Name().Short(),
			Timestamp: c.Committer.When,
			Commit:    t.Hash().String(),
		})
	}
	return tags, nil
}
