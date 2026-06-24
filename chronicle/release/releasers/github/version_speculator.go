package github

import (
	"fmt"
	"strings"

	"github.com/coreos/go-semver/semver"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/internal/git"
	"github.com/anchore/chronicle/internal/log"
)

var _ release.VersionSpeculator = (*VersionSpeculator)(nil)

type VersionSpeculator struct {
	git git.Interface
	release.SpeculationBehavior
}

func NewVersionSpeculator(gitter git.Interface, behavior release.SpeculationBehavior) VersionSpeculator {
	return VersionSpeculator{
		git:                 gitter,
		SpeculationBehavior: behavior,
	}
}

func (s VersionSpeculator) NextIdealVersion(currentVersion string, changes change.Changes) (string, error) {
	var breaking, feature, patch bool
	for _, c := range changes {
		for _, chTy := range c.ChangeTypes {
			switch chTy.Kind {
			case change.SemVerMajor:
				if s.EnforceV0 {
					feature = true
				} else {
					breaking = true
				}
			case change.SemVerMinor:
				feature = true
			case change.SemVerPatch:
				patch = true
			}
		}
	}

	// when there's no prior release, default to v0.0.0 as the starting point
	if currentVersion == "" {
		currentVersion = "v0.0.0"
	}

	v, err := semver.NewVersion(strings.TrimLeft(currentVersion, "v"))
	if err != nil {
		return "", fmt.Errorf("invalid current version given: %q: %w", currentVersion, err)
	}
	original := *v

	if patch {
		v.BumpPatch()
	}

	if feature {
		v.BumpMinor()
	}

	if breaking {
		v.BumpMajor()
	}

	if v.String() == original.String() {
		if !s.NoChangesBumpsPatch {
			return "", fmt.Errorf("no changes found that affect the version (changes=%d)", len(changes))
		}
		v.BumpPatch()
	}

	prefix := ""
	if strings.HasPrefix(currentVersion, "v") {
		prefix = "v"
	}
	return prefix + v.String(), nil
}

func (s VersionSpeculator) NextUniqueVersion(currentVersion string, changes change.Changes) (string, error) {
	nextReleaseVersion, err := s.NextIdealVersion(currentVersion, changes)
	if err != nil {
		return "", err
	}

	tags, err := s.git.TagsFromLocal()
	if err != nil {
		return "", err
	}

	taken := make(map[string]struct{}, len(tags))
	for _, t := range tags {
		taken[t.Name] = struct{}{}
	}

	// when the ideal version's tag is already taken (e.g. from a failed release that can't be
	// rolled back in the go ecosystem) we must roll forward to the next unused version. we roll
	// forward along the same field that the release bumped: a feature release rolls to the next
	// minor (v0.13.0 -> v0.14.0, never v0.13.1) and a patch release rolls to the next patch
	// (v0.13.1 -> v0.13.2). major releases are special: we never speculate a brand new major
	// version, so a taken major rolls forward by minor instead (v2.0.0 -> v2.1.0).
	bumpKind := s.effectiveBumpKind(changes)

	for {
		if _, ok := taken[nextReleaseVersion]; !ok {
			break
		}

		verObj, err := semver.NewVersion(strings.TrimLeft(nextReleaseVersion, "v"))
		if err != nil {
			return "", err
		}

		switch bumpKind {
		case change.SemVerMinor, change.SemVerMajor:
			verObj.BumpMinor()
		default:
			verObj.BumpPatch()
		}

		var prefix string
		if strings.HasPrefix(nextReleaseVersion, "v") {
			prefix = "v"
		}

		takenVersion := nextReleaseVersion
		nextReleaseVersion = prefix + verObj.String()

		log.WithFields("taken", takenVersion, "next", nextReleaseVersion).
			Warnf("speculated release version %q already has a tag; rolling forward to %q", takenVersion, nextReleaseVersion)
	}

	return nextReleaseVersion, nil
}

// effectiveBumpKind reports the highest-significance semver field that the given changes would
// bump, honoring EnforceV0 (which downgrades a major bump to a minor bump for v0.x projects).
func (s VersionSpeculator) effectiveBumpKind(changes change.Changes) change.SemVerKind {
	kind := change.Significance(changes)
	if kind == change.SemVerMajor && s.EnforceV0 {
		kind = change.SemVerMinor
	}
	return kind
}
