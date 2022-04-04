package release

import "github.com/anchore/chronicle/chronicle/release/change"

type SpeculationBehavior struct {
	EnforceV0           bool
	NoChangesBumpsPatch bool
}

type VersionSpeculator interface {
	NextIdealVersion(currentVersion string, changes change.Changes) (string, error)
	NextUniqueVersion(currentVersion string, changes change.Changes) (string, error)
}
