package release

import "github.com/anchore/chronicle/chronicle/release/change"

// Summarizer is an abstraction for summarizing release information from a specific source (e.g. GitBub, GitLab, local repo tags, etc).
type Summarizer interface {
	LastRelease() (*Release, error)
	Release(ref string) (*Release, error)
	Changes(sinceRef, untilRef string) ([]change.Change, error)
	ReferenceURL(tag string) string
	ChangesURL(sinceRef, untilRef string) string
}
