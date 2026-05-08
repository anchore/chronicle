package trunk

import (
	"fmt"
	"io"
	"strings"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
)

// renderExpanded writes one row per commit plus additional indented rows for
// each linked issue. The "closes" column is replaced by per-issue rows.
func (e *Encoder) renderExpanded(w io.Writer, d release.Description) error {
	st := newStyles(e.IsTTY)

	rows := e.buildExpandedRows(d)
	wHash, wPR, wTitle := measureExpandedColumns(rows)

	issuePrefix := buildIssuePrefix(wHash, wPR)

	// header row sits on the trunk line: "│" at col 0 keeps trunk continuous.
	header := fmt.Sprintf("%s  %-*s  %-*s  %-*s  %s",
		st.trunkLine,
		wHash, "commit",
		wPR, "pr",
		wTitle, "title",
		"type / filter reason",
	)

	if err := writeTopAnchor(w, d, st); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(w, st.dim.Render(header)); err != nil {
		return err
	}

	for _, r := range rows {
		if r.isIssue {
			if err := writeExpandedIssueRow(w, st, r, issuePrefix, wTitle); err != nil {
				return err
			}
			continue
		}

		if _, err := fmt.Fprintln(w, st.dim.Render(st.trunkLine)); err != nil {
			return err
		}

		title := truncateRunes(r.title, wTitle)

		if err := writeExpandedCommitRow(w, st, r, title, wHash, wPR, wTitle); err != nil {
			return err
		}
	}

	if d.PreviousRelease != nil {
		if _, err := fmt.Fprintln(w, st.dim.Render(st.trunkLine)); err != nil {
			return err
		}
		if err := writeBottomAnchor(w, d.PreviousRelease, st); err != nil {
			return err
		}
	}

	return nil
}

// measureExpandedColumns scans commit and issue rows to find per-column widths,
// floored by the header label widths and capped by maxTitleWidth for the title.
// Widths are based on visible (rune-count) content.
func measureExpandedColumns(rows []expandedRow) (wHash, wPR, wTitle int) {
	wHash = len("commit")
	wPR = len("pr")
	wTitle = len("title")
	for _, r := range rows {
		if l := visibleLen(r.title); l > wTitle {
			wTitle = l
		}
		if r.isIssue {
			continue
		}
		if l := visibleLen(r.hash); l > wHash {
			wHash = l
		}
		if l := visibleLen(r.pr); l > wPR {
			wPR = l
		}
	}
	if wTitle > maxTitleWidth {
		wTitle = maxTitleWidth
	}
	return
}

// writeExpandedCommitRow renders a top-level commit row with categorical
// coloring on the glyph and the type column. Hash and PR# are hyperlinked
// when URLs are present. Cell styling is applied inside each hyperlink wrap
// so dim attributes survive terminals that reset SGR state at OSC 8
// boundaries.
func writeExpandedCommitRow(w io.Writer, st styles, r expandedRow, title string, wHash, wPR, wTitle int) error {
	var glyphStyle, typeStyle, restStyle = st.normal, st.normal, st.normal

	if r.filtered {
		glyphStyle = st.dim
		typeStyle = st.dim
		restStyle = st.dim
	} else {
		cat := st.styleForKind(r.kind)
		glyphStyle = cat
		typeStyle = cat
	}

	hashCell := padToVisibleWidth(st.styledLink(r.hash, r.hashURL, restStyle), visibleLen(r.hash), wHash)
	prCell := padToVisibleWidth(st.styledLink(r.pr, r.prURL, restStyle), visibleLen(r.pr), wPR)
	titleCell := padToVisibleWidth(restStyle.Render(title), visibleLen(title), wTitle)

	mid := fmt.Sprintf("  %s  %s  %s  ",
		hashCell,
		prCell,
		titleCell,
	)

	line := glyphStyle.Render(r.glyph) + mid + typeStyle.Render(r.typ)
	_, err := fmt.Fprintln(w, line)
	return err
}

