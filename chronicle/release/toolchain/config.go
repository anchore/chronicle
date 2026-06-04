package toolchain

// Config controls toolchain detection.
type Config struct {
	// Enabled is the master opt-in switch. When false, Detect is a no-op.
	Enabled bool
	// Ecosystems is the exact set of ecosystems to run (replace semantics). Empty means all known.
	Ecosystems []string
	// Ignore holds path globs excluded from discovery (e.g. "**/vendor/**").
	Ignore []string
	// Paths maps an ecosystem identifier to its discovery globs, overriding the detector default.
	Paths map[string][]string
}
