package markdown

import (
	"bytes"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
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

func Test_removeConventionalCommitPrefix(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		// positive cases
		{name: "feat: add user authentication", want: "add user authentication"},
		{name: "fix: resolve null pointer exception", want: "resolve null pointer exception"},
		{name: "docs: update README", want: "update README"},
		{name: "style: format code according to style guide", want: "format code according to style guide"},
		{name: "refactor: extract reusable function", want: "extract reusable function"},
		{name: "perf: optimize database queries", want: "optimize database queries"},
		{name: "test: add unit tests", want: "add unit tests"},
		{name: "build: update build process", want: "update build process"},
		{name: "ci: configure Travis CI", want: "configure Travis CI"},
		{name: "chore: perform maintenance tasks", want: "perform maintenance tasks"},
		// positive case odd balls
		{name: "chore: can end with punctuation.", want: "can end with punctuation."},
		{name: "revert!: revert: previous: commit", want: "revert: previous: commit"},
		{name: "feat(api)!: implement new: API endpoints", want: "implement new: API endpoints"},
		{name: "feat!: add awesome new feature (closes #123)", want: "add awesome new feature (closes #123)"},
		{name: "fix(ui): fix layout issue (fixes #456)", want: "fix layout issue (fixes #456)"},
		// negative cases
		{name: "reallycoolthing: is done!", want: "reallycoolthing: is done!"},
		{name: "feature: is done!", want: "feature: is done!"},
		{name: "feat(scope):   ", want: "feat(scope):   "},
		{name: "feat(scope):something", want: "feat(scope):something"},
		{name: "feat: something\n wicked this way comes", want: "feat: something\n wicked this way comes"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, removeConventionalCommitPrefix(tt.name), "removeConventionalCommitPrefix(%v)", tt.name)
		})
	}
}
