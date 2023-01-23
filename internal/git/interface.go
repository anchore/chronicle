package git

import "fmt"

var _ Interface = (*gitter)(nil)

type Interface interface {
	HeadTagOrCommit() (string, error)
	HeadTag() (string, error)
	RemoteURL() (string, error)
	SearchForTag(tagRef string) (*Tag, error)
	TagsFromLocal() ([]Tag, error)
	CommitsBetween(Range) ([]string, error)
}

type gitter struct {
	repoPath string
}

func New(repoPath string) (Interface, error) {
	if !IsRepository(repoPath) {
		return nil, fmt.Errorf("not a git repository: %q", repoPath)
	}
	return gitter{
		repoPath: repoPath,
	}, nil
}

func (g gitter) CommitsBetween(cfg Range) ([]string, error) {
	return CommitsBetween(g.repoPath, cfg)
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
