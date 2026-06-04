package toolchain

import "github.com/anchore/chronicle/chronicle/dependency"

// Config controls toolchain detection.
type Config struct {
	// Enabled is the master opt-in switch. When false, Detect is a no-op.
	Enabled bool
	// Ecosystems is the exact set of ecosystems to run (replace semantics). Empty means all known.
	Ecosystems []dependency.Ecosystem
	// Ignore holds path globs excluded from discovery (e.g. "**/vendor/**").
	Ignore []string
	// Paths maps an ecosystem to its discovery globs, overriding the detector default.
	Paths map[dependency.Ecosystem][]string
}

// DefaultIgnore returns the path globs excluded from toolchain source discovery by default, so a
// vendored or test-fixture manifest is not mistaken for a real declared requirement. Callers can
// append their own excludes (e.g. the dependencies `exclude` list).
func DefaultIgnore() []string {
	return []string{
		"**/vendor/**",
		"**/node_modules/**",
		"**/testdata/**",
		"**/examples/**",
	}
}
