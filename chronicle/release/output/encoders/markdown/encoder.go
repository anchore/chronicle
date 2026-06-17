package markdown

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
const ID = "md"

const headerTemplate = `{{if .Title }}# {{.Title}}

{{ end }}{{if .Changes }}{{ formatChangeSections .Changes }}

{{ end }}{{if .DependencyDiff }}{{ formatDependencies .DependencyDiff }}

{{ end }}**[(Full Changelog)]({{.VCSChangesURL}})**
`

// Encoder renders a Description as GitHub-flavored markdown.
//
// NoCollapse disables the collapsible <details> rendering of dependency
// sections. It exists for the md-pretty terminal encoder: glamour renders
// <details> inline (a terminal can't collapse anything) and squashes the blank
// lines between sections, so md-pretty asks for expanded sections instead. The
// zero value keeps collapsing on, preserving the plain-markdown behavior.
type Encoder struct {
	NoCollapse bool
}

func (e *Encoder) ID() string { return ID }

func (e *Encoder) Encode(w io.Writer, title string, d release.Description) error {
	// title supports templating against the description (e.g. `{{ .Version }}`),
	// so it must be rendered before the body template runs.
	resolvedTitle, err := renderTitle(title, d)
	if err != nil {
		return err
	}

	view := struct {
		release.Description
		Title string
	}{Description: d, Title: resolvedTitle}

	funcMap := template.FuncMap{
		"formatChangeSections": func(changes change.Changes) string {
			return formatChangeSections(d.SupportedChanges, changes, d.ConventionalCommitTypes)
		},
		"formatDependencies": func(diff *dependency.Diff) string {
			return formatDependencies(diff, d.DependencyRender, !e.NoCollapse)
		},
	}

	tmpl, err := template.New("markdown").Funcs(funcMap).Parse(headerTemplate)
	if err != nil {
		return fmt.Errorf("parsing markdown template: %w", err)
	}
	return tmpl.Execute(w, view)
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
	result := fmt.Sprintf("### %s\n\n", title)
	for _, summary := range summaries {
		result += formatSummary(summary, recognizedTypes)
	}
	return result
}

func formatSummary(summary change.Change, recognizedTypes []string) string {
	result := change.TrimConventionalCommitPrefix(strings.TrimSpace(summary.Text), recognizedTypes...)
	result = fmt.Sprintf("- %s", result)
	if endsWithPunctuation(result) {
		result = result[:len(result)-1]
	}

	return result + formatReferences(summary.References) + "\n"
}

// formatDependencies renders the ### Dependencies section from a Diff. It is
// gated by the caller (template) so it is only invoked when DependencyDiff is
// non-nil; we guard against an empty diff for safety.
func formatDependencies(diff *dependency.Diff, rc *render.Config, supportsCollapsed bool) string {
	if diff == nil || diff.Totals.Total() == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("### Dependencies\n\n")
	// the summary always reports the full per-kind totals; the enumeration below
	// honors OnlyVulnerable via VisibleChanges, so the two can differ without the
	// diff itself ever being filtered.
	sb.WriteString(render.SummaryLine(*diff) + "\n")

	// the flat vulnerabilities rollup carries two parts. The remediated/introduced
	// groups only earn their place when the per-package change lists are collapsed
	// (hidden behind <details>): then they surface vulns that would otherwise be
	// buried; when sections render expanded the inline annotations already show
	// them, so they are omitted. The remaining group has no inline home anywhere,
	// so it renders whenever opted in (ShowRemaining), collapsed or not.
	sb.WriteString(vulnerabilitySection(*diff, usesCollapse(rc, supportsCollapsed), rc.ShowsRemaining()))

	// group the visible changes by ecosystem; with a single ecosystem render flat
	// (no subsection header), otherwise emit a #### subsection per ecosystem.
	groups := render.GroupByEcosystem(rc.VisibleChanges(diff.Changes))
	multi := len(groups) > 1
	for _, g := range groups {
		if multi {
			sb.WriteString("\n#### " + g.Title + "\n")
		}
		sb.WriteString(formatEcosystemActions(g.Changes, rc, supportsCollapsed))
	}

	return strings.TrimRight(sb.String(), "\n")
}

// formatEcosystemActions renders the per-change-kind blocks for one ecosystem's
// changes, honoring each kind's display mode (hide/summary/list/collapsed).
func formatEcosystemActions(changes []dependency.PackageChange, rc *render.Config, supportsCollapsed bool) string {
	var sb strings.Builder
	for _, a := range render.ActionOrder {
		mode := rc.ResolveDisplay(a.Kind, supportsCollapsed)
		if mode == render.ModeHide {
			continue
		}

		subset := render.ChangesOfKind(changes, a.Kind)
		if len(subset) == 0 {
			continue
		}

		header := fmt.Sprintf("%s (%s)", a.Label, rc.PackageCountLabel(len(subset)))

		switch mode {
		case render.ModeSummary:
			fmt.Fprintf(&sb, "\n**%s**\n", header)
		case render.ModeCollapsed:
			fmt.Fprintf(&sb, "\n<details>\n<summary>%s</summary>\n\n", header)
			sb.WriteString(dependencyList(subset))
			sb.WriteString("</details>\n")
		default: // ModeList
			fmt.Fprintf(&sb, "\n**%s**\n\n", header)
			sb.WriteString(dependencyList(subset))
		}
	}
	return sb.String()
}

