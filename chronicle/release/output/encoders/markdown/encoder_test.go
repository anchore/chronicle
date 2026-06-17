package markdown

import (
	"bytes"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/require"

	"github.com/anchore/chronicle/chronicle/dependency"
	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/chronicle/release/render"
)

func TestMarkdownPresenter_Present(t *testing.T) {
	assertEncoderAgainstGoldenSnapshot(t,
		`{{ .Version }}`,
		release.Description{
			SupportedChanges: []change.TypeTitle{
				{ChangeType: change.NewType("bug", change.SemVerPatch), Title: "Bug Fixes"},
				{ChangeType: change.NewType("added", change.SemVerMinor), Title: "Added Features"},
				{ChangeType: change.NewType("breaking", change.SemVerMajor), Title: "Breaking Changes"},
				{ChangeType: change.NewType("removed", change.SemVerMajor), Title: "Removed Features"},
			},
			Release: release.Release{
				Version: "v0.19.1",
				Date:    time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			},
			VCSReferenceURL: "https://github.com/anchore/syft/tree/v0.19.1",
			VCSChangesURL:   "https://github.com/anchore/syft/compare/v0.19.0...v0.19.1",
			Changes: []change.Change{
				{
					ChangeTypes: []change.Type{change.NewType("bug", change.SemVerPatch)},
					Text:        "fix: Redirect cursor hide/show to stderr.",
					References:  []change.Reference{{Text: "#456", URL: "https://github.com/anchore/syft/pull/456"}},
				},
				{
					ChangeTypes: []change.Type{change.NewType("added", change.SemVerMinor)},
					Text:        "added feature!",
					References: []change.Reference{
						{Text: "#457", URL: "https://github.com/anchore/syft/pull/457"},
						{Text: "@wagoodman", URL: "https://github.com/wagoodman"},
					},
				},
				{
					ChangeTypes: []change.Type{change.NewType("added", change.SemVerMinor)},
					Text:        "feat(api)!: another added feature",
				},
				{
					ChangeTypes: []change.Type{change.NewType("breaking", change.SemVerMajor)},
					Text:        "breaking change?",
					References: []change.Reference{
						{Text: "#458", URL: "https://github.com/anchore/syft/pull/458"},
						{Text: "#450", URL: "https://github.com/anchore/syft/issues/450"},
						{Text: "@wagoodman", URL: "https://github.com/wagoodman"},
					},
				},
			},
			Notice: "notice!",
		},
	)
}

func TestMarkdownPresenter_Present_NoTitle(t *testing.T) {
	assertEncoderAgainstGoldenSnapshot(t,
		"",
		release.Description{
			SupportedChanges: []change.TypeTitle{
				{ChangeType: change.NewType("bug", change.SemVerPatch), Title: "Bug Fixes"},
			},
			Release: release.Release{
				Version: "v0.19.1",
				Date:    time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			},
			VCSReferenceURL: "https://github.com/anchore/syft/tree/v0.19.1",
			VCSChangesURL:   "https://github.com/anchore/syft/compare/v0.19.0...v0.19.1",
			Changes: []change.Change{
				{
					ChangeTypes: []change.Type{change.NewType("bug", change.SemVerPatch)},
					Text:        "Redirect cursor hide/show to stderr",
					References:  []change.Reference{{Text: "#456", URL: "https://github.com/anchore/syft/pull/456"}},
				},
			},
			Notice: "notice!",
		},
	)
}

func TestMarkdownPresenter_Present_NoChanges(t *testing.T) {
	assertEncoderAgainstGoldenSnapshot(t,
		"Changelog",
		release.Description{
			SupportedChanges: []change.TypeTitle{},
			Release: release.Release{
				Version: "v0.19.1",
				Date:    time.Date(2021, time.September, 16, 19, 34, 0, 0, time.UTC),
			},
			VCSReferenceURL: "https://github.com/anchore/syft/tree/v0.19.1",
			VCSChangesURL:   "https://github.com/anchore/syft/compare/v0.19.0...v0.19.1",
			Changes:         []change.Change{},
			Notice:          "notice!",
		},
	)
}

