package bus

import (
	"fmt"
	"strings"
	"sync"

	"github.com/charmbracelet/lipgloss"

	"github.com/anchore/chronicle/chronicle/event"
	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
)

// SummaryOpts is the input for ReportSummary. Description carries the change set
// (per-type counts) and previous release; PreviousVersion / NextVersion produce
// the version-transition line. NextVersion empty means speculation is off and
// the version line is omitted.
type SummaryOpts struct {
	Description     *release.Description
	PreviousVersion string
	NextVersion     string
	BumpKind        change.SemVerKind
}

// state cache populated by PublishGroup / PublishTree so ReportSummary can pull
// the most recent range/evidence data without each caller wiring it through.
var (
	cacheMu     sync.Mutex
	lastGroups  []*event.Group
	lastTrees   []*event.Tree
	groupByName = make(map[string]*event.Group)
	treeByName  = make(map[string]*event.Tree)
)

// identity state — the OWNER/REPO of the repo being scanned. Set by the
// worker once the remote URL parses cleanly so the live TUI's range bracket
// can render its "Project: OWNER/REPO" title.
var (
	identityMu sync.Mutex
	idRepo     string
)

// SetRepo records the OWNER/REPO for the live TUI's project title and the
// post-teardown summary header.
func SetRepo(r string) {
	identityMu.Lock()
	defer identityMu.Unlock()
	idRepo = r
}

// Repo returns the OWNER/REPO recorded by SetRepo, or "" if not set. Public
// so the live TUI can read it when constructing the range bracket's title.
func Repo() string {
	identityMu.Lock()
	defer identityMu.Unlock()
	return idRepo
}

func registerGroup(g *event.Group) {
	if g == nil {
		return
	}
	cacheMu.Lock()
	defer cacheMu.Unlock()
	lastGroups = append(lastGroups, g)
	groupByName[g.Header] = g
}

func registerTree(t *event.Tree) {
	if t == nil {
		return
	}
	cacheMu.Lock()
	defer cacheMu.Unlock()
	lastTrees = append(lastTrees, t)
	treeByName[t.Header] = t
}

// styles for the rendered summary block. Match the live TUI's palette in
// cmd/chronicle/cli/ui/style.go so the post-teardown summary picks up where
// the live TUI left off.
var (
	dimStyle      = lipgloss.NewStyle().Faint(true)
	bumpStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("220"))
	okMarkStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	failMarkStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
)

const (
	// tree-prefix glyphs for the rendered summary block. Match the live TUI's
	// bracket/tree rendering in cmd/chronicle/cli/ui.
	prefixFirst  = "┌──"
	prefixMiddle = "├──"
	prefixLast   = "└──"
)

// ReportSummary builds the multi-line final summary block (range / evidence /
// changes-by-type / version transition) and emits it via Summary so it lands
// on stderr post-teardown — keeping stdout reserved for the changelog product.
func ReportSummary(opts SummaryOpts) {
	out := buildSummary(opts)
	if out == "" {
		return
	}
	Summary(out)
}

func buildSummary(opts SummaryOpts) string {
	var sections []string

	// renderRange owns the "Project: OWNER/REPO" header now, so it's a single
	// section. Sections are blank-line separated for visual consistency.
	if rng := renderRange(); rng != "" {
		sections = append(sections, rng)
	}
	if ev := renderEvidence(); ev != "" {
		sections = append(sections, ev)
	}
	if ch := renderChanges(opts.Description); ch != "" {
		sections = append(sections, ch)
	}
	if vt := renderVersionTransition(opts); vt != "" {
		sections = append(sections, vt)
	}

	return strings.Join(sections, "\n\n")
}

// renderProjectTitle builds the "Project: OWNER/REPO" line that sits above
// the range bracket. The whole line is bold so it reads as a section title;
// returns "" if no repo has been recorded.
func renderProjectTitle() string {
	repo := Repo()
	if repo == "" {
		return ""
	}
	return boldStyle.Render("Project: " + repo)
}

