package markdown

import (
	"bytes"
	"testing"
	"time"

	"github.com/gkampitakis/go-snaps/snaps"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/wagoodman/go-presenter"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
)

func TestMarkdownPresenter_Present(t *testing.T) {
	must := func(m *Presenter, err error) *Presenter {
		require.NoError(t, err)
		return m
	}
	assertPresenterAgainstGoldenSnapshot(
		t,
		must(
			NewMarkdownPresenter(Config{
				// this is the default configuration from the CLI
				Title: `{{ .Version }}`,
				Description: release.Description{
					SupportedChanges: []change.TypeTitle{
						{
							ChangeType: change.NewType("bug", change.SemVerPatch),
							Title:      "Bug Fixes",
						},
						{
							ChangeType: change.NewType("added", change.SemVerMinor),
							Title:      "Added Features",
						},
						{
							ChangeType: change.NewType("breaking", change.SemVerMajor),
							Title:      "Breaking Changes",
						},
						{
							ChangeType: change.NewType("removed", change.SemVerMajor),
							Title:      "Removed Features",
						},
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
							References: []change.Reference{
								{
									Text: "#456",
									URL:  "https://github.com/anchore/syft/pull/456",
								},
							},
						},
						{
							ChangeTypes: []change.Type{change.NewType("added", change.SemVerMinor)},
							Text:        "added feature!",
							References: []change.Reference{
								{
									Text: "#457",
									URL:  "https://github.com/anchore/syft/pull/457",
								},
								{
									Text: "@wagoodman",
									URL:  "https://github.com/wagoodman",
								},
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
								{
									Text: "#458",
									URL:  "https://github.com/anchore/syft/pull/458",
								},
								{
									Text: "#450",
									URL:  "https://github.com/anchore/syft/issues/450",
								},
								{
									Text: "@wagoodman",
									URL:  "https://github.com/wagoodman",
								},
							},
						},
					},
					Notice: "notice!",
				},
			}),
		),
	)
}

func TestMarkdownPresenter_Present_NoTitle(t *testing.T) {
	must := func(m *Presenter, err error) *Presenter {
		require.NoError(t, err)
		return m
	}
	assertPresenterAgainstGoldenSnapshot(
		t,
		must(
			NewMarkdownPresenter(Config{
				Title: "",
				Description: release.Description{
					SupportedChanges: []change.TypeTitle{
						{
							ChangeType: change.NewType("bug", change.SemVerPatch),
							Title:      "Bug Fixes",
						},
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
							References: []change.Reference{
								{
									Text: "#456",
									URL:  "https://github.com/anchore/syft/pull/456",
								},
							},
						},
					},
					Notice: "notice!",
				},
			}),
		),
	)
}

func TestMarkdownPresenter_Present_NoChanges(t *testing.T) {
	must := func(m *Presenter, err error) *Presenter {
		require.NoError(t, err)
		return m
	}
	assertPresenterAgainstGoldenSnapshot(
		t,
		must(
			NewMarkdownPresenter(Config{
				Title: "Changelog",
				Description: release.Description{
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
			}),
		),
	)
}

type redactor func(s []byte) []byte

func assertPresenterAgainstGoldenSnapshot(t *testing.T, pres presenter.Presenter, redactors ...redactor) {
	t.Helper()

	var buffer bytes.Buffer
	err := pres.Present(&buffer)
	assert.NoError(t, err)
	actual := buffer.Bytes()

	// remove dynamic values, which should be tested independently
	for _, r := range redactors {
		actual = r(actual)
	}

	snaps.MatchSnapshot(t, string(actual))
}

func Test_removeConventionalCommitPrefix(t *testing.T) {

	tests := []struct {
		name string
		want string
	}{
		// positive cases
		{
			name: "feat: add user authentication",
			want: "add user authentication",
		},
		{
			name: "fix: resolve null pointer exception",
			want: "resolve null pointer exception",
		},
		{
			name: "docs: update README",
			want: "update README",
		},
		{
			name: "style: format code according to style guide",
			want: "format code according to style guide",
		},
		{
			name: "refactor: extract reusable function",
			want: "extract reusable function",
		},
		{
			name: "perf: optimize database queries",
			want: "optimize database queries",
		},
		{
			name: "test: add unit tests",
			want: "add unit tests",
		},
		{
			name: "build: update build process",
			want: "update build process",
		},
		{
			name: "ci: configure Travis CI",
			want: "configure Travis CI",
		},
		{
			name: "chore: perform maintenance tasks",
			want: "perform maintenance tasks",
		},
		// positive case odd balls
		{
			name: "chore: can end with punctuation.",
			want: "can end with punctuation.",
		},
		{
			name: "revert!: revert: previous: commit",
			want: "revert: previous: commit",
		},
		{
			name: "feat(api)!: implement new: API endpoints",
			want: "implement new: API endpoints",
		},
		{
			name: "feat!: add awesome new feature (closes #123)",
			want: "add awesome new feature (closes #123)",
		},
		{
			name: "fix(ui): fix layout issue (fixes #456)",
			want: "fix layout issue (fixes #456)",
		},
		// negative cases
		{
			name: "reallycoolthing: is done!",
			want: "reallycoolthing: is done!",
		},
		{
			name: "feature: is done!",
			want: "feature: is done!",
		},
		{
			// missing description... just leave it alone
			name: "feat(scope):   ",
			want: "feat(scope):   ",
		},
		{
			// missing whitespace in description (yes, that's a cc requirement)
			name: "feat(scope):something",
			want: "feat(scope):something",
		},
		{
			// has a newline
			name: "feat: something\n wicked this way comes",
			want: "feat: something\n wicked this way comes",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equalf(t, tt.want, removeConventionalCommitPrefix(tt.name), "removeConventionalCommitPrefix(%v)", tt.name)
		})
	}
}
