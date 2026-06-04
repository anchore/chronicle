package toolchain

// dirtyLister is the slice of git.Interface needed to inspect uncommitted working-tree changes.
type dirtyLister interface {
	WorktreeDirtyPaths() ([]string, error)
}

// DirtySourceFiles returns the configured toolchain source files that have uncommitted changes in
// the working tree. Detection reads committed objects, so when a changelog ends at HEAD any such
// edit is invisible to the diff — callers use this to warn that a toolchain change present only in
// the working tree won't appear until committed. Returns nil when detection is disabled, no
// ecosystem is selected, or the tree is clean.
func DirtySourceFiles(gitter dirtyLister, cfg Config) ([]string, error) {
	if !cfg.Enabled {
		return nil, nil
	}

	detectors := selectedDetectors(cfg.Ecosystems)
	if len(detectors) == 0 {
		return nil, nil
	}

	dirty, err := gitter.WorktreeDirtyPaths()
	if err != nil {
		return nil, err
	}

	m := newMatcher(detectors, cfg)
	var matched []string
	for _, p := range dirty {
		if m.match(p) {
			matched = append(matched, p)
		}
	}
	return matched, nil
}
