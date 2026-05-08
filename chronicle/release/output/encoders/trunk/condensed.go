package trunk

import (
	"fmt"
	"io"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
)

// maxTitleWidth caps auto-sized title columns so very long PR titles don't
// produce unwieldy line lengths.
const maxTitleWidth = 80

// renderCondensed writes one row per commit between the top and bottom anchors.
// Issues are collapsed into the "closes" column on the same row as their PR.
func (e *Encoder) renderCondensed(w io.Writer, d release.Description) error {
	st := newStyles(e.IsTTY)

	rows := e.buildCondensedRows(d)
	wHash, wPR, wTitle, wCloses := measureCondensedColumns(rows)

	// header row sits on top of the trunk line. Putting "│" at column 0 keeps
	// the trunk visually continuous from anchor → header → data rows.
	header := fmt.Sprintf("%s  %-*s  %-*s  %-*s  %-*s  %s",
		st.trunkLine,
		wHash, "commit",
		wPR, "pr",
		wTitle, "title",
		wCloses, "closes",
		"type / filter reason",
	)

	if err := writeTopAnchor(w, d, st); err != nil {
		return err
	}

	if _, err := fmt.Fprintln(w, st.dim.Render(header)); err != nil {
		return err
	}

	// second pass: write rows.
	for _, r := range rows {
		if _, err := fmt.Fprintln(w, st.dim.Render(st.trunkLine)); err != nil {
			return err
		}

		title := truncateRunes(r.title, wTitle)

		if err := writeCondensedRow(w, st, r, title, wHash, wPR, wTitle, wCloses); err != nil {
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

// measureCondensedColumns scans rows to find the widths needed for each column,
// floored by the header label widths and capped by maxTitleWidth for the title.
// Widths are based on visible (rune-count) content — escape codes for hyperlinks
// or styling are added later and must not affect column sizing.
func measureCondensedColumns(rows []condensedRow) (wHash, wPR, wTitle, wCloses int) {
	wHash = len("commit")
	wPR = len("pr")
	wTitle = len("title")
	wCloses = len("closes")
	for _, r := range rows {
		if l := visibleLen(r.hash); l > wHash {
			wHash = l
		}
		if l := visibleLen(r.pr); l > wPR {
			wPR = l
		}
		if l := visibleLen(r.title); l > wTitle {
			wTitle = l
		}
		if l := closeRefsVisibleWidth(r.closes); l > wCloses {
			wCloses = l
		}
	}
	if wTitle > maxTitleWidth {
		wTitle = maxTitleWidth
	}
	return
}

// writeCondensedRow renders one commit row with the glyph and type column
// colored by the row's semver category. Filtered rows render entirely dim,
// overriding the category color so the dim/normal distinction stays primary.
// Hash, PR#, and each issue ref in the closes column are wrapped with OSC 8
// hyperlinks when the destination is a TTY and a URL is available. Cell
// styling is applied INSIDE each hyperlink wrap so dim/color attributes
// survive terminals that reset SGR state at OSC 8 boundaries.
func writeCondensedRow(w io.Writer, st styles, r condensedRow, title string, wHash, wPR, wTitle, wCloses int) error {
	var glyphStyle, typeStyle, restStyle = st.normal, st.normal, st.normal

	switch {
	case r.filtered:
		glyphStyle = st.dim
		typeStyle = st.dim
		restStyle = st.dim
	default:
		cat := st.styleForKind(r.kind)
		glyphStyle = cat
		typeStyle = cat
	}

	hashCell := padToVisibleWidth(st.styledLink(r.hash, r.hashURL, restStyle), visibleLen(r.hash), wHash)
	prCell := padToVisibleWidth(st.styledLink(r.pr, r.prURL, restStyle), visibleLen(r.pr), wPR)
	closesCell := padToVisibleWidth(renderCloseRefs(st, r.closes, restStyle), closeRefsVisibleWidth(r.closes), wCloses)
	titleCell := padToVisibleWidth(restStyle.Render(title), visibleLen(title), wTitle)

	mid := fmt.Sprintf("  %s  %s  %s  %s  ",
		hashCell,
		prCell,
		titleCell,
		closesCell,
	)

	line := glyphStyle.Render(r.glyph) + mid + typeStyle.Render(r.typ)
	_, err := fmt.Fprintln(w, line)
	return err
}

// condensedRow holds the pre-formatted strings for a single condensed commit row.
type condensedRow struct {
	glyph    string
	hash     string
	hashURL  string
	pr       string
	prURL    string
	title    string
	closes   []closeRef
	typ      string
	filtered bool
	kind     change.SemVerKind
}

// buildCondensedRows returns the rows that should be rendered, already formatted,
// respecting ShowFiltered.
func (e *Encoder) buildCondensedRows(d release.Description) []condensedRow {
	var rows []condensedRow
	for _, c := range d.Trunk.Commits {
		filtered := c.PR == nil || c.PR.Filtered

		if filtered && !e.ShowFiltered {
			continue
		}

		var (
			hash    = shortHash(c.Hash)
			hashURL = c.URL
			prNum   = "—"
			prURL   string
			title   = c.Subject
			closes  []closeRef
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

			for _, iss := range c.PR.Issues {
				if !iss.Filtered || e.ShowFiltered {
					closes = append(closes, closeRef{
						text: fmt.Sprintf("#%d", iss.Number),
						url:  iss.URL,
					})
				}
			}
		}

		rows = append(rows, condensedRow{
			glyph:    glyph,
			hash:     hash,
			hashURL:  hashURL,
			pr:       prNum,
			prURL:    prURL,
			title:    title,
			closes:   closes,
			typ:      typ,
			filtered: filtered,
			kind:     kind,
		})
	}
	return rows
}
