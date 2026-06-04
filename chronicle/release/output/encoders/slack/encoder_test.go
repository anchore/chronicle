package slack

import (
	"bytes"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/require"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
)

func TestSlackPresenter_Present(t *testing.T) {
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

func TestSlackPresenter_Present_NoTitle(t *testing.T) {
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

func TestSlackPresenter_Present_NoChanges(t *testing.T) {
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

func TestSlackPresenter_Present_EscapesMrkdwn(t *testing.T) {
	assertEncoderAgainstGoldenSnapshot(t,
		`release <{{ .Version }}> & friends`,
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
					// reserved chars in the body, plus trailing punctuation that
					// must be trimmed before escaping (so "&gt;" stays intact).
					Text:       "fix: handle <-chan close & guard A > B.",
					References: []change.Reference{{Text: "#456", URL: "https://github.com/anchore/syft/pull/456"}},
				},
			},
		},
	)
}

func TestSlackPresenter_Present_Toolchain(t *testing.T) {
	assertEncoderAgainstGoldenSnapshot(t,
		"Changelog",
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
			Toolchain: &release.ToolchainData{
				Updates: []release.ToolchainUpdate{
					{Tool: "go", Source: "go directive", File: "go.mod", From: "1.21", To: "1.23", Direction: release.ToolchainUpgrade},
				},
			},
		},
	)
}

func TestSlackPresenter_Present_ToolchainDowngrade(t *testing.T) {
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
			Toolchain: &release.ToolchainData{
				Updates: []release.ToolchainUpdate{
					{Tool: "go", Source: "go directive", File: "go.mod", From: "1.23", To: "1.21", Direction: release.ToolchainDowngrade},
				},
			},
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
			want: " [PR <https://github.com/o/r/pull/1|#1>]",
		},
		{
			name: "single issue",
			refs: []change.Reference{iss1},
			want: " [Issue <https://github.com/o/r/issues/10|#10>]",
		},
		{
			name: "handle alone (no issue, no PR) renders standalone",
			refs: []change.Reference{handleGH},
			want: " [`@alice`]",
		},
		{
			name: "handle bundles into PR group when both present",
			refs: []change.Reference{pr1, handleGH},
			want: " [PR <https://github.com/o/r/pull/1|#1> `@alice`]",
		},
		{
			name: "handle bundles into issue group when no PR present",
			refs: []change.Reference{iss1, handleGH},
			want: " [Issue <https://github.com/o/r/issues/10|#10> `@alice`]",
		},
		{
			name: "handle bundles into PR when both issue and PR present, issue stays alone",
			refs: []change.Reference{iss1, pr1, handleGH},
			want: " [Issue <https://github.com/o/r/issues/10|#10>] [PR <https://github.com/o/r/pull/1|#1> `@alice`]",
		},
		{
			name: "ref with empty URL goes to trailing other bracket",
			refs: []change.Reference{pr1, noURL},
			want: " [PR <https://github.com/o/r/pull/1|#1>] [CVE-2024-0001]",
		},
		{
			name: "multiple of each kind preserves input order, label stays singular",
			refs: []change.Reference{pr1, iss1, pr2, iss2, handleGH},
			want: " [Issue <https://github.com/o/r/issues/10|#10> <https://github.com/o/r/issues/11|#11>] [PR <https://github.com/o/r/pull/1|#1> <https://github.com/o/r/pull/2|#2> `@alice`]",
		},
		{
			name: "github @-handle renders backticked without link inside host group",
			refs: []change.Reference{pr1, handleGH},
			want: " [PR <https://github.com/o/r/pull/1|#1> `@alice`]",
		},
		{
			name: "non-github @-handle also renders backticked (slack does not auto-credit)",
			refs: []change.Reference{pr1, handleOther},
			want: " [PR <https://github.com/o/r/pull/1|#1> `@bob`]",
		},
		{
			name: "all four buckets render in fixed order: issue, PR, other (handles bundled into PR)",
			refs: []change.Reference{handleGH, weird, pr1, iss1},
			want: " [Issue <https://github.com/o/r/issues/10|#10>] [PR <https://github.com/o/r/pull/1|#1> `@alice`] [<https://example.com/notes|release-notes>]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.Equal(t, tt.want, formatReferences(tt.refs))
		})
	}
}
