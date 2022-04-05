package release

import "github.com/anchore/chronicle/chronicle/release/change"

type MockVersionSpeculator struct {
	MockNextIdealVersion  string
	MockNextUniqueVersion string
}

func (m MockVersionSpeculator) NextIdealVersion(_ string, _ change.Changes) (string, error) {
	return m.MockNextIdealVersion, nil
}

func (m MockVersionSpeculator) NextUniqueVersion(_ string, _ change.Changes) (string, error) {
	return m.MockNextUniqueVersion, nil
}
