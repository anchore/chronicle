package dependency

import "sort"

// VersionComparer classifies the direction of a version change for a given
// package ecosystem. Implemented by scan.grypeVersionComparer in production;
// a trivial fake is used in tests.
type VersionComparer interface {
	// Compare returns <0 if a<b, 0 if equal, >0 if a>b, and ok=false when
	// the versions are not comparable in this ecosystem.
	Compare(pkgType, a, b string) (int, bool)
}

// Compare builds a Diff from two snapshots. Packages present only in since are
// Removed; only in until are Added; present in both with a differing version are
// Updated or Downgraded (determined by cmp). Equal versions are omitted. The
// Changes slice is sorted deterministically (by Type then Name) so output is
// stable for tests and rendering.
func Compare(since, until Snapshot, cmp VersionComparer) Diff {
	sinceIdx := indexPackages(since.Packages)
	untilIdx := indexPackages(until.Packages)

	var changes []PackageChange

	// packages present in since — either Removed or changed
	for key, sincePkg := range sinceIdx {
		untilPkg, inUntil := untilIdx[key]
		if !inUntil {
			changes = append(changes, PackageChange{
				Name:        sincePkg.Name,
				Type:        sincePkg.Type,
				Ecosystem:   sincePkg.Ecosystem,
				FromVersion: sincePkg.Version,
				ToVersion:   "",
				Kind:        Removed,
			})
			continue
		}

		if sincePkg.Version == untilPkg.Version {
			// no change; omit
			continue
		}

		kind := classifyVersionChange(sincePkg.Type, sincePkg.Version, untilPkg.Version, cmp)
		changes = append(changes, PackageChange{
			Name:        sincePkg.Name,
			Type:        sincePkg.Type,
			Ecosystem:   untilPkg.Ecosystem,
			FromVersion: sincePkg.Version,
			ToVersion:   untilPkg.Version,
			Kind:        kind,
		})
	}

	// packages only in until are Added
	for key, untilPkg := range untilIdx {
		if _, inSince := sinceIdx[key]; !inSince {
			changes = append(changes, PackageChange{
				Name:        untilPkg.Name,
				Type:        untilPkg.Type,
				Ecosystem:   untilPkg.Ecosystem,
				FromVersion: "",
				ToVersion:   untilPkg.Version,
				Kind:        Added,
			})
		}
	}

	// NewDiff sorts and tallies, so the rollups can't drift from the changes.
	return NewDiff(changes)
}

// sortChanges orders changes deterministically by Type then Name so output is
// stable for tests and rendering. NewDiff calls it for every diff it builds.
func sortChanges(changes []PackageChange) {
	sort.Slice(changes, func(i, j int) bool {
		if changes[i].Type != changes[j].Type {
			return changes[i].Type < changes[j].Type
		}
		return changes[i].Name < changes[j].Name
	})
}

// indexPackages returns a map from PackageKey to Package for fast lookup.
func indexPackages(pkgs []Package) map[PackageKey]Package {
	idx := make(map[PackageKey]Package, len(pkgs))
	for _, p := range pkgs {
		idx[p.Key()] = p
	}
	return idx
}

// classifyVersionChange uses cmp to decide Updated vs Downgraded. When cmp
// reports ok=false (versions not comparable), we classify as Updated (changed
// but direction unknown).
func classifyVersionChange(pkgType, from, to string, cmp VersionComparer) ChangeKind {
	result, ok := cmp.Compare(pkgType, from, to)
	if !ok {
		// unknown direction — treat as updated
		return Updated
	}
	if result > 0 {
		// Compare(from, to) > 0 means from > to: the version decreased
		return Downgraded
	}
	// from < to (version increased), or string-differs-but-equal — treat as an update
	return Updated
}
