package event

// Metric is one named figure in a Leaf's resolved value (e.g. {"package", 142}
// or {"added", 10}). A blank Name renders as a bare number; a single named
// metric renders as a count of that thing (the UI pluralizes the noun); two or
// more render as a `name=count` breakdown. All presentation lives in the UI —
// the event only carries the raw figures.
type Metric struct {
	Name  string
	Count int
}

// Num builds an unnamed metric rendered as a bare number (e.g. an evidence
// count "47").
func Num(n int) Metric { return Metric{Count: n} }

// Count builds a named metric rendered as "<n> <name>" with the noun pluralized
// by the UI (e.g. Count("package", 142) → "142 packages").
func Count(name string, n int) Metric { return Metric{Name: name, Count: n} }
