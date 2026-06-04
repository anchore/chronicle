package toolchain

import (
	"github.com/anchore/chronicle/chronicle/dependency"
)

// registry holds the detectors keyed by ecosystem.
var registry = map[dependency.Ecosystem]Detector{}

func register(d Detector) {
	registry[d.Tool()] = d
}

func init() {
	register(goDetector{})
}

// requirement is a single declared toolchain requirement extracted from one source file.
// The owning detector supplies the ecosystem and the caller supplies the file path.
type requirement struct {
	source  string // human label for the field read, e.g. "go directive"
	version string // the declared requirement as written, e.g. "1.21" or ">=3.9"
}

// Detector extracts declared toolchain requirements for a single ecosystem. Detection is
// source-only: detectors parse file content read from a git ref and never trigger a build,
// install, or environment inspection.
type Detector interface {
	// Tool is the ecosystem this detector handles (e.g. dependency.EcosystemGo).
	Tool() dependency.Ecosystem
	// DefaultPaths are the glob patterns inspected when the user provides no path override.
	DefaultPaths() []string
	// Requirement parses one file's content and returns the requirement(s) it declares. A file
	// that declares nothing relevant returns an empty slice (not an error).
	Requirement(path string, content []byte) ([]requirement, error)
	// Compare reports whether the 'to' version is newer (>0), older (<0), or equal (==0) to 'from'.
	// ok is false when the versions are not comparable (e.g. opaque constraints), leaving the
	// upgrade/downgrade direction unknown.
	Compare(from, to string) (cmp int, ok bool)
}

// KnownEcosystems returns the ecosystems with a registered detector, in canonical order.
func KnownEcosystems() []dependency.Ecosystem {
	var out []dependency.Ecosystem
	for _, e := range dependency.Ecosystems() {
		if _, ok := registry[e]; ok {
			out = append(out, e)
		}
	}
	return out
}

// DefaultPaths returns the default discovery globs for an ecosystem, or nil when there is no
// detector. It lets configuration defaults derive from the detectors so the globs have a single
// source of truth.
func DefaultPaths(eco dependency.Ecosystem) []string {
	if d, ok := registry[eco]; ok {
		return d.DefaultPaths()
	}
	return nil
}
