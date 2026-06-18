package release

import (
	"github.com/anchore/chronicle/chronicle/dependency"
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/chronicle/release/render"
)

// Description contains all the data and metadata about a release that is pertinent to a changelog.
type Description struct {
	Release                             // the release being described
	VCSReferenceURL  string             // the URL to find more information about this release, e.g. https://github.com/anchore/chronicle/releases/tag/v0.4.1
	VCSChangesURL    string             // the URL to find the specific source changes that makeup this release, e.g. https://github.com/anchore/chronicle/compare/v0.3.0...v0.4.1
	Notice           string             // manual note or summary that describes the changelog at a high level
	Changes          change.Changes     // all issues and PRs that makeup this release
	SupportedChanges []change.TypeTitle // the sections of the changelog and their display titles

	// ConventionalCommitTypes are the (possibly non-standard) conventional-commit
	// type prefixes recognized for this changelog, used by encoders to strip the
	// prefix from change display text consistently with how it was categorized.
	ConventionalCommitTypes []string
	PreviousRelease         *Release   // the release this changelog starts from; nil if since the beginning of history
	Speculated              bool       // true when the version was inferred by the speculator
	Trunk                   *TrunkData // optional, populated by summarizers that implement TrunkSummarizer

	// DependencyDiff is the optional source-scan SBOM diff between the since and
	// until refs. Nil when the dependencies feature is disabled. Kept separate
	// from Changes/SupportedChanges so it renders independently of the
	// label-driven change sections.
	DependencyDiff *dependency.Diff

	// DependencyRender carries the display preferences for DependencyDiff. It
	// travels alongside the data rather than inside it, so the diff stays pure
	// data and the prose encoders receive their presentation config explicitly.
	// Nil means encoders apply render.DefaultConfig(). Excluded from JSON: it is
	// presentation, not part of the serialized changelog artifact.
	DependencyRender *render.Config `json:"-"`

	// Toolchain carries declared toolchain-requirement changes (e.g. a minimum Go
	// version bump). Optional, populated by the worker when toolchain detection is
	// enabled and a requirement changed. Rendered as a rollup within the
	// Dependencies section, so it travels alongside DependencyDiff.
	Toolchain *ToolchainData `json:",omitempty"`

	// raw evidence totals (pre-filter), surfaced for the summary report so it
	// can show "N (M kept)" trailers. Populated by the worker after the
	// summarizer runs; zero when not provided.
	PRTotal     int `json:"-"`
	IssueTotal  int `json:"-"`
	CommitTotal int `json:"-"`
}

// HasDependencyContent reports whether the Dependencies section has anything to
// render: either a non-empty package diff or at least one toolchain-requirement
// change. Encoders gate the section on this so a lone toolchain bump (no package
// changes) still surfaces.
func (d Description) HasDependencyContent() bool {
	return (d.DependencyDiff != nil && d.DependencyDiff.Totals.Total() > 0) || d.Toolchain.HasUpdates()
}
