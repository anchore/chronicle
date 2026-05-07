package git

import (
	"fmt"
	"path/filepath"
)

var _ Interface = (*gitter)(nil)

type Interface interface {
	FirstCommit() (string, error)
	HeadTagOrCommit() (string, error)
	HeadTag() (string, error)
	RemoteURL() (string, error)
	SearchForTag(tagRef string) (*Tag, error)
	TagsFromLocal() ([]Tag, error)
	CommitsBetween(Range) ([]string, error)
	CommitsBetweenWithMeta(Range) ([]Commit, error)
}

type gitter struct {
	repoPath string
}

func New(repoPath string) (Interface, error) {
	if _, err := openRepo(repoPath); err != nil {
		abs, absErr := filepath.Abs(repoPath)
		if absErr != nil {
			abs = repoPath
		}
		return nil, fmt.Errorf("could not open git repository at %q (resolved to %q): %w", repoPath, abs, err)
	}
	return gitter{
		repoPath: repoPath,
	}, nil
}

func (g gitter) CommitsBetween(cfg Range) ([]string, error) {
	return CommitsBetween(g.repoPath, cfg)
}

func (g gitter) CommitsBetweenWithMeta(cfg Range) ([]Commit, error) {
	return CommitsBetweenWithMeta(g.repoPath, cfg)
}

func (g gitter) HeadTagOrCommit() (string, error) {
	return HeadTagOrCommit(g.repoPath)
}

func (g gitter) HeadTag() (string, error) {
	return HeadTag(g.repoPath)
}

func (g gitter) RemoteURL() (string, error) {
	return RemoteURL(g.repoPath)
}

func (g gitter) SearchForTag(tagRef string) (*Tag, error) {
	return SearchForTag(g.repoPath, tagRef)
}

func (g gitter) TagsFromLocal() ([]Tag, error) {
	return TagsFromLocal(g.repoPath)
}

func (g gitter) FirstCommit() (string, error) {
	return FirstCommit(g.repoPath)
}
