package release

import "github.com/anchore/chronicle/chronicle/release/change"

// Description contains all the data and metadata about a release that is pertinent to a changelog.
type Description struct {
	Release                             // the release being described
	VCSReferenceURL  string             // the URL to find more information about this release, e.g. https://github.com/anchore/chronicle/releases/tag/v0.4.1
	VCSChangesURL    string             // the URL to find the specific source changes that makeup this release, e.g. https://github.com/anchore/chronicle/compare/v0.3.0...v0.4.1
	Notice           string             // manual note or summary that describes the changelog at a high level
	Changes          change.Changes     // all issues and PRs that makeup this release
	SupportedChanges []change.TypeTitle // the sections of the changelog and their display titles
}
