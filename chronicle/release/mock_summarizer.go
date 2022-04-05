package release

import (
	"github.com/anchore/chronicle/chronicle/release/change"
)

type MockSummarizer struct {
	MockLastRelease string
	MockRelease     string
	MockChanges     []change.Change
	MockRefURL      string
	MockChangesURL  string
}

func (m MockSummarizer) LastRelease() (*Release, error) {
	if m.MockLastRelease == "" {
		return nil, nil
	}
	return &Release{
		Version: m.MockLastRelease,
	}, nil
}

func (m MockSummarizer) Release(_ string) (*Release, error) {
	if m.MockRelease == "" {
		return nil, nil
	}
	return &Release{
		Version: m.MockRelease,
	}, nil
}

func (m MockSummarizer) Changes(_, _ string) ([]change.Change, error) {
	return m.MockChanges, nil
}

func (m MockSummarizer) ReferenceURL(_ string) string {
	return m.MockRefURL
}

func (m MockSummarizer) ChangesURL(_, _ string) string {
	return m.MockChangesURL
}
