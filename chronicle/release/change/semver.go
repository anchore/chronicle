package change

import "strings"

type SemVerKind int

const (
	SemVerUnknown SemVerKind = iota
	SemVerPatch
	SemVerMinor
	SemVerMajor
)

var SemVerFields = []SemVerKind{
	SemVerMajor,
	SemVerMinor,
	SemVerPatch,
}

func ParseSemVerKind(semver string) SemVerKind {
	for _, f := range SemVerFields {
		if f.String() == strings.ToLower(semver) {
			return f
		}
	}
	return SemVerUnknown
}

func (f SemVerKind) String() string {
	switch f {
	case SemVerMajor:
		return "major"
	case SemVerMinor:
		return "minor"
	case SemVerPatch:
		return "patch"
	}
	return ""
}

func Significance(changes []Change) SemVerKind {
	var current = SemVerUnknown
	for _, c := range changes {
		for _, t := range c.ChangeTypes {
			if t.Kind > current {
				current = t.Kind
			}
		}
	}
	return current
}
