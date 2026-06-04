package toolchain

import (
	"path"
	"strings"
)

// globMatch reports whether the slash-separated path name matches the glob pattern. It supports
// "**" (matches zero or more path segments), and standard single-segment wildcards ("*", "?",
// character classes) within a segment via path.Match. A leading "./" on either argument is ignored.
func globMatch(pattern, name string) bool {
	pattern = strings.TrimPrefix(pattern, "./")
	name = strings.TrimPrefix(name, "./")
	return matchSegments(strings.Split(pattern, "/"), strings.Split(name, "/"))
}

func matchSegments(pat, name []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			rest := pat[1:]
			// "**" at the end matches all remaining segments (including none).
			if len(rest) == 0 {
				return true
			}
			// otherwise try to consume zero or more segments before matching the remainder.
			for i := 0; i <= len(name); i++ {
				if matchSegments(rest, name[i:]) {
					return true
				}
			}
			return false
		}

		if len(name) == 0 {
			return false
		}
		if !matchSegment(pat[0], name[0]) {
			return false
		}
		pat = pat[1:]
		name = name[1:]
	}
	return len(name) == 0
}

func matchSegment(pat, name string) bool {
	ok, err := path.Match(pat, name)
	if err != nil {
		// malformed pattern (e.g. unterminated class) — fall back to literal comparison.
		return pat == name
	}
	return ok
}
