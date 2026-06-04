package toolchain

import "sort"

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
	// Tool is the ecosystem identifier (e.g. "go", "python", "node").
	Tool() string
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

// registry holds the detectors keyed by ecosystem identifier.
var registry = map[string]Detector{}

func register(d Detector) {
	registry[d.Tool()] = d
}

func init() {
	register(goDetector{})
}

// KnownEcosystems returns the registered ecosystem identifiers in sorted order.
func KnownEcosystems() []string {
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// DefaultPaths returns the default discovery globs for an ecosystem, or nil when unknown. It lets
// configuration defaults derive from the detectors so the globs have a single source of truth.
func DefaultPaths(tool string) []string {
	if d, ok := registry[tool]; ok {
		return d.DefaultPaths()
	}
	return nil
}
