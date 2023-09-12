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
				Title: "Changelog",
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
							Text:        "Redirect cursor hide/show to stderr",
							References: []change.Reference{
								{
									Text: "#456",
									URL:  "https://github.com/anchore/syft/pull/456",
								},
							},
						},
						{
							ChangeTypes: []change.Type{change.NewType("added", change.SemVerMinor)},
							Text:        "added feature",
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
							Text:        "another added feature",
						},
						{
							ChangeTypes: []change.Type{change.NewType("breaking", change.SemVerMajor)},
							Text:        "breaking change",
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
				Title: "",
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
