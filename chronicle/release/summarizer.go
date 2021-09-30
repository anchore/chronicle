package release

import (
	"github.com/anchore/chronicle/chronicle/release/change"
)

type Summarizer interface {
	LastRelease() (*Release, error)
	Release(ref string) (*Release, error)
	Changes(sinceRef, untilRef string) ([]change.Summary, error)
	TagURL(tag string) string
	ChangesURL(sinceRef, untilRef string) string
}
