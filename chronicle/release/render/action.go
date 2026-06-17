package render

import "github.com/anchore/chronicle/chronicle/dependency"

// Action pairs a change kind with its display label. ActionOrder is the order
// kinds render within an ecosystem: listed kinds (updates, downgrades) first,
// then the rest (added, removed). Encoders share it so ordering and labels stay
// identical across formats.
type Action struct {
	Kind  dependency.ChangeKind
	Label string
}

// ActionOrder is the canonical per-ecosystem ordering and labelling of change
// kinds, shared by every encoder.
var ActionOrder = []Action{
	{dependency.Updated, "Updated"},
	{dependency.Downgraded, "Downgraded"},
	{dependency.Added, "Added"},
	{dependency.Removed, "Removed"},
}

// ChangesOfKind returns the subset of changes whose Kind matches, preserving the
// incoming order.
func ChangesOfKind(changes []dependency.PackageChange, kind dependency.ChangeKind) []dependency.PackageChange {
	var subset []dependency.PackageChange
	for _, c := range changes {
		if c.Kind == kind {
			subset = append(subset, c)
		}
	}
	return subset
}