func TestMarkdownPresenter_Present_DependencyDiff(t *testing.T) {
	// exercises Updated (with remediated), Downgraded (with reintroduction),
	// Added (no annotation), and Removed (with remediated) change kinds. The
	// rollup counts derive from the per-change Vuln deltas (3 unique remediated,
	// 1 introduced).
	diff := dependency.NewDiff([]dependency.PackageChange{
		{
			Name:        "golang.org/x/net",
			Type:        "go-module",
			FromVersion: "v0.17.0",
			ToVersion:   "v0.23.0",
			Kind:        dependency.Updated,
			Vuln: &dependency.VulnDelta{
				Remediated: []dependency.Vulnerability{
					{ID: "CVE-2023-44487", Severity: "high", DataSource: "https://nvd.nist.gov/vuln/detail/CVE-2023-44487"},
					// no DataSource: exercises the bare-ID fallback (unlinked).
					{ID: "CVE-2023-39325", Severity: "high"},
				},
			},
		},
		{
			Name:        "github.com/foo/bar",
			Type:        "go-module",
			FromVersion: "v1.2.0",
			ToVersion:   "v1.1.0",
			Kind:        dependency.Downgraded,
			Vuln: &dependency.VulnDelta{
				Introduced: []dependency.Vulnerability{
					{ID: "CVE-2024-12345", Severity: "critical", DataSource: "https://nvd.nist.gov/vuln/detail/CVE-2024-12345"},
				},
			},
		},
		{
			Name:      "github.com/new/dep",
			Type:      "go-module",
			ToVersion: "v0.4.0",
			Kind:      dependency.Added,
		},
		{
			Name:        "github.com/old/dep",
			Type:        "go-module",
			FromVersion: "v0.9.0",
			Kind:        dependency.Removed,
			Vuln: &dependency.VulnDelta{
				Remediated: []dependency.Vulnerability{
					{ID: "CVE-2022-0001", Severity: "medium", DataSource: "https://nvd.nist.gov/vuln/detail/CVE-2022-0001"},
				},
			},
		},
	})

	assertEncoderAgainstGoldenSnapshot(t,
		`{{ .Version }}`,
		release.Description{
			SupportedChanges: []change.TypeTitle{
				{ChangeType: change.NewType("bug", change.SemVerPatch), Title: "Bug Fixes"},
			},
			Release: release.Release{
				Version: "v0.20.0",
				Date:    time.Date(2024, time.January, 15, 12, 0, 0, 0, time.UTC),
			},
			VCSChangesURL: "https://github.com/anchore/syft/compare/v0.19.0...v0.20.0",
			Changes: []change.Change{
				{
					ChangeTypes: []change.Type{change.NewType("bug", change.SemVerPatch)},
					Text:        "fix: some bug fix",
				},
			},
			DependencyDiff: &diff,
		},
	)
}

func TestMarkdownPresenter_Present_DependencyDiff_MultiEcosystem(t *testing.T) {
	// multiple ecosystems render as #### subsections; here every action is set
	// to list so the tables are visible under each ecosystem.
	allList := render.Config{
		Actions: map[dependency.ChangeKind][]render.Mode{
			dependency.Updated:    {render.ModeList},
			dependency.Downgraded: {render.ModeList},
			dependency.Added:      {render.ModeList},
			dependency.Removed:    {render.ModeList},
		},
	}
	diff := dependency.NewDiff([]dependency.PackageChange{
		{Name: "golang.org/x/net", Type: "go-module", FromVersion: "v0.17.0", ToVersion: "v0.23.0", Kind: dependency.Updated},
		{Name: "github.com/new/dep", Type: "go-module", ToVersion: "v0.4.0", Kind: dependency.Added},
		{Name: "left-pad", Type: "npm", FromVersion: "1.2.0", ToVersion: "1.3.0", Kind: dependency.Updated},
		{Name: "requests", Type: "python", FromVersion: "2.31.0", ToVersion: "2.30.0", Kind: dependency.Downgraded},
	})
	assertEncoderAgainstGoldenSnapshot(t,
		`{{ .Version }}`,
		release.Description{
			Release:          release.Release{Version: "v0.20.0"},
			VCSChangesURL:    "https://example.com/compare",
			DependencyDiff:   &diff,
			DependencyRender: &allList,
		},
	)
}

