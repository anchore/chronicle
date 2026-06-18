// Package slack encodes a release description as Slack "mrkdwn" text suitable
// for the `text` field of a Slack webhook payload. It mirrors the markdown
// encoder but swaps in Slack's flavor: `<url|text>` links, `*bold*` section
// labels (Slack has no real headers in text), and `•` bullets.
package slack

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"text/template"

	"github.com/anchore/chronicle/chronicle/dependency"
	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/chronicle/release/render"
)

// ID is the registered name for this encoder.
const ID = "slack"

type Encoder struct{}

func (e *Encoder) ID() string { return ID }

func (e *Encoder) Encode(w io.Writer, title string, d release.Description) error {
	// title supports templating against the description (e.g. `{{ .Version }}`),
	// so it must be rendered before the body is assembled.
	resolvedTitle, err := renderTitle(title, d)
	if err != nil {
		return err
	}

	var out strings.Builder
	if resolvedTitle != "" {
		fmt.Fprintf(&out, "*%s*\n\n", escapeMrkdwn(resolvedTitle))
	}

	if sections := formatChangeSections(d.SupportedChanges, d.Changes, d.ConventionalCommitTypes); sections != "" {
		out.WriteString(sections)
		out.WriteString("\n\n")
	}

	if deps := formatDependencies(d.DependencyDiff, d.DependencyRender, d.Toolchain); deps != "" {
		out.WriteString(deps)
		out.WriteString("\n\n")
	}

	fmt.Fprintf(&out, "*<%s|Full Changelog>*\n", d.VCSChangesURL)

	_, err = io.WriteString(w, out.String())
	return err
}

