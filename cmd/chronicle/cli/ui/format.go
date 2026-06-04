package ui

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/anchore/chronicle/chronicle/event"
)

// this file owns the figure→text formatting shared by the live TUI (slot/leaf)
// and the post-teardown recap (summary). The event layer carries raw values;
// every decision about how they appear lives here.

// formatValues joins a slot's raw value segments into display text: a sha is
// shortened, a timestamp is date-formatted, text is verbatim. Segments that
// render empty (e.g. a zero date) are dropped so they leave no stray separator.
func formatValues(vals []event.Value) string {
	parts := make([]string, 0, len(vals))
	for _, v := range vals {
		if s := formatValue(v); s != "" {
			parts = append(parts, s)
		}
	}
	return strings.Join(parts, "  ")
}

func formatValue(v event.Value) string {
	switch v.Kind {
	case event.ValueSHA:
		return shortSHA(v.Text)
	case event.ValueDate:
		if v.Time.IsZero() {
			return ""
		}
		return v.Time.Format("Jan 2 2006")
	default:
		return v.Text
	}
}

// shortSHA returns the conventional leading 7 chars of a commit sha, or the
// input if shorter.
func shortSHA(sha string) string {
	if len(sha) > 7 {
		return sha[:7]
	}
	return sha
}

// formatMetrics renders a leaf's resolved figures: a single unnamed metric is a
// bare number ("47"), a single named metric is a pluralized count ("142
// packages"), and several render as a "name=count" breakdown ("added=10
// removed=12 …").
func formatMetrics(metrics []event.Metric) string {
	switch len(metrics) {
	case 0:
		return ""
	case 1:
		m := metrics[0]
		if m.Name == "" {
			return strconv.Itoa(m.Count)
		}
		return fmt.Sprintf("%d %s", m.Count, pluralize(m.Name, m.Count))
	default:
		parts := make([]string, len(metrics))
		for i, m := range metrics {
			parts[i] = fmt.Sprintf("%s=%d", m.Name, m.Count)
		}
		return strings.Join(parts, " ")
	}
}

// droppedText renders the inner text of a leaf's "(N dropped)" trailer, or ""
// when nothing was dropped (so the row shows no trailer). Callers wrap it in
// dim parens.
func droppedText(n int) string {
	if n <= 0 {
		return ""
	}
	return fmt.Sprintf("%d dropped", n)
}

// pluralize returns the plural form of a counted noun unless n == 1. A small
// table covers the irregular nouns the leaves use; everything else takes "s".
func pluralize(noun string, n int) string {
	if n == 1 {
		return noun
	}
	switch noun {
	case "vulnerability":
		return "vulnerabilities"
	default:
		return noun + "s"
	}
}
