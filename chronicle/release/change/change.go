package change

import (
	"time"
)

var UnknownType = NewType("unknown", SemVerUnknown)
var UnknownTypes = []Type{UnknownType}

type Changes []Change

// Change represents the smallest unit within a release that can be summarized.
type Change struct {
	Text        string      // title or short summary describing the change (e.g. GitHub issue or PR title)
	ChangeTypes []Type      // the kind(s) of change(s) this specific change description represents (e.g. breaking, enhancement, patch, etc.)
	Timestamp   time.Time   // the timestamp best representing when the change was committed to the VCS baseline (e.g. GitHub PR merged).
	References  []Reference // any URLs that relate to the change
	EntryType   string      // a free-form helper string that indicates where the change came from (e.g. a "github-issue"). This can be useful for parsing the `Entry` field.
	Entry       interface{} // the original data entry from the source that represents the change. The `EntryType` field should be used to help indicate how the shape should be interpreted.
}

// Reference indicates where you can find additional information about a particular change.
type Reference struct {
	Text string
	URL  string
}

// ByChangeType returns the set of changes that match one of the given change types.
func (s Changes) ByChangeType(types ...Type) (result Changes) {
	for _, summary := range s {
		if ContainsAny(types, summary.ChangeTypes) {
			result = append(result, summary)
		}
	}
	return result
}