func TestMarkdownPresenter_Present_DependencyDiff_CollapsedWithVulns(t *testing.T) {
	// collapsed wraps the table in <details>; the vulnerability note appears
	// because the diff carries vulnerability data (2 unique remediated).
	cfg := render.Config{
		Actions: map[dependency.ChangeKind][]render.Mode{
			dependency.Updated: {render.ModeCollapsed},
			dependency.Added:   {render.ModeSummary},
		},
	}
	diff := dependency.NewDiff([]dependency.PackageChange{
		{
			Name: "golang.org/x/net", Type: "go-module", FromVersion: "v0.17.0", ToVersion: "v0.23.0", Kind: dependency.Updated,
			Vuln: &dependency.VulnDelta{Remediated: []dependency.Vulnerability{{ID: "CVE-2023-44487", DataSource: "https://nvd.nist.gov/vuln/detail/CVE-2023-44487"}, {ID: "CVE-2023-39325", DataSource: "https://nvd.nist.gov/vuln/detail/CVE-2023-39325"}}},
		},
		{Name: "github.com/new/dep", Type: "go-module", ToVersion: "v0.4.0", Kind: dependency.Added},
	})
	assertEncoderAgainstGoldenSnapshot(t,
		`{{ .Version }}`,
		release.Description{
			Release:          release.Release{Version: "v0.20.0"},
			VCSChangesURL:    "https://example.com/compare",
			DependencyDiff:   &diff,
			DependencyRender: &cfg,
		},
	)
}

func TestMarkdownPresenter_Present_OnlyVulnerable_NoCollapse(t *testing.T) {
	// mirrors the md-pretty path: NoCollapse (can't collapse) + OnlyVulnerable,
	// with Added/Removed explicitly set to collapsed,summary. With collapse
	// unavailable they fall through to a bare summary header and would render
	// empty; OnlyVulnerable must upgrade that to a list so the vulnerable
	// added/removed packages show.
	cfg := render.Config{
		OnlyVulnerable: true,
		Actions: map[dependency.ChangeKind][]render.Mode{
			dependency.Added:   {render.ModeCollapsed, render.ModeSummary},
			dependency.Removed: {render.ModeCollapsed, render.ModeSummary},
		},
	}
	diff := dependency.NewDiff([]dependency.PackageChange{
		{
			Name: "golang.org/x/net", Type: "go-module", FromVersion: "v0.17.0", ToVersion: "v0.23.0", Kind: dependency.Updated,
			Vuln: &dependency.VulnDelta{Remediated: []dependency.Vulnerability{{ID: "CVE-2023-44487", DataSource: "https://nvd.nist.gov/vuln/detail/CVE-2023-44487"}}},
		},
		// a non-vulnerable update: must be filtered out entirely by OnlyVulnerable.
		{Name: "github.com/quiet/dep", Type: "go-module", FromVersion: "v1.0.0", ToVersion: "v1.1.0", Kind: dependency.Updated},
		{
			Name: "github.com/new/dep", Type: "go-module", ToVersion: "v0.4.0", Kind: dependency.Added,
			Vuln: &dependency.VulnDelta{Introduced: []dependency.Vulnerability{{ID: "CVE-2024-99999", DataSource: "https://nvd.nist.gov/vuln/detail/CVE-2024-99999"}}},
		},
		{
			Name: "github.com/old/dep", Type: "go-module", FromVersion: "v0.9.0", Kind: dependency.Removed,
			Vuln: &dependency.VulnDelta{Remediated: []dependency.Vulnerability{{ID: "CVE-2022-0001", DataSource: "https://nvd.nist.gov/vuln/detail/CVE-2022-0001"}}},
		},
	})

	var buf bytes.Buffer
	require.NoError(t, (&Encoder{NoCollapse: true}).Encode(&buf, `{{ .Version }}`, release.Description{
		Release:          release.Release{Version: "v0.20.0"},
		VCSChangesURL:    "https://example.com/compare",
		DependencyDiff:   &diff,
		DependencyRender: &cfg,
	}))
	snaps.MatchSnapshot(t, buf.String())
}

