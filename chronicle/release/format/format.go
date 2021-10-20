package format

import "strings"

type Format string

var (
	MarkdownFormat Format = "md"
	JSONFormat     Format = "json"
)

func FromString(option string) *Format {
	option = strings.ToLower(option)
	switch option {
	case "m", "md", "markdown":
		return &MarkdownFormat
	case "j", "json", "jason":
		return &JSONFormat
	default:
		return nil
	}
}

func All() []Format {
	return []Format{
		MarkdownFormat,
		JSONFormat,
	}
}

func Default() Format {
	return MarkdownFormat
}
