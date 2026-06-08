package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/anchore/chronicle/chronicle/event"
	"github.com/anchore/chronicle/chronicle/release/change"
)

// this file renders the post-teardown recap block (project / range / evidence /
// changes / version transition). It is the static counterpart to the live TUI:
// the worker publishes the same range/evidence groups and a raw event.Summary,
// and this code paints them once more on a clean stretch of terminal after the
// live area is torn down. All styling lives here — the event layer carries only
// figures.

// bumpStyle highlights the bumped semver element in the version-transition line.
var bumpStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220"))

// RenderSummary builds the multi-line recap block from the range/evidence groups
// the UI collected and the raw figures in s. Sections are blank-line separated;
// an empty result means there was nothing worth showing.
func RenderSummary(groups []*event.Group, trees []*event.Tree, s event.Summary) string {
	var sections []string

	if rng := renderRange(findGroup(groups, headerRange), s.Repo); rng != "" {
		sections = append(sections, rng)
	}
	if ev := renderEvidence(findTree(trees, "evidence")); ev != "" {
		sections = append(sections, ev)
	}
	if ch := renderChanges(s.Changes); ch != "" {
		sections = append(sections, ch)
	}
	if vt := renderVersionTransition(s); vt != "" {
		sections = append(sections, vt)
	}

	return strings.Join(sections, "\n\n")
}

func findGroup(groups []*event.Group, header string) *event.Group {
	for _, g := range groups {
		if g != nil && g.Header == header {
			return g
		}
	}
	return nil
}

func findTree(trees []*event.Tree, header string) *event.Tree {
	for _, t := range trees {
		if t != nil && t.Header == header {
			return t
		}
	}
	return nil
}

// projectTitle is the bold "Project: OWNER/REPO" line that heads the recap (and
// the live range bracket). The repo string is raw; the styling is here. Returns
// "" when no repo identity was recorded.
func projectTitle(repo string) string {
	if repo == "" {
		return ""
	}
	return boldStyle.Render("Project: " + repo)
}

