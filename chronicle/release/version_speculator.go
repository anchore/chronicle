package release

import "github.com/anchore/chronicle/chronicle/release/change"

// SpeculationBehavior contains configuration that controls how to determine the next release version.
type SpeculationBehavior struct {
	EnforceV0           bool // if true, and the version is currently < v1.0 breaking changes do NOT bump the major semver field; instead the minor version is bumped.
	NoChangesBumpsPatch bool // if true, and no changes make up the current release, still bump the patch semver field.
}

// VersionSpeculator is something that is capable of surmising the next release based on the set of changes from the last release.
type VersionSpeculator interface {
	// NextIdealVersion reports the next version based on the currentVersion and a set of changes
	NextIdealVersion(currentVersion string, changes change.Changes) (string, error)

	// NextUniqueVersion is the same as NextIdealVersion, however, it additionally considers if the final speculated version is already released. If so, then the next non-released patch version (relative to the ideal version) is returned.
	NextUniqueVersion(currentVersion string, changes change.Changes) (string, error)
}