func TestMarkdownPresenter_Present_DependencyDiff_ShowRemaining(t *testing.T) {
	// ShowRemaining surfaces the carried-over vulnerabilities (Diff.Remaining)
	// as a "🟡 Remaining" rollup group. Here updated is collapsed (so the
	// remediated rollup also shows), and the remaining set spans both a changed
	// package and an unchanged one — CVE-2021-1111 appears on two packages and
	// must collapse to one listing naming both.
	cfg := render.Config{
		ShowRemaining: true,
		Actions: map[dependency.ChangeKind][]render.Mode{
			dependency.Updated: {render.ModeCollapsed},
		},
	}
	diff := dependency.NewDiff([]dependency.PackageChange{
		{
			Name: "golang.org/x/net", Type: "go-module", FromVersion: "v0.17.0", ToVersion: "v0.23.0", Kind: dependency.Updated,
			Vuln: &dependency.VulnDelta{Remediated: []dependency.Vulnerability{{ID: "CVE-2023-44487", Severity: "high", DataSource: "https://nvd.nist.gov/vuln/detail/CVE-2023-44487"}}},
		},
	})
	diff.Remaining = []dependency.PackageVulns{
		{
			Package: dependency.Package{Name: "github.com/legacy/lib", Type: "go-module"},
			Vulns: []dependency.Vulnerability{
				{ID: "CVE-2021-1111", Severity: "critical", DataSource: "https://nvd.nist.gov/vuln/detail/CVE-2021-1111"},
				// no DataSource: exercises the bare-ID fallback (unlinked).
				{ID: "CVE-2021-2222", Severity: "medium"},
			},
		},
		{
			Package: dependency.Package{Name: "golang.org/x/net", Type: "go-module"},
			Vulns: []dependency.Vulnerability{
				{ID: "CVE-2021-1111", Severity: "critical", DataSource: "https://nvd.nist.gov/vuln/detail/CVE-2021-1111"},
			},
		},
	}
	diff.RemainingCount = 2

	assertEncoderAgainstGoldenSnapshot(t,
		`{{ .Version }}`,
		release.Description{
			Release:          release.Release{Version: "v0.20.0"},
			VCSChangesURL:    "https://example.com/compare",
			DependencyDiff:   &diff,
			DependencyRender: &cfg,
		},
	)
}

func assertEncoderAgainstGoldenSnapshot(t *testing.T, title string, d release.Description) {
	t.Helper()
	var buf bytes.Buffer
	require.NoError(t, (&Encoder{}).Encode(&buf, title, d))
	snaps.MatchSnapshot(t, buf.String())
}

