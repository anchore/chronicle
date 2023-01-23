package markdown

import (
	"bytes"
	"flag"
	"testing"
	"time"

	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/stretchr/testify/assert"
	"github.com/wagoodman/go-presenter"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/go-testutils"
)

var updateMarkdownPresenterGoldenFiles = flag.Bool("update-markdown", false, "update the *.golden files for markdown presenters")

func TestMarkdownPresenter_Present(t *testing.T) {
	must := func(m *Presenter, err error) *Presenter {
		if err != nil {
			t.Fatalf(err.Error())
		}
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
						{
							ChangeType: change.NewType("unlabeled", change.SemVerMajor),
							Title:      "Unlabeled PRs",
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
									Text: "456",
									URL:  "https://github.com/anchore/syft/pull/456",
								},
							},
						},
						{
							ChangeTypes: []change.Type{change.NewType("added", change.SemVerMinor)},
							Text:        "added feature",
						},
						{
							ChangeTypes: []change.Type{change.NewType("added", change.SemVerMinor)},
							Text:        "another added feature",
						},
						{
							ChangeTypes: []change.Type{change.NewType("breaking", change.SemVerMajor)},
							Text:        "breaking change",
						},
					},
					Notice: "notice!",
				},
			}),
		),
		*updateMarkdownPresenterGoldenFiles,
	)
}

type redactor func(s []byte) []byte

func assertPresenterAgainstGoldenSnapshot(t *testing.T, pres presenter.Presenter, updateSnapshot bool, redactors ...redactor) {
	t.Helper()

	var buffer bytes.Buffer
	err := pres.Present(&buffer)
	assert.NoError(t, err)
	actual := buffer.Bytes()

	// replace the expected snapshot contents with the current presenter contents
	if updateSnapshot {
		testutils.UpdateGoldenFileContents(t, actual)
	}

	var expected = testutils.GetGoldenFileContents(t)

	// remove dynamic values, which should be tested independently
	for _, r := range redactors {
		actual = r(actual)
		expected = r(expected)
	}

	if !bytes.Equal(expected, actual) {
		dmp := diffmatchpatch.New()
		diffs := dmp.DiffMain(string(expected), string(actual), true)
		t.Errorf("mismatched output:\n%s", dmp.DiffPrettyText(diffs))
	}
}
