package release

import (
	"github.com/anchore/chronicle/chronicle/release/change"
)

// Summarizer is an abstraction for summarizing release information from a source (e.g. GitBub, GitLab, local repo tags, etc).
type Summarizer interface {
	// LastRelease returns the last posted release (chronologically) from a source (e.g. a GitHub Release entry via the API). If no release can be found then nil is returned (without an error).
	LastRelease() (*Release, error)

	// Release returns the specific release for the given ref (e.g. a tag or commit that has a GitHub Release entry via the API). If no release can be found then nil is returned (without an error)
	Release(ref string) (*Release, error)

	// Changes returns all changes between the two given references (e.g. tag or commits). If `untilRef` is not provided then the latest VCS change found will be used.
	Changes(sinceRef, untilRef string) ([]change.Change, error)

	// ReferenceURL is the URL to find more information about this release, e.g. https://github.com/anchore/chronicle/releases/tag/v0.4.1 .
	ReferenceURL(tag string) string

	// ChangesURL is the URL to find the specific source changes that makeup this release, e.g. https://github.com/anchore/chronicle/compare/v0.3.0...v0.4.1 .
	ChangesURL(sinceRef, untilRef string) string
}
