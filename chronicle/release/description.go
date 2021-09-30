package release

import (
	"github.com/anchore/chronicle/chronicle/release/change"
)

type Description struct {
	VCSTagURL     string
	VCSChangesURL string
	Changes       change.Summaries
	Notice        string
}
