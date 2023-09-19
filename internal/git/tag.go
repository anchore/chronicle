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
	Annotated bool
}

type Range struct {
	SinceRef     string
	UntilRef     string
	IncludeStart bool
	IncludeEnd   bool
}

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

	untilHash, err := r.ResolveRevision(plumbing.Revision(cfg.UntilRef))
	if err != nil {
		return nil, fmt.Errorf("unable to find until git ref=%q: %w", cfg.UntilRef, err)
	}

	iter, err := r.Log(&git.LogOptions{From: *untilHash})
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

	return newTag(r, ref)
}

func TagsFromLocal(repoPath string) ([]Tag, error) {
	r, err := git.PlainOpen(repoPath)
	if err != nil {
		return nil, err
	}

	tagRefs, err := r.Tags()
	if err != nil {
		return nil, err
	}

	var tags []Tag
	for {
		t, err := tagRefs.Next()
		if err == io.EOF || t == nil {
			break
		} else if err != nil {
			return nil, err
		}

		tag, err := newTag(r, t)
		if err != nil {
			return nil, err
		}
		if tag == nil {
			continue
		}

		tags = append(tags, *tag)
	}
	return tags, nil
}

func newTag(r *git.Repository, t *plumbing.Reference) (*Tag, error) {
	// the plumbing reference is to the tag. For a lightweight tag, the tag object points directly to the commit
	// with the code blob. For an annotated tag, the tag object has a commit for the tag itself, but resolves to
	// the commit with the code blob. It's important to use the timestamp from the tag object when available
	// for annotated tags and to use the commit timestamp for lightweight tags.

	if !t.Name().IsTag() {
		return nil, nil
	}

	c, err := r.CommitObject(t.Hash())
	if err == nil && c != nil {
		// this is a lightweight tag... the tag hash points directly to the commit object
		return &Tag{
			Name:      t.Name().Short(),
			Timestamp: c.Committer.When,
			Commit:    c.Hash.String(),
			Annotated: false,
		}, nil
	}

	// this is an annotated tag... the tag hash points to a tag object, which points to the commit object
	// use the timestamp info from the tag object

	tagObj, err := object.GetTag(r.Storer, t.Hash())
	if err != nil {
		return nil, fmt.Errorf("unable to resolve tag for %q: %w", t.Name(), err)
	}

	if tagObj == nil {
		return nil, fmt.Errorf("unable to resolve tag for %q", t.Name())
	}

	return &Tag{
		Name: t.Name().Short(),
		// it is possible for this git lib to return timestamps parsed from the underlying data that have the timezone
		// but not the name of the timezone. This can result in odd suffixes like "-0400 -0400" instead of "-0400 EDT".
		// This causes some difficulty in testing since the user's local git config and env may result in different
		// values. Here I've normalized to the local timezone which tends to be the most common case. Downstream of
		// this function, the timestamp is converted to UTC.
		Timestamp: tagObj.Tagger.When.In(time.Local),
		Commit:    tagObj.Target.String(),
		Annotated: true,
	}, nil
}
