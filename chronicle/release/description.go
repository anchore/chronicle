package release

import "github.com/anchore/chronicle/chronicle/release/change"

type Description struct {
	Release
	VCSTagURL        string
	VCSChangesURL    string
	Notice           string
	Changes          change.Changes
	SupportedChanges []change.TypeTitle
}
