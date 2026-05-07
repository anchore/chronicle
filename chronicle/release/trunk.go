package release

import (
	"time"

	"github.com/anchore/chronicle/chronicle/release/change"
)

// TrunkData carries commit-anchored release information for the trunk encoder.
type TrunkData struct {
	Commits []TrunkCommit // chronological, newest-first
}

type TrunkCommit struct {
	Hash      string
	URL       string // optional; populated by source-aware summarizers (e.g., github builds /owner/repo/commit/<hash>)
	Subject   string // first line of the commit message; used as title when no PR maps to this commit
	Author    string
	Timestamp time.Time
	PR        *TrunkPR // nil when no merge PR maps to this commit
}

type TrunkPR struct {
	Number      int
	Title       string
	URL         string
	Author      string
	Labels      []string
	ChangeTypes []change.Type
	Issues      []TrunkIssue
	Filtered    bool
	Reason      string // populated when Filtered (e.g. "label:chore", "no change-type")
}

type TrunkIssue struct {
	Number      int
	Title       string
	URL         string
	Labels      []string
	ChangeTypes []change.Type
	Filtered    bool
	Reason      string
}
