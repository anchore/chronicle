package github

import "github.com/anchore/chronicle/chronicle/release/change"

type labelSet map[string]change.Type

func defaultLabelChangeTypes() map[string]change.Type {
	return map[string]change.Type{
		"bug":         change.BugFix,
		"security":    change.Vulnerability,
		"deprecated":  change.DeprecatedFeature,
		"enhancement": change.AddedFeature,
		// TODO: do there values by default make sense?
		//"removed":         change.RemovedFeature,
		//"changed":         change.ChangedFeature,
	}
}

func (l labelSet) labels() (results []string) {
	for name := range l {
		results = append(results, name)
	}
	return results
}

func (l labelSet) changeTypes(labels ...string) (results []change.Type) {
	for _, label := range labels {
		if ct, exists := l[label]; exists {
			results = append(results, ct)
		}
	}
	return results
}
