package change

type Type struct {
	Name string
	Kind SemVerKind
}

func NewType(name string, kind SemVerKind) Type {
	return Type{
		Name: name,
		Kind: kind,
	}
}

func ContainsAny(query, against []Type) bool {
	for _, qt := range query {
		for _, at := range against {
			if qt.Name == at.Name {
				return true
			}
		}
	}
	return false
}
