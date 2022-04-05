package change

// TypeSet is a unique set of types indexed by their name
type TypeSet map[string]Type

func (l TypeSet) Names() (results []string) {
	for name := range l {
		results = append(results, name)
	}
	return results
}

func (l TypeSet) ChangeTypes(labels ...string) (results []Type) {
	for _, label := range labels {
		if ct, exists := l[label]; exists {
			results = append(results, ct)
		}
	}
	return results
}
