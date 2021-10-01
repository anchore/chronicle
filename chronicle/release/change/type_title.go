package change

type TypeTitles []TypeTitle

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
