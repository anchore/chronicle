package change

type Type string

const (
	AddedFeature      Type = "Added"
	ChangedFeature    Type = "Changed"
	DeprecatedFeature Type = "Deprecated"
	RemovedFeature    Type = "Removed"
	BugFix            Type = "Fixed"
	Vulnerability     Type = "Security"
)

var AllChangeTypes = []Type{
	AddedFeature,
	ChangedFeature,
	DeprecatedFeature,
	RemovedFeature,
	BugFix,
	Vulnerability,
}

func containsAnyChangeType(query, against []Type) bool {
	for _, qt := range query {
		for _, at := range against {
			if qt == at {
				return true
			}
		}
	}
	return false
}
