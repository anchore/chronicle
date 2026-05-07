// Package trunk provides a commit-anchored vertical-trunk visualization of a release.
// It renders commits newest-first between version anchors, showing PR numbers, titles,
// linked issues, and change types. Two display modes are available: condensed (one row
// per commit) and expanded (issues get their own indented rows beneath their PR).
package trunk

import (
	"errors"
	"io"

	"github.com/anchore/chronicle/chronicle/release"
)

// ID is the registered name for this encoder.
const ID = "trunk"

// Encoder renders a trunk-style commit visualization.
//
// Condensed=true collapses issues into the "closes" column of each commit row.
// Condensed=false (expanded) gives each issue its own indented row.
// ShowFiltered=true renders filtered rows in a dim style; false omits them.
// IsTTY controls whether ANSI escape codes are emitted.
type Encoder struct {
	Condensed    bool
	ShowFiltered bool
	IsTTY        bool
}

func (e *Encoder) ID() string { return ID }

// Encode writes the trunk visualization to w. The title parameter is ignored
// because version anchors embed the version and date instead.
func (e *Encoder) Encode(w io.Writer, _ string, d release.Description) error {
	if d.Trunk == nil {
		return errors.New(`the "trunk" output format requires commits in scope; enable "consider-pr-merge-commits" in your config`)
	}
	if len(d.Trunk.Commits) == 0 {
		return errors.New(`the "trunk" output format requires commits in scope; enable "consider-pr-merge-commits" in your config`)
	}
	if e.Condensed {
		return e.renderCondensed(w, d)
	}
	return e.renderExpanded(w, d)
}