func Test_formatReferences(t *testing.T) {
	pr1 := change.Reference{Text: "#1", URL: "https://github.com/o/r/pull/1"}
	pr2 := change.Reference{Text: "#2", URL: "https://github.com/o/r/pull/2"}
	iss1 := change.Reference{Text: "#10", URL: "https://github.com/o/r/issues/10"}
	iss2 := change.Reference{Text: "#11", URL: "https://github.com/o/r/issues/11"}
	handleGH := change.Reference{Text: "@alice", URL: "https://github.com/alice"}
	handleOther := change.Reference{Text: "@bob", URL: "https://example.com/bob"}
	noURL := change.Reference{Text: "CVE-2024-0001", URL: ""}
	weird := change.Reference{Text: "release-notes", URL: "https://example.com/notes"}

	tests := []struct {
		name string
		refs []change.Reference
		want string
	}{
		{
			name: "empty refs",
			refs: nil,
			want: "",
		},
		{
			name: "single PR",
			refs: []change.Reference{pr1},
			want: " [PR [#1](https://github.com/o/r/pull/1)]",
		},
		{
			name: "single issue",
			refs: []change.Reference{iss1},
			want: " [Issue [#10](https://github.com/o/r/issues/10)]",
		},
		{
			name: "handle alone (no issue, no PR) renders standalone",
			refs: []change.Reference{handleGH},
			want: " [@alice]",
		},
		{
			name: "handle bundles into PR group when both present",
			refs: []change.Reference{pr1, handleGH},
			want: " [PR [#1](https://github.com/o/r/pull/1) @alice]",
		},
		{
			name: "handle bundles into issue group when no PR present",
			refs: []change.Reference{iss1, handleGH},
			want: " [Issue [#10](https://github.com/o/r/issues/10) @alice]",
		},
		{
			name: "handle bundles into PR when both issue and PR present, issue stays alone",
			refs: []change.Reference{iss1, pr1, handleGH},
			want: " [Issue [#10](https://github.com/o/r/issues/10)] [PR [#1](https://github.com/o/r/pull/1) @alice]",
		},
		{
			name: "ref with empty URL goes to trailing other bracket",
			refs: []change.Reference{pr1, noURL},
			want: " [PR [#1](https://github.com/o/r/pull/1)] [CVE-2024-0001]",
		},
		{
			name: "multiple of each kind preserves input order, label stays singular",
			refs: []change.Reference{pr1, iss1, pr2, iss2, handleGH},
			want: " [Issue [#10](https://github.com/o/r/issues/10) [#11](https://github.com/o/r/issues/11)] [PR [#1](https://github.com/o/r/pull/1) [#2](https://github.com/o/r/pull/2) @alice]",
		},
		{
			name: "github @-handle renders bare without markdown link inside host group",
			refs: []change.Reference{pr1, handleGH},
			want: " [PR [#1](https://github.com/o/r/pull/1) @alice]",
		},
		{
			name: "non-github @-handle still renders as markdown link",
			refs: []change.Reference{pr1, handleOther},
			want: " [PR [#1](https://github.com/o/r/pull/1) [@bob](https://example.com/bob)]",
		},
		{
			name: "all four buckets render in fixed order: issue, PR, other (handles bundled into PR)",
			refs: []change.Reference{handleGH, weird, pr1, iss1},
			want: " [Issue [#10](https://github.com/o/r/issues/10)] [PR [#1](https://github.com/o/r/pull/1) @alice] [[release-notes](https://example.com/notes)]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, formatReferences(tt.refs))
		})
	}
}

func Test_formatSummary_trimsRecognizedPrefix(t *testing.T) {
	tests := []struct {
		name       string
		text       string
		recognized []string
		want       string
	}{
		{
			name: "standard prefix trimmed without any recognized types",
			text: "feat: add a thing",
			want: "- add a thing\n",
		},
		{
			name:       "non-standard recognized prefix is trimmed",
			text:       "deps: bump foo to v2",
			recognized: []string{"deps"},
			want:       "- bump foo to v2\n",
		},
		{
			name: "non-standard prefix is left intact when not recognized",
			text: "deps: bump foo to v2",
			want: "- deps: bump foo to v2\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatSummary(change.Change{Text: tt.text}, tt.recognized)
			require.Equal(t, tt.want, got)
		})
	}
}
