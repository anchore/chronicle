package git

import (
	"fmt"
	"io"
	"time"

	git "github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/plumbing/storer"

	"github.com/anchore/chronicle/internal/log"
)

type Tag struct {
	Name      string
	Timestamp time.Time
	Commit    string
}

type Range struct {
	SinceRef     string
	UntilRef     string
	IncludeStart bool
	IncludeEnd   bool
}

// TODO: put under test
func CommitsBetween(repoPath string, cfg Range) ([]string, error) {
	r, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, err
	}

	var sinceHash *plumbing.Hash
	if cfg.SinceRef != "" {
		sinceHash, err = r.ResolveRevision(plumbing.Revision(cfg.SinceRef))
		if err != nil {
			return nil, fmt.Errorf("unable to find since git ref=%q: %w", cfg.SinceRef, err)
		}
	}

	var sinceTime time.Time
	if sinceHash != nil {
		c, err := r.CommitObject(*sinceHash)
		if err != nil {
			return nil, fmt.Errorf("unable to find since git commit=%q: %w", sinceHash, err)
		}
		sinceTime = c.Committer.When
	}

	untilHash, err := r.ResolveRevision(plumbing.Revision(cfg.UntilRef))
	if err != nil {
		return nil, fmt.Errorf("unable to find until git ref=%q: %w", cfg.UntilRef, err)
	}

	iter, err := r.Log(&git.LogOptions{
		From:  *untilHash,
		Since: &sinceTime,
	})
	if err != nil {
		return nil, fmt.Errorf("unable to find until git log for ref=%q: %w", cfg.UntilRef, err)
	}

	log.WithFields("since", sinceHash, "until", untilHash).Trace("searching commit range")

	var commits []string
	err = iter.ForEach(func(c *object.Commit) (retErr error) {
		hash := c.Hash.String()

		switch {
		case untilHash != nil && c.Hash == *untilHash:
			if cfg.IncludeEnd {
				commits = append(commits, hash)
			}
		case sinceHash != nil && c.Hash == *sinceHash:
			retErr = storer.ErrStop
			if cfg.IncludeStart {
				commits = append(commits, hash)
			}
		default:
			commits = append(commits, hash)
		}

		return
	})

	return commits, err
}

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
		Commit:    commit.Hash.String(),
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
