package change

type TypeTitles []TypeTitle

// TypeTitle is a changetype paired with the section title that should be used in the changelog.
type TypeTitle struct {
	ChangeType Type
	Title      string
}

func (tts TypeTitles) Types() (ty []Type) {
	for _, c := range tts {
		ty = append(ty, c.ChangeType)
	}
	return ty
}