func renderRange(g *event.Group, repo string) string {
	if g == nil {
		return ""
	}
	names := g.Names()
	if len(names) == 0 {
		return ""
	}

	labelWidth := 0
	for _, n := range names {
		if l := len(g.Slot(n).Label()); l > labelWidth {
			labelWidth = l
		}
	}

	var b strings.Builder
	// the range bracket carries the project title; fall back to the raw header
	// (e.g. in tests with no recorded identity).
	if title := projectTitle(repo); title != "" {
		b.WriteString(title)
		b.WriteString("\n")
	} else if header := summaryHeader(g.Header); header != "" {
		b.WriteString(header)
		b.WriteString("\n")
	}
	for i, name := range names {
		s := g.Slot(name)
		prefix := branchMid
		if i == len(names)-1 {
			prefix = branchLast
		}
		line := fmt.Sprintf("%s %s %s   %s",
			dimStyle.Render(prefix), stateMark(s.State()), padRight(s.Label(), labelWidth), dimStyle.Render(s.Intent()))
		if text := formatValues(s.Values()); text != "" {
			line += " " + dimStyle.Render(arrow) + " " + resolvedStyle.Render(text)
		} else if err := s.Err(); err != nil {
			line += " " + dimStyle.Render(err.Error())
		}
		b.WriteString(line)
		if i < len(names)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

func renderEvidence(t *event.Tree) string {
	if t == nil {
		return ""
	}
	names := t.Names()
	if len(names) == 0 {
		return ""
	}

	// alignment is computed only over flat (childless) leaves: a child-bearing
	// leaf carries a long rollup label that would inflate the count column.
	nameWidth, countWidth := 0, 0
	for _, n := range names {
		l := t.Leaf(n)
		if len(l.Children()) > 0 {
			continue
		}
		if w := len(n); w > nameWidth {
			nameWidth = w
		}
		if c := formatMetrics(l.Metrics()); len(c) > countWidth {
			countWidth = len(c)
		}
	}

	var lines []string
	if header := summaryHeader(t.Header); header != "" {
		lines = append(lines, header)
	}
	for i, name := range names {
		l := t.Leaf(name)
		last := i == len(names)-1
		prefix := branchMid
		if last {
			prefix = branchLast
		}

		children := l.Children()
		if len(children) == 0 {
			lines = append(lines, evidenceLine(prefix, name, nameWidth, countWidth, l))
			continue
		}

		// child-bearing leaf: render its rollup label unpadded, then its branches
		// indented beneath it.
		lines = append(lines, evidenceLine(prefix, name, nameWidth, 0, l))
		childWidth, childCountWidth := 0, 0
		for _, c := range children {
			if w := len(c.Name()); w > childWidth {
				childWidth = w
			}
			if cw := len(formatMetrics(c.Metrics())); cw > childCountWidth {
				childCountWidth = cw
			}
		}
		cont := "│   "
		if last {
			cont = "    "
		}
		for j, c := range children {
			cp := cont + branchMid
			if j == len(children)-1 {
				cp = cont + branchLast
			}
			lines = append(lines, evidenceLine(cp, c.Name(), childWidth, childCountWidth, c))
		}
	}
	return strings.Join(lines, "\n")
}

// evidenceLine renders one evidence row: "{prefix} {mark} {name}   {count}{ (N dropped)}?".
// countWidth of 0 leaves the count unpadded (used for the long child-bearing rollup).
func evidenceLine(prefix, name string, nameWidth, countWidth int, l *event.Leaf) string {
	count := formatMetrics(l.Metrics())
	if countWidth > 0 {
		count = padRight(count, countWidth)
	}
	// a skipped leaf carries no figures; show "skipped" rather than a misleading
	// zero.
	if l.State() == event.SlotSkipped {
		count = dimStyle.Render("skipped")
	}
	line := fmt.Sprintf("%s %s %s   %s", dimStyle.Render(prefix), stateMark(l.State()), padRight(name, nameWidth), count)
	if note := droppedText(l.Dropped()); note != "" {
		line += "  " + dimStyle.Render("("+note+")")
	} else if err := l.Err(); err != nil {
		line += "  " + dimStyle.Render(err.Error())
	}
	return line
}

// tierLabel is the visible label for a semver-kind tier in the Changes section.
type tierLabel string

const (
	tierMajor   tierLabel = "major"
	tierMinor   tierLabel = "minor"
	tierPatch   tierLabel = "patch"
	tierUnknown tierLabel = "unknown" // change types that don't carry a clear bump intent
)

func tierFor(k change.SemVerKind) tierLabel {
	switch k {
	case change.SemVerMajor:
		return tierMajor
	case change.SemVerMinor:
		return tierMinor
	case change.SemVerPatch:
		return tierPatch
	}
	return tierUnknown
}

// pillEntry is one change type's contribution to a tier, kept un-rendered so the
// renderer can collapse a redundant single entry to a bare count.
type pillEntry struct {
	name  string
	count int
}

// renderChanges builds the Changes section as a tree of semver tiers. Major,
// minor, and patch are always shown so release shape is scannable at a glance;
// unknown appears only when non-zero and is highlighted because it flags change
// types that didn't carry a clear semver intent. Empty tiers render as a dim
// em-dash.
func renderChanges(changes []event.SummaryChange) string {
	if len(changes) == 0 {
		return ""
	}

	// bucket categories by tier, preserving input order within each bucket so the
	// user's configured order is honored. Zero-count types are dropped.
	buckets := map[tierLabel][]pillEntry{}
	for _, c := range changes {
		if c.Count == 0 {
			continue
		}
		t := tierFor(c.Kind)
		buckets[t] = append(buckets[t], pillEntry{name: c.Name, count: c.Count})
	}

	tiers := []tierLabel{tierMajor, tierMinor, tierPatch}
	if _, ok := buckets[tierUnknown]; ok {
		tiers = append(tiers, tierUnknown)
	}

	tierWidth := 0
	for _, t := range tiers {
		if l := len(string(t)); l > tierWidth {
			tierWidth = l
		}
	}

	var b strings.Builder
	b.WriteString(boldStyle.Render("Changes"))
	b.WriteString("\n")
	for i, t := range tiers {
		prefix := branchMid
		if i == len(tiers)-1 {
			prefix = branchLast
		}
		label := tierLabelStyle(t).Render(padRight(string(t), tierWidth))
		fmt.Fprintf(&b, "%s %s   %s", dimStyle.Render(prefix), label, renderTierEntries(t, buckets[t]))
		if i < len(tiers)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// renderTierEntries renders the right-hand cell of a Changes row, collapsing a
// single pill whose name duplicates the tier label (e.g. tier "unknown" with one
// "unknown" type → just "1") to avoid the redundant "[unknown 1]".
func renderTierEntries(t tierLabel, entries []pillEntry) string {
	switch {
	case len(entries) == 0:
		return dimStyle.Render("—")
	case len(entries) == 1 && entries[0].name == string(t):
		return boldStyle.Render(fmt.Sprintf("%d", entries[0].count))
	default:
		pills := make([]string, len(entries))
		for i, e := range entries {
			pills[i] = renderPill(e.name, e.count)
		}
		return strings.Join(pills, " ")
	}
}

// tierLabelStyle picks the tier label style: major/minor/patch default-fg; the
// unknown tier pops in orange to flag unclassified change types.
func tierLabelStyle(t tierLabel) lipgloss.Style {
	if t == tierUnknown {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
	}
	return lipgloss.NewStyle()
}

// renderPill formats a change-type entry as `name=N` with a dim "=" so the eye
// is drawn to the name and bold count.
func renderPill(name string, count int) string {
	return name + dimStyle.Render("=") + boldStyle.Render(fmt.Sprintf("%d", count))
}

func renderVersionTransition(s event.Summary) string {
	if s.NextVersion == "" {
		return ""
	}
	prev, next := s.PreviousVersion, s.NextVersion
	bumpLabel := s.BumpKind.String()

	if prev == next || bumpLabel == "" {
		// no bump — render fully dim.
		return dimStyle.Render(fmt.Sprintf("%s → %s   (no bump)", prev, next))
	}

	return fmt.Sprintf("%s %s %s   %s",
		prev,
		dimStyle.Render("→"),
		highlightBumpedElement(prev, next, s.BumpKind),
		dimStyle.Render("("+bumpLabel+" bump)"),
	)
}

// highlightBumpedElement returns next with its major/minor/patch element styled
// per the bump kind, leaving the rest at default fg.
func highlightBumpedElement(prev, next string, kind change.SemVerKind) string {
	prevCore, _ := splitSemverCore(prev)
	nextCore, nextSuffix := splitSemverCore(next)

	prevParts := strings.SplitN(prevCore, ".", 3)
	nextParts := strings.SplitN(nextCore, ".", 3)
	if len(nextParts) < 3 || len(prevParts) < 3 {
		// not parseable as semver — bold the whole next version.
		return bumpStyle.Render(next)
	}

	idx := -1
	switch kind {
	case change.SemVerMajor:
		idx = 0
	case change.SemVerMinor:
		idx = 1
	case change.SemVerPatch:
		idx = 2
	}
	if idx < 0 {
		return bumpStyle.Render(next)
	}

	leadingV := ""
	core := nextCore
	if strings.HasPrefix(core, "v") {
		leadingV = "v"
		core = core[1:]
		nextParts = strings.SplitN(core, ".", 3)
	}

	var b strings.Builder
	b.WriteString(leadingV)
	for i, part := range nextParts {
		if i > 0 {
			b.WriteString(".")
		}
		if i == idx {
			b.WriteString(bumpStyle.Render(part))
		} else {
			b.WriteString(part)
		}
	}
	b.WriteString(nextSuffix)
	return b.String()
}

// splitSemverCore separates a leading dotted-numeric core from any pre-release /
// build suffix. Example: "v1.2.3-rc.1" -> ("v1.2.3", "-rc.1").
func splitSemverCore(v string) (core, suffix string) {
	for i := 0; i < len(v); i++ {
		if c := v[i]; c == '-' || c == '+' {
			return v[:i], v[i:]
		}
	}
	return v, ""
}

// stateMark renders the status glyph for a slot/leaf state in the recap. The
// recap should not normally see running states, but a dim dot covers them.
func stateMark(s event.SlotState) string {
	switch s {
	case event.SlotResolved:
		return okMarkStyle.Render(checkMark)
	case event.SlotFailed:
		return failStyle.Render(xMark)
	case event.SlotSkipped:
		return dimStyle.Render(skipMark)
	}
	return dimStyle.Render(dotMark)
}

// summaryHeader maps an internal cache key (e.g. "range", "evidence") to the
// recap section heading. "range" is suppressed — the project title takes its
// place; everything else is Title-cased and bolded.
func summaryHeader(name string) string {
	if name == headerRange || name == "" {
		return ""
	}
	return boldStyle.Render(strings.ToUpper(name[:1]) + name[1:])
}
