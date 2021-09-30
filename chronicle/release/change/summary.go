package change

import "time"

type Summaries []Summary

type Summary struct {
	Text        string
	ChangeTypes []Type
	Timestamp   time.Time
	References  []Reference
}

type Reference struct {
	Text string
	URL  string
}

func (s Summaries) ByChangeType(types ...Type) (result Summaries) {
	for _, summary := range s {
		if containsAnyChangeType(types, summary.ChangeTypes) {
			result = append(result, summary)
		}
	}
	return result
}
