// Package render holds the presentation layer for a dependency.Diff: the
// shared, format-agnostic text the prose encoders (markdown, slack) render —
// the summary sentence, version transitions, vulnerability notes, ecosystem
// grouping — plus the display Config that drives them. It imports the core
// dependency package but is itself free of any output format, so each encoder
// wraps this text in its own markup. The core dependency package stays pure
// data + algorithms with no presentation concerns.
package render

import (
	"strings"

	"github.com/anchore/chronicle/chronicle/dependency"
)

// Mode controls how one change kind (updated/downgraded/added/removed) is
// rendered. "collapsed" is "list behind a summary"; it requires format support
// (markdown <details>) and otherwise falls through to the next mode.
type Mode string

const (
	// ModeHide omits the change kind entirely.
	ModeHide Mode = "hide"
	// ModeSummary shows only the count header (e.g. "Added (20 packages)").
	ModeSummary Mode = "summary"
	// ModeList enumerates every package as a bullet list (markdown and slack).
	ModeList Mode = "list"
	// ModeCollapsed enumerates every package inside a collapsible block. Only
	// markdown supports it; other encoders fall through to the next mode in the
	// configured list (or ModeList).
	ModeCollapsed Mode = "collapsed"
)

// Config captures the user's display preferences for a Diff. Each change kind
// maps to an ordered list of display modes; an encoder uses the first mode it
// supports (so a format that can't collapse falls back to the next entry).
type Config struct {
	Actions map[dependency.ChangeKind][]Mode

	// OnlyVulnerable enumerates only the changes that remediated or introduced a
	// vulnerability, while the summary still reports the full per-kind totals.
	// The filtering happens at render time (VisibleChanges) so the underlying
	// Diff is never mutated and its totals stay whole.
	OnlyVulnerable bool
}

// DefaultConfig returns the default display preferences: every kind is
// collapsed where supported, falling back to a full list in formats that can't
// collapse (so no kind is reduced to a bare count by default).
func DefaultConfig() Config {
	return Config{
		Actions: map[dependency.ChangeKind][]Mode{
			dependency.Updated:    {ModeCollapsed, ModeList},
			dependency.Downgraded: {ModeCollapsed, ModeList},
			dependency.Added:      {ModeCollapsed, ModeList},
			dependency.Removed:    {ModeCollapsed, ModeList},
		},
	}
}

// ModesFor returns the ordered display modes for a change kind, falling back to
// the default when the config (or per-kind entry) is absent or empty.
func (c *Config) ModesFor(kind dependency.ChangeKind) []Mode {
	if c != nil && c.Actions != nil {
		if m, ok := c.Actions[kind]; ok && len(m) > 0 {
			return m
		}
	}
	return DefaultConfig().Actions[kind]
}

// ResolveDisplay picks the effective display mode for a change kind given the
// encoder's capabilities. supportsCollapsed is the only capability that varies
// today (markdown true, slack false). When no configured mode is supported it
// degrades to ModeList so a list always shows rather than vanishing.
//
// Under OnlyVulnerable a resolved ModeSummary is upgraded to ModeList: the
// enumerated set is already filtered to just the vulnerable changes, so a bare
// count would hide exactly the packages the user asked to see (the rollup totals
// still live in SummaryLine). This is what keeps Added/Removed from rendering as
// an empty header in only-vulnerable output (e.g. md-pretty, which can't
// collapse and so falls through to summary).
func (c *Config) ResolveDisplay(kind dependency.ChangeKind, supportsCollapsed bool) Mode {
	for _, m := range c.ModesFor(kind) {
		if m == ModeCollapsed && !supportsCollapsed {
			continue // unsupported here — try the next fallback mode
		}
		if m == ModeSummary && c != nil && c.OnlyVulnerable {
			return ModeList
		}
		return m
	}
	return ModeList
}

// VisibleChanges returns the changes to enumerate for the given set, honoring
// OnlyVulnerable: when set, only changes with vulnerability impact survive;
// otherwise the input is returned unchanged. Nil-safe (nil config shows all).
func (c *Config) VisibleChanges(changes []dependency.PackageChange) []dependency.PackageChange {
	if c == nil || !c.OnlyVulnerable {
		return changes
	}
	out := make([]dependency.PackageChange, 0, len(changes))
	for _, ch := range changes {
		if ch.Vuln.HasImpact() {
			out = append(out, ch)
		}
	}
	return out
}

// ParseMode validates a single display mode string (accepting the
// "collapse"/"enumerate" aliases), returning ok=false for unknown values.
func ParseMode(s string) (Mode, bool) {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "hide":
		return ModeHide, true
	case "summary":
		return ModeSummary, true
	case "list", "enumerate":
		return ModeList, true
	case "collapsed", "collapse":
		return ModeCollapsed, true
	default:
		return "", false
	}
}

// ParseModes parses a comma-separated fallback list (e.g. "collapsed,list"),
// dropping unrecognized entries. Returns nil when nothing valid is found, so
// ModesFor falls back to the default.
func ParseModes(s string) []Mode {
	var out []Mode
	for _, part := range strings.Split(s, ",") {
		if m, ok := ParseMode(part); ok {
			out = append(out, m)
		}
	}
	return out
}
