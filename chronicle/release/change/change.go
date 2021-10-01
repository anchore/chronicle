package change

import (
	"time"
)

type Changes []Change

// Change represents the smallest unit within a release that can be summarized.
type Change struct {
	Text        string
	ChangeTypes []Type
	Timestamp   time.Time
	References  []Reference
	EntryType   string
	Entry       interface{}
}

type Reference struct {
	Text string
	URL  string
}

func (s Changes) ByChangeType(types ...Type) (result Changes) {
	for _, summary := range s {
		if ContainsAny(types, summary.ChangeTypes) {
			result = append(result, summary)
		}
	}
	return result
}
