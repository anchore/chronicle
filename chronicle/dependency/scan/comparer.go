package scan

import (
	"github.com/anchore/chronicle/chronicle/dependency"
	"github.com/anchore/grype/grype/version"
)

// grypeVersionComparer implements dependency.VersionComparer using grype's
// ecosystem-aware version package, so update-vs-downgrade is decided correctly
// per package type (semver, deb, rpm, go, python, …) rather than by string
// comparison. Lives here because it touches grype; report injects it into
// dependency.Compare.
type grypeVersionComparer struct{}

// NewVersionComparer returns the grype-backed VersionComparer for injection
// into dependency.Compare.
func NewVersionComparer() dependency.VersionComparer {
	return grypeVersionComparer{}
}

func (grypeVersionComparer) Compare(pkgType, a, b string) (int, bool) {
	// map the syft package type string to a grype version format; an unknown
	// ecosystem is not comparable.
	format := version.ParseFormat(pkgType)
	if format == version.UnknownFormat {
		return 0, false
	}

	va := version.New(a, format)
	vb := version.New(b, format)
	if va == nil || vb == nil {
		return 0, false
	}

	result, err := va.Compare(vb)
	if err != nil {
		return 0, false
	}
	return result, true
}