// writeExpandedIssueRow renders an indented "closes #N  title  type" row
// under its parent commit. The #N is hyperlinked when the issue URL is set.
// The "closes #N" prefix and the type column take the issue's categorical
// color (mirroring how the commit glyph carries the commit's category). The
// trunk char at column 0 stays dim.
func writeExpandedIssueRow(w io.Writer, st styles, r expandedRow, issuePrefix string, wTitle int) error {
	issueTitle := truncateRunes(r.title, wTitle)

	var labelStyle, typeStyle, titleStyle = st.normal, st.normal, st.normal
	if r.filtered {
		labelStyle = st.dim
		typeStyle = st.dim
		titleStyle = st.dim
	} else {
		cat := st.styleForKind(r.kind)
		labelStyle = cat
		typeStyle = cat
	}

	issueRef := fmt.Sprintf("#%d", r.issueNum)
	linkedRef := st.styledLink(issueRef, r.issueURL, labelStyle)
	label := labelStyle.Render("closes ") + linkedRef

	// issuePrefix is "│" followed by spaces; render the trunk char dim so it
	// matches the trunk decoration on the spacer lines.
	prefix := st.dim.Render(st.trunkLine) + issuePrefix[len(st.trunkLine):]

	titleCell := padToVisibleWidth(titleStyle.Render(issueTitle), visibleLen(issueTitle), wTitle)

	line := prefix + label + "  " + titleCell + "  " + typeStyle.Render(r.typ)
	_, err := fmt.Fprintln(w, line)
	return err
}

// expandedRow is a single line in the expanded output — either a commit row or
// an issue sub-row.
type expandedRow struct {
	// commit fields (only set when isIssue=false).
	glyph    string
	hash     string
	hashURL  string
	pr       string
	prURL    string
	title    string
	typ      string
	filtered bool
	kind     change.SemVerKind
	// issue sub-row fields (only set when isIssue=true).
	isIssue  bool
	issueNum int
	issueURL string
}

// buildExpandedRows returns commit rows interleaved with issue sub-rows.
func (e *Encoder) buildExpandedRows(d release.Description) []expandedRow {
	var rows []expandedRow
	for _, c := range d.Trunk.Commits {
		filtered := c.PR == nil || c.PR.Filtered

		if filtered && !e.ShowFiltered {
			continue
		}

		rows = append(rows, buildExpandedCommitRow(c, filtered))

		if filtered || c.PR == nil {
			continue
		}

		rows = append(rows, e.buildExpandedIssueRows(c.PR.Issues)...)
	}
	return rows
}

// buildExpandedCommitRow constructs a single commit row, choosing fields based
// on whether the commit is filtered (no PR or PR.Filtered) or kept.
func buildExpandedCommitRow(c release.TrunkCommit, filtered bool) expandedRow {
	var (
		hash    = shortHash(c.Hash)
		hashURL = c.URL
		prNum   = "—"
		prURL   string
		title   = c.Subject
		typ     string
		glyph   string
		kind    change.SemVerKind
	)

	if filtered {
		glyph = "·"
		if c.PR != nil {
			prNum = fmt.Sprintf("#%d", c.PR.Number)
			prURL = c.PR.URL
			title = c.PR.Title
			typ = "filtered: " + c.PR.Reason
		} else {
			typ = "filtered: no PR"
		}
	} else {
		glyph = "●"
		prNum = fmt.Sprintf("#%d", c.PR.Number)
		prURL = c.PR.URL
		title = c.PR.Title
		typ = joinChangeTypes(c.PR.ChangeTypes)
		kind = highestKind(c.PR.ChangeTypes)
	}

	return expandedRow{
		glyph:    glyph,
		hash:     hash,
		hashURL:  hashURL,
		pr:       prNum,
		prURL:    prURL,
		title:    title,
		typ:      typ,
		filtered: filtered,
		kind:     kind,
	}
}

// buildExpandedIssueRows constructs the indented issue sub-rows for a kept PR,
// honoring ShowFiltered when issues are themselves filtered.
func (e *Encoder) buildExpandedIssueRows(issues []release.TrunkIssue) []expandedRow {
	var rows []expandedRow
	for _, iss := range issues {
		if iss.Filtered && !e.ShowFiltered {
			continue
		}

		issTyp := joinChangeTypes(iss.ChangeTypes)
		if iss.Filtered {
			issTyp = "filtered: " + iss.Reason
		}

		rows = append(rows, expandedRow{
			title:    iss.Title,
			typ:      issTyp,
			filtered: iss.Filtered,
			kind:     highestKind(iss.ChangeTypes),
			isIssue:  true,
			issueNum: iss.Number,
			issueURL: iss.URL,
		})
	}
	return rows
}

// buildIssuePrefix constructs the left-margin string that places the issue
// label directly under the title column. Layout: glyph(1) + sp(2) + hash(wHash)
// + sp(2) + pr(wPR) + sp(2) = the offset at which the title column begins.
// We use "│" as the leftmost char and spaces for the rest.
func buildIssuePrefix(wHash, wPR int) string {
	indent := 1 + 2 + wHash + 2 + wPR + 2
	return "│" + strings.Repeat(" ", indent-1)
}
