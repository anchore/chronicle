package toolchain

import "strings"

// matcher routes file paths to the detector responsible for them, honoring per-ecosystem path
// globs and the global ignore list. It is built once per detection run so each git ref is walked
// a single time.
type matcher struct {
	ecos      []ecosystemPaths
	ignore    []string
	recursive bool
}

type ecosystemPaths struct {
	detector Detector
	globs    []string
}

func newMatcher(detectors []Detector, cfg Config) *matcher {
	m := &matcher{ignore: cfg.Ignore, recursive: cfg.Recursive}
	for _, d := range detectors {
		globs := cfg.Paths[d.Tool()]
		if len(globs) == 0 {
			globs = d.DefaultPaths()
		}
		m.ecos = append(m.ecos, ecosystemPaths{detector: d, globs: globs})
	}
	return m
}

// match reports whether a path is a discovery candidate for any configured ecosystem. It is the
// predicate handed to the git tree walk.
func (m *matcher) match(p string) bool {
	return m.detectorFor(p) != nil
}

// detectorFor returns the detector whose configured globs match p (and that is not ignored), or
// nil. An explicitly-listed path wins over an ignore glob only when it is not itself a glob match
// against the ignore set; ignore is applied first to keep discovery quiet by default.
func (m *matcher) detectorFor(p string) Detector {
	// non-recursive discovery: a manifest in any subdirectory is out of scope, so reject anything
	// below the root regardless of the per-ecosystem globs (which default to recursive "**/...").
	if !m.recursive && strings.Contains(strings.TrimPrefix(p, "./"), "/") {
		return nil
	}
	if m.isIgnored(p) {
		return nil
	}
	for _, e := range m.ecos {
		for _, g := range e.globs {
			if globMatch(g, p) {
				return e.detector
			}
		}
	}
	return nil
}

func (m *matcher) isIgnored(p string) bool {
	for _, ig := range m.ignore {
		if globMatch(ig, p) {
			return true
		}
	}
	return false
}
