package event

import "time"

// ValueKind tags a slot value segment so the UI knows how to render it (a sha is
// shortened, a timestamp is date-formatted). The event carries the raw value;
// the UI model owns the formatting.
type ValueKind int

const (
	// ValueText is a literal string shown verbatim (e.g. a tag name).
	ValueText ValueKind = iota
	// ValueSHA is a full commit sha; the UI shortens it to its leading chars.
	ValueSHA
	// ValueDate is a timestamp; the UI renders it in a compact date form. A
	// zero time renders as nothing.
	ValueDate
)

// Value is one raw segment of a resolved Slot (e.g. a tag, a sha, a date). The
// worker supplies the raw data and the UI decides how it appears.
type Value struct {
	Kind ValueKind
	Text string    // for ValueText / ValueSHA
	Time time.Time // for ValueDate
}

// Text builds a literal value segment.
func Text(s string) Value { return Value{Kind: ValueText, Text: s} }

// SHA builds a commit-sha value segment (the UI shortens it).
func SHA(s string) Value { return Value{Kind: ValueSHA, Text: s} }

// Date builds a timestamp value segment (the UI formats it; zero renders empty).
func Date(t time.Time) Value { return Value{Kind: ValueDate, Time: t} }