func renderTitle(raw string, d release.Description) (string, error) {
	t, err := template.New("title").Parse(raw)
	if err != nil {
		return "", fmt.Errorf("parsing title template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, d); err != nil {
		return "", fmt.Errorf("executing title template: %w", err)
	}
	return buf.String(), nil
}

func formatChangeSections(sections []change.TypeTitle, changes change.Changes, recognizedTypes []string) string {
	var result string
	for _, section := range sections {
		summaries := changes.ByChangeType(section.ChangeType)
		if len(summaries) > 0 {
			result += formatChangeSection(section.Title, summaries, recognizedTypes) + "\n"
		}
	}
	return strings.TrimRight(result, "\n")
}

func formatChangeSection(title string, summaries []change.Change, recognizedTypes []string) string {
	result := fmt.Sprintf("*%s*\n", title)
	for _, summary := range summaries {
		result += formatSummary(summary, recognizedTypes)
	}
	return result
}

func formatSummary(summary change.Change, recognizedTypes []string) string {
	text := change.TrimConventionalCommitPrefix(strings.TrimSpace(summary.Text), recognizedTypes...)
	if endsWithPunctuation(text) {
		text = text[:len(text)-1]
	}
	// escape only after trimming punctuation, otherwise the trim could lop the
	// trailing ";" off an escaped entity (e.g. "&gt;").
	result := fmt.Sprintf("• %s", escapeMrkdwn(text))

	return result + formatReferences(summary.References) + "\n"
}

// formatReferences groups references by kind and renders them as space-prefixed
// bracketed groups: `[Issue ...] [PR ... @handles] [other]`, matching the
// markdown encoder's bundling rules but with Slack link syntax.
func formatReferences(refs []change.Reference) string {
	if len(refs) == 0 {
		return ""
	}

	var issues, prs, handles, others []string
	for _, ref := range refs {
		frag := renderRef(ref)
		switch {
		case strings.HasPrefix(ref.Text, "@"):
			handles = append(handles, frag)
		case strings.Contains(ref.URL, "/issues/"):
			issues = append(issues, frag)
		case strings.Contains(ref.URL, "/pull/"):
			prs = append(prs, frag)
		default:
			others = append(others, frag)
		}
	}

	// bundle handles into the PR group if present, else the Issue group, else standalone.
	switch {
	case len(prs) > 0:
		prs = append(prs, handles...)
		handles = nil
	case len(issues) > 0:
		issues = append(issues, handles...)
		handles = nil
	}

	var out strings.Builder
	if len(issues) > 0 {
		fmt.Fprintf(&out, " [Issue %s]", strings.Join(issues, " "))
	}
	if len(prs) > 0 {
		fmt.Fprintf(&out, " [PR %s]", strings.Join(prs, " "))
	}
	if len(handles) > 0 {
		fmt.Fprintf(&out, " [%s]", strings.Join(handles, " "))
	}
	if len(others) > 0 {
		fmt.Fprintf(&out, " [%s]", strings.Join(others, " "))
	}
	return out.String()
}

// renderRef renders a single reference to its Slack fragment. Unlike the
// markdown encoder, @-handles are rendered as backticked plain text rather
// than links: Slack only auto-credits contributors when it recognizes a real
// mention (`<@USER_ID>`), which we don't have from a github user URL, so a
// linked handle adds nothing. Backticks set the handle apart visually instead.
func renderRef(ref change.Reference) string {
	switch {
	case strings.HasPrefix(ref.Text, "@"):
		return fmt.Sprintf("`%s`", escapeMrkdwn(ref.Text))
	case ref.URL == "":
		return escapeMrkdwn(ref.Text)
	default:
		return fmt.Sprintf("<%s|%s>", ref.URL, escapeMrkdwn(ref.Text))
	}
}

// vulnLink renders a vulnerability ID as a Slack link to its data source
// (grype's primary reference URL), falling back to the escaped bare ID when
// grype supplied no URL.
func vulnLink(v dependency.Vulnerability) string {
	if v.DataSource == "" {
		return escapeMrkdwn(v.ID)
	}
	return fmt.Sprintf("<%s|%s>", v.DataSource, escapeMrkdwn(v.ID))
}

// mrkdwnEscaper escapes the only three characters Slack reserves in mrkdwn
// text. & must be listed first so its replacement isn't re-escaped; NewReplacer
// scans left-to-right without re-scanning, so this is single-pass safe.
var mrkdwnEscaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// escapeMrkdwn escapes change/title display text so characters like `<` and `&`
// aren't mistaken for Slack link or mention syntax. It must never be applied to
// the `<url|text>` link wrappers we build, only to the text inside them.
func escapeMrkdwn(s string) string {
	return mrkdwnEscaper.Replace(s)
}

// formatDependencies renders the dependency diff as a Slack mrkdwn block,
// mirroring the markdown encoder's section but with `*bold*` labels and `•`
// bullets. Returns "" when there is nothing to show.
func formatDependencies(diff *dependency.Diff, rc *render.Config, tc *release.ToolchainData) string {
	hasDiff := diff != nil && diff.Totals.Total() > 0
	hasToolchain := tc.HasUpdates()
	if !hasDiff && !hasToolchain {
		return ""
	}
	if rc == nil {
		def := render.DefaultConfig()
		rc = &def
	}

	var sb strings.Builder
	sb.WriteString("*Dependencies*\n\n")

	if hasDiff {
		// summary reports the full totals; enumeration honors OnlyVulnerable. Slack
		// never collapses, so its change lists always render expanded with inline
		// vulnerability annotations — no remediated/introduced rollup (that only
		// earns its place above collapsed markdown sections). The remaining rollup,
		// which has no inline home, still renders when opted in.
		sb.WriteString(render.SummaryLine(*diff) + "\n")

		if rc.ShowsRemaining() {
			writeVulnGroup(&sb, "🟡 Remaining", render.RemainingVulns(*diff))
		}
	}

	// the toolchains rollup is a peer of the vulnerabilities rollup, rendered as a
	// bold group under the *Dependencies* label (slack has no header levels). It
	// renders even when there is no package diff, so a lone toolchain bump surfaces.
	// The leading blank line only separates it from preceding content (summary /
	// remaining rollup); when it is first under the label there is nothing to space.
	if rollup := toolchainRollup(tc); rollup != "" {
		if hasDiff {
			sb.WriteString("\n")
		}
		sb.WriteString(rollup)
	}

	if hasDiff {
		// group the visible changes by ecosystem; flat for a single ecosystem,
		// otherwise a bold ecosystem label per group (slack has no header levels).
		// The change kinds render as a subordinate bullet list under the
		// *Dependencies* header.
		groups := render.GroupByEcosystem(rc.VisibleChanges(diff.Changes))
		multi := len(groups) > 1
		for _, g := range groups {
			if multi {
				sb.WriteString("\n*" + escapeMrkdwn(g.Title) + "*\n")
			} else {
				sb.WriteString("\n")
			}
			sb.WriteString(formatEcosystemActions(g.Changes, rc))
		}
	}

	return strings.TrimRight(sb.String(), "\n")
}

// toolchainRollup renders the "Toolchains" rollup in Slack mrkdwn: a bold-labeled
// group (mirroring the vulnerability rollup) listing each declared toolchain-
// requirement change. Reconciliation warnings are not rendered (operator/JSON-
// facing only). Returns "" when there is nothing to show.
func toolchainRollup(tc *release.ToolchainData) string {
	lines := tc.DisplayLines()
	if len(lines) == 0 {
		return ""
	}
	var sb strings.Builder
	fmt.Fprintf(&sb, "*Toolchains (%d)*\n", len(lines))
	for _, l := range lines {
		fmt.Fprintf(&sb, "• %s minimum version: %s → %s", escapeMrkdwn(l.Label), escapeMrkdwn(l.From), escapeMrkdwn(l.To))
		if l.Direction == release.ToolchainDowngrade {
			sb.WriteString(" (downgrade)")
		}
		if len(l.Files) > 0 {
			fmt.Fprintf(&sb, " (%s)", escapeMrkdwn(strings.Join(l.Files, ", ")))
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// formatEcosystemActions renders the per-change-kind bullets for one ecosystem's
// changes. Slack has no <details> or tables, so "collapsed" falls through to the
// configured fallback (list or summary). Each kind is a bullet; in list mode its
// packages render as indented sub-bullets.
func formatEcosystemActions(changes []dependency.PackageChange, rc *render.Config) string {
	var sb strings.Builder
	for _, a := range render.ActionOrder {
		mode := rc.ResolveDisplay(a.Kind, false) // slack cannot collapse
		if mode == render.ModeHide {
			continue
		}

		subset := render.ChangesOfKind(changes, a.Kind)
		if len(subset) == 0 {
			continue
		}

		// the kind is a bullet subordinate to the *Dependencies* header.
		fmt.Fprintf(&sb, "• %s (%s)\n", a.Label, rc.PackageCountLabel(len(subset)))
		if mode != render.ModeList {
			continue
		}
		for _, c := range subset {
			sb.WriteString("    " + dependencyChangeLine(c) + "\n")
		}
	}
	return sb.String()
}

// writeVulnGroup writes one labeled vulnerability rollup group as Slack bullets
// (or nothing when empty): a bold label with a count, then one bullet per vuln —
// the linked ID, its severity, and the affected packages. Mirrors the markdown
// encoder's group in Slack mrkdwn.
func writeVulnGroup(sb *strings.Builder, label string, vulns []render.VulnListing) {
	if len(vulns) == 0 {
		return
	}
	fmt.Fprintf(sb, "\n*%s (%d)*\n", label, len(vulns))
	for _, v := range vulns {
		fmt.Fprintf(sb, "• %s", vulnLink(dependency.Vulnerability{ID: v.ID, DataSource: v.DataSource}))
		if v.Severity != "" {
			fmt.Fprintf(sb, " (%s)", escapeMrkdwn(v.Severity))
		}
		if len(v.Packages) > 0 {
			fmt.Fprintf(sb, " — %s", escapeMrkdwn(strings.Join(v.Packages, ", ")))
		}
		sb.WriteString("\n")
	}
}

// dependencyChangeLine renders a single change as a Slack bullet: the package
// name, the version transition in code, and any vulnerability note in bold
// parentheses (mirroring the markdown list, in Slack mrkdwn — `*x*` is bold).
func dependencyChangeLine(c dependency.PackageChange) string {
	var sb strings.Builder
	fmt.Fprintf(&sb, "• %s %s", escapeMrkdwn(c.Name), render.VersionTransitionWith(c, render.Backtick))
	if note := render.VulnNoteWith(c, vulnLink); note != "" {
		sb.WriteString(" *(" + note + ")*")
	}
	return sb.String()
}

func endsWithPunctuation(s string) bool {
	if len(s) == 0 {
		return false
	}
	return strings.Contains("!.?", s[len(s)-1:]) //nolint:gocritic
}
