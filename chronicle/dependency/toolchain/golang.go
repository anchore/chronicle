package toolchain

import (
	"golang.org/x/mod/modfile"
	"golang.org/x/mod/semver"

	"github.com/anchore/chronicle/chronicle/dependency"
)

// goDetector reads the minimum Go version declared by the `go` directive in a go.mod file. The
// `toolchain` directive (a pinned toolchain) is intentionally ignored — only the declared minimum
// is reported.
type goDetector struct{}

func (goDetector) Tool() dependency.Ecosystem { return dependency.EcosystemGo }

func (goDetector) DefaultPaths() []string { return []string{"**/go.mod"} }

func (goDetector) Requirement(path string, content []byte) ([]requirement, error) {
	// ParseLax tolerates directives and dependency lines we don't care about, so a go.mod with
	// constructs newer than our x/mod version still yields the `go` directive.
	f, err := modfile.ParseLax(path, content, nil)
	if err != nil {
		return nil, err
	}
	if f.Go == nil || f.Go.Version == "" {
		return nil, nil
	}
	return []requirement{{source: "go directive", version: f.Go.Version}}, nil
}

// Compare orders go directive versions (e.g. "1.21", "1.21.4") using semver semantics. The
// versions need a "v" prefix to be valid semver, which x/mod/semver requires.
func (goDetector) Compare(from, to string) (int, bool) {
	fv, tv := "v"+from, "v"+to
	if !semver.IsValid(fv) || !semver.IsValid(tv) {
		return 0, false
	}
	return semver.Compare(tv, fv), true
}
