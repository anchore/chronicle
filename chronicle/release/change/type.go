package change

type Type string

func ContainsAny(query, against []Type) bool {
	for _, qt := range query {
		for _, at := range against {
			if qt == at {
				return true
			}
		}
	}
	return false
}