func renderRange() string {
	cacheMu.Lock()
	g := groupByName["range"]
	cacheMu.Unlock()
	if g == nil {
		return ""
	}
	names := g.Names()
	if len(names) == 0 {
		return ""
	}

	// compute label width for column alignment
	labelWidth := 0
	for _, n := range names {
		if l := len(g.Slot(n).Label()); l > labelWidth {
			labelWidth = l
		}
	}

	var b strings.Builder
	// the range bracket carries the "Project: OWNER/REPO" line as its title.
	// Falls back to the raw header if no project identity was recorded (e.g.
	// in tests).
	if title := renderProjectTitle(); title != "" {
		b.WriteString(title)
		b.WriteString("\n")
	} else if header := displaySummaryHeader(g.Header); header != "" {
		b.WriteString(header)
		b.WriteString("\n")
	}
	for i, name := range names {
		s := g.Slot(name)
		prefix := prefixMiddle
		if i == len(names)-1 {
			prefix = prefixLast
		}
		mark := stateMark(s.State())
		label := padRight(s.Label(), labelWidth)
		intent := dimStyle.Render(s.Intent())
		line := fmt.Sprintf("%s %s %s   %s", dimStyle.Render(prefix), mark, label, intent)
		if vals := s.Values(); len(vals) > 0 {
			line += " " + dimStyle.Render("→") + " " + strings.Join(vals, "  ")
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

func renderEvidence() string {
	cacheMu.Lock()
	t := treeByName["evidence"]
	cacheMu.Unlock()
	if t == nil {
		return ""
	}
	names := t.Names()
	if len(names) == 0 {
		return ""
	}

	// compute name width for alignment
	nameWidth := 0
	countWidth := 0
	for _, n := range names {
		if l := len(n); l > nameWidth {
			nameWidth = l
		}
		if c := t.Leaf(n).Count(); len(c) > countWidth {
			countWidth = len(c)
		}
	}

	var b strings.Builder
	if header := displaySummaryHeader(t.Header); header != "" {
		b.WriteString(header)
		b.WriteString("\n")
	}
	for i, name := range names {
		l := t.Leaf(name)
		prefix := prefixMiddle
		if i == len(names)-1 {
			prefix = prefixLast
		}
		mark := stateMark(l.State())
		nm := padRight(name, nameWidth)
		count := padRight(l.Count(), countWidth)
		line := fmt.Sprintf("%s %s %s   %s", dimStyle.Render(prefix), mark, nm, count)
		if note := l.Note(); note != "" {
			line += "  " + dimStyle.Render("("+note+")")
		} else if err := l.Err(); err != nil {
			line += "  " + dimStyle.Render(err.Error())
		}
		b.WriteString(line)
		if i < len(names)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// tierLabel is the visible label for a semver-kind tier in the rendered
// Changes section.
type tierLabel string

const (
	tierMajor   tierLabel = "major"
	tierMinor   tierLabel = "minor"
	tierPatch   tierLabel = "patch"
	tierUnknown tierLabel = "unknown" // SemVerUnknown — change types that don't carry a clear bump intent
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

// pillEntry is the structured form of a category in a tier — kept un-rendered
// so the renderer can decide whether to emit a `[name N]` pill or just the
// bare count when the name would be redundant with the tier label.
type pillEntry struct {
	name  string
	count int
}

// renderChanges builds the Changes section as a tree of semver tiers. Major,
// minor, and patch are *always* shown so the user can scan release shape at a
// glance; unknown is shown only when non-zero, and is highlighted because it
// represents change types that didn't carry a clear semver intent and may need
// triage. Empty tiers render as a dim em-dash placeholder.
func renderChanges(desc *release.Description) string {
	if desc == nil || len(desc.SupportedChanges) == 0 {
		return ""
	}

	// bucket categories by tier, preserving SupportedChanges order within each
	// bucket so the user's config order is honored.
	buckets := map[tierLabel][]pillEntry{}
	for _, tt := range desc.SupportedChanges {
		count := len(desc.Changes.ByChangeType(tt.ChangeType))
		if count == 0 {
			continue
		}
		t := tierFor(tt.ChangeType.Kind)
		buckets[t] = append(buckets[t], pillEntry{name: tt.ChangeType.Name, count: count})
	}

	// major/minor/patch are always present in the rendered tree; unknown only
	// when it has at least one entry.
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
		prefix := prefixMiddle
		if i == len(tiers)-1 {
			prefix = prefixLast
		}
		label := tierLabelStyle(t).Render(padRight(string(t), tierWidth))
		fmt.Fprintf(&b, "%s %s   %s", dimStyle.Render(prefix), label, renderTierEntries(t, buckets[t]))
		if i < len(tiers)-1 {
			b.WriteString("\n")
		}
	}
	return b.String()
}

// renderTierEntries renders the right-hand cell of a Changes row. It collapses
// a single-pill row to just the bold count when the change-type name would
// duplicate the tier label (e.g. tier "unknown" with one "unknown" change-
// type → just "1" instead of "[unknown 1]").
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

// tierLabelStyle picks the rendering style for a tier label. major/minor/patch
// render in default-fg; the unknown ("other") tier pops in orange to flag that
// some change types weren't classified.
func tierLabelStyle(t tierLabel) lipgloss.Style {
	if t == tierUnknown {
		return lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
	}
	return lipgloss.NewStyle()
}

// renderPill formats a single change-type entry as `name=N`. The "=" is dim
// (auxiliary punctuation) so the eye is drawn to the name and bold count.
func renderPill(name string, count int) string {
	return name + dimStyle.Render("=") + boldStyle.Render(fmt.Sprintf("%d", count))
}

// boldStyle is the plain-bold style used for emphasis (repo title, pill
// counts). Defined here in addition to the live TUI's copy in style.go so
// the post-teardown summary renders consistently when imported standalone.
var boldStyle = lipgloss.NewStyle().Bold(true)

func renderVersionTransition(opts SummaryOpts) string {
	if opts.NextVersion == "" {
		return ""
	}
	prev := opts.PreviousVersion
	next := opts.NextVersion
	bumpLabel := opts.BumpKind.String()

	if prev == next || bumpLabel == "" {
		// no bump case — render fully dim
		line := fmt.Sprintf("%s → %s   (no bump)", prev, next)
		return dimStyle.Render(line)
	}

	stylized := highlightBumpedElement(prev, next, opts.BumpKind)
	return fmt.Sprintf("%s %s %s   %s",
		prev,
		dimStyle.Render("→"),
		stylized,
		dimStyle.Render("("+bumpLabel+" bump)"),
	)
}

// highlightBumpedElement returns NextVersion with the major/minor/patch element
// styled per the bump kind, leaving the rest at default fg.
func highlightBumpedElement(prev, next string, kind change.SemVerKind) string {
	prevCore, _ := splitSemverCore(prev)
	nextCore, nextSuffix := splitSemverCore(next)

	prevParts := strings.SplitN(prevCore, ".", 3)
	nextParts := strings.SplitN(nextCore, ".", 3)
	if len(nextParts) < 3 || len(prevParts) < 3 {
		// not parseable as semver — fall back to bolding the entire next version
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

// splitSemverCore separates a leading dotted-numeric core from any suffix
// (pre-release / build metadata). Example: "v1.2.3-rc.1" -> ("v1.2.3", "-rc.1").
func splitSemverCore(v string) (core, suffix string) {
	for i := 0; i < len(v); i++ {
		c := v[i]
		if c == '-' || c == '+' {
			return v[:i], v[i:]
		}
	}
	return v, ""
}

func stateMark(s event.SlotState) string {
	switch s {
	case event.SlotResolved:
		return okMarkStyle.Render("✔")
	case event.SlotFailed:
		return failMarkStyle.Render("✘")
	}
	// pending or running (the post-teardown summary should not normally see
	// running states, but render a dim dot for either)
	return dimStyle.Render("·")
}

// displaySummaryHeader maps an internal cache key (e.g. "range", "evidence")
// to the label rendered in the post-teardown summary. Mirrors the live TUI's
// displayHeader: "range" is suppressed (the project title takes its place),
// everything else is Title-cased and bolded as a section heading.
func displaySummaryHeader(name string) string {
	if name == "range" || name == "" {
		return ""
	}
	return boldStyle.Render(strings.ToUpper(name[:1]) + name[1:])
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}
