package release

import (
	"fmt"
	"strings"

	"github.com/anchore/chronicle/internal/git"

	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/coreos/go-semver/semver"
)

func FindNextVersion(currentVersion string, changes change.Changes, enforceV0 bool, noChangesBumpsPatch bool) (string, error) {
	var breaking, feature, patch bool
	for _, c := range changes {
		for _, chTy := range c.ChangeTypes {
			switch chTy.Kind {
			case change.SemVerMajor:
				if enforceV0 {
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
		if !noChangesBumpsPatch {
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

func FindNextUniqueVersion(currentVersion string, changes change.Changes, enforceV0 bool, noChangesBumpsPatch bool, repoPath string) (string, string, error) {
	nextReleaseVersion, err := FindNextVersion(currentVersion, changes, enforceV0, noChangesBumpsPatch)
	if err != nil {
		return "", "", err
	}

	var nextSemanticVersion = nextReleaseVersion

	tags, err := git.TagsFromLocal(repoPath)
	if err != nil {
		return "", "", err
	}
retry:
	for {
		for _, t := range tags {
			if t.Name == nextReleaseVersion {
				// looks like there is already a tag for this speculative release, let's choose a patch variant of this
				verObj, err := semver.NewVersion(strings.TrimLeft(nextReleaseVersion, "v"))
				if err != nil {
					return "", "", err
				}
				verObj.BumpPatch()

				var prefix string
				if strings.HasPrefix(nextReleaseVersion, "v") {
					prefix = "v"
				}

				releaseVersionCandidate := prefix + verObj.String()

				nextReleaseVersion = releaseVersionCandidate
				continue retry
			}
		}
		// we've checked that there are no existing tags that match the next release
		break
	}

	return nextReleaseVersion, nextSemanticVersion, nil
}
