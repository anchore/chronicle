package release

import "github.com/anchore/chronicle/chronicle/dependency"

// ToolchainData carries declared toolchain-requirement changes detected between the since and
// until refs (e.g. a bump to the minimum Go version in go.mod). It is populated only when
// toolchain detection is enabled and at least one requirement changed.
type ToolchainData struct {
	Updates  []ToolchainUpdate  // each detected requirement change
	Warnings []ToolchainWarning // reconciliation issues; surfaced to operators and JSON output, never the changelog body
}

// ToolchainDirection indicates whether a requirement moved up or down between refs. It is empty
// when the two versions are not comparable (e.g. opaque version constraints).
type ToolchainDirection string

const (
	ToolchainDirectionUnknown ToolchainDirection = ""
	ToolchainUpgrade          ToolchainDirection = "upgrade"
	ToolchainDowngrade        ToolchainDirection = "downgrade"
)

// ToolchainUpdate is a single declared toolchain-requirement change sourced from one file/field.
type ToolchainUpdate struct {
	Tool      dependency.Ecosystem // ecosystem the requirement belongs to (e.g. dependency.EcosystemGo)
	Source    string               // human label for what was read, e.g. "go directive", "requires-python"
	File      string               // path relative to the repo root (disambiguates multi-module repos)
	From      string               // the declared requirement as written at the since ref, e.g. "1.21"
	To        string               // ... and at the until ref, e.g. "1.23"
	Direction ToolchainDirection   // whether the requirement was upgraded or downgraded (empty if not comparable)
}

// ToolchainWarning flags an inconsistency found while reconciling multiple sources within one
// ecosystem (e.g. two modules declaring different resulting minimums).
type ToolchainWarning struct {
	Tool    dependency.Ecosystem
	Message string
	Files   []string
}

// ToolchainDisplay is a single toolchain line ready for rendering, with same-version updates
// collapsed across files. Files is populated only when disambiguation is needed (i.e. the
// ecosystem has more than one distinct version transition).
type ToolchainDisplay struct {
	Label     string // display label for the ecosystem, e.g. "Go"
	From      string
	To        string
	Direction ToolchainDirection
	Files     []string
}

// HasUpdates reports whether there is at least one detected requirement change to render. It is
// nil-safe so callers can gate rendering without a separate nil check.
func (d *ToolchainData) HasUpdates() bool {
	return d != nil && len(d.Updates) > 0
}

// DisplayLines groups the updates for rendering: updates that share an ecosystem and the same
// from/to versions collapse onto one line. When an ecosystem has multiple distinct transitions
// (e.g. divergent modules), each line carries its contributing files for disambiguation.
func (d *ToolchainData) DisplayLines() []ToolchainDisplay {
	if d == nil {
		return nil
	}

	type group struct {
		tool      dependency.Ecosystem
		from      string
		to        string
		direction ToolchainDirection
		files     []string
	}

	var order []string
	groups := make(map[string]*group)
	toolGroupCount := make(map[dependency.Ecosystem]int)

	for _, u := range d.Updates {
		key := string(u.Tool) + "\x00" + u.From + "\x00" + u.To
		g, ok := groups[key]
		if !ok {
			g = &group{tool: u.Tool, from: u.From, to: u.To, direction: u.Direction}
			groups[key] = g
			order = append(order, key)
			toolGroupCount[u.Tool]++
		}
		if u.File != "" {
			g.files = append(g.files, u.File)
		}
	}

	var out []ToolchainDisplay
	for _, key := range order {
		g := groups[key]
		line := ToolchainDisplay{Label: g.tool.Label(), From: g.from, To: g.to, Direction: g.direction}
		// only surface files when a single ecosystem reports more than one transition,
		// otherwise the path is just noise (the common single-module case).
		if toolGroupCount[g.tool] > 1 {
			line.Files = g.files
		}
		out = append(out, line)
	}
	return out
}
