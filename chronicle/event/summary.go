package event

import "github.com/anchore/chronicle/chronicle/release/change"

// Summary is the raw payload of a CLISummaryType event: the figures the
// post-teardown recap block is built from. It carries data only — no styling,
// glyphs, or layout. The UI renders it (alongside the range/evidence groups it
// already received) into the recap block.
//
// It deliberately avoids referencing release.Description: the release package
// imports event, so pulling it in here would form an import cycle. The worker
// flattens the change set into Changes before publishing.
type Summary struct {
	// Repo is the OWNER/REPO identity of the scanned project, or "" if unknown.
	Repo string

	// Changes is the per-change-type tally that drives the "Changes" section,
	// in the user's configured order.
	Changes []SummaryChange

	// PreviousVersion / NextVersion / BumpKind drive the version-transition
	// line. NextVersion == "" means speculation was off and the line is omitted.
	PreviousVersion string
	NextVersion     string
	BumpKind        change.SemVerKind
}

// SummaryChange is one change type's contribution to the recap: its name, its
// semver tier, and how many changes of that type were collected.
type SummaryChange struct {
	Name  string
	Kind  change.SemVerKind
	Count int
}
