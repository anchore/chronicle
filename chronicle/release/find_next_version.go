package release

import (
	"fmt"
	"strings"

	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/coreos/go-semver/semver"
)

func FindNextVersion(currentVersion string, changes change.Changes, enforceV0 bool) (string, error) {
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
		return "", fmt.Errorf("no changes found that affect the version (changes=%d)", len(changes))
	}

	prefix := ""
	if strings.HasPrefix(currentVersion, "v") {
		prefix = "v"
	}
	return prefix + v.String(), nil
}
