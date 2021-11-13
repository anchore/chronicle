package release

import "github.com/anchore/chronicle/chronicle/release/change"

type Description struct {
	Release
	VCSReferenceURL  string
	VCSChangesURL    string
	Notice           string
	Changes          change.Changes
	SupportedChanges []change.TypeTitle
}