// dependencyList renders changes as a compact bullet list rather than a table
// (whose columns pad to the widest cell, wasting space on variable-width version
// strings). Each line is the package name, the version transition in backticks,
// and any vulnerability note in bold parentheses (bolded so the vuln impact
// stands out against the plain package name).
func dependencyList(changes []dependency.PackageChange) string {
	var sb strings.Builder
	for _, c := range changes {
		fmt.Fprintf(&sb, "- %s %s", c.Name, render.VersionTransitionWith(c, render.Backtick))
		if note := render.VulnNoteWith(c, vulnLink); note != "" {
			sb.WriteString(" **(" + note + ")**")
		}
		sb.WriteString("\n")
	}
	return sb.String()
}

// usesCollapse reports whether any change kind actually resolves to collapsed
// display — i.e. the format supports it and at least one section is hidden
// behind a <details>. It gates the vulnerabilities rollup, which is only useful
// when sections are collapsed (so md-pretty, slack, and all-expanded markdown
// don't get it).
func usesCollapse(rc *render.Config, supportsCollapsed bool) bool {
	if !supportsCollapsed {
		return false
	}
	for _, a := range render.ActionOrder {
		if rc.ResolveDisplay(a.Kind, supportsCollapsed) == render.ModeCollapsed {
			return true
		}
	}
	return false
}

// vulnerabilitySection renders the flat, non-collapsible vulnerabilities rollup
// shown above the per-package change lists: vuln-centric groups (with affected
// packages) sit directly under the Dependencies heading, with no nested
// subsection. includeDeltas gates the remediated/introduced groups (redundant
// with inline notes unless sections collapse); includeRemaining gates the
// carried-over group. Returns "" when nothing selected carries vulnerabilities.
func vulnerabilitySection(d dependency.Diff, includeDeltas, includeRemaining bool) string {
	var rem, intro, remaining []render.VulnListing
	if includeDeltas {
		rem = render.RemediatedVulns(d)
		intro = render.IntroducedVulns(d)
	}
	if includeRemaining {
		remaining = render.RemainingVulns(d)
	}
	if len(rem) == 0 && len(intro) == 0 && len(remaining) == 0 {
		return ""
	}
	var sb strings.Builder
	writeVulnGroup(&sb, "🟢 Remediated", rem)
	writeVulnGroup(&sb, "🔴 Introduced", intro)
	writeVulnGroup(&sb, "🟡 Remaining", remaining)
	return sb.String()
}

// writeVulnGroup writes one labeled vulnerability group as a bullet list (or
// nothing when empty). Each bullet is the linked ID, its severity in
// parentheses, and the affected packages.
func writeVulnGroup(sb *strings.Builder, label string, vulns []render.VulnListing) {
	if len(vulns) == 0 {
		return
	}
	fmt.Fprintf(sb, "\n**%s (%d)**\n\n", label, len(vulns))
	for _, v := range vulns {
		fmt.Fprintf(sb, "- %s", vulnLink(dependency.Vulnerability{ID: v.ID, DataSource: v.DataSource}))
		if v.Severity != "" {
			fmt.Fprintf(sb, " (%s)", v.Severity)
		}
		if len(v.Packages) > 0 {
			fmt.Fprintf(sb, " — %s", strings.Join(v.Packages, ", "))
		}
		sb.WriteString("\n")
	}
}

// vulnLink renders a vulnerability ID as a markdown link to its data source
// (grype's primary reference URL), falling back to the bare ID when grype
// supplied no URL. The md-pretty encoder turns these into clickable terminal
// hyperlinks via its OSC 8 substitution pass.
func vulnLink(v dependency.Vulnerability) string {
	if v.DataSource == "" {
		return v.ID
	}
	return fmt.Sprintf("[%s](%s)", v.ID, v.DataSource)
}

// formatReferences groups references by kind and renders them as space-prefixed
// bracketed groups: `[Issue ...] [PR ... @handles] [other]`. See package docs
// for the full bundling rules.
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

// renderRef renders a single reference to its markdown fragment, preserving
// the bare-text behavior for @-handles linked to a github user page (so the
// github release page can still auto-credit contributors).
func renderRef(ref change.Reference) string {
	switch {
	case ref.URL == "":
		return ref.Text
	case strings.HasPrefix(ref.Text, "@") && strings.HasPrefix(ref.URL, "https://github.com/"):
		return ref.Text
	default:
		return fmt.Sprintf("[%s](%s)", ref.Text, ref.URL)
	}
}

func endsWithPunctuation(s string) bool {
	if len(s) == 0 {
		return false
	}
	return strings.Contains("!.?", s[len(s)-1:]) //nolint:gocritic
}
