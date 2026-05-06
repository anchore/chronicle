package output

import (
	"fmt"
	"path/filepath"
	"strings"
)

// stdoutToken is the explicit value for "write to stdout" on the right-hand
// side of an `=`. Borrowed from the unix convention.
const stdoutToken = "-"

// Spec is one parsed `-o NAME[=PATH]` entry. An empty Path means stdout.
type Spec struct {
	Name string
	Path string // empty for stdout
}

// IsStdout reports whether the spec writes to stdout.
func (s Spec) IsStdout() bool {
	return s.Path == ""
}

// ParseSpec parses a single `NAME[=PATH]` token. NAME=- and bare NAME both
// mean stdout. NAME= (empty path) is rejected as ambiguous.
func ParseSpec(raw string) (Spec, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return Spec{}, fmt.Errorf("empty output spec")
	}
	parts := strings.SplitN(raw, "=", 2)
	name := strings.TrimSpace(parts[0])
	if name == "" {
		return Spec{}, fmt.Errorf("output spec %q: missing format name", raw)
	}
	if len(parts) == 1 {
		return Spec{Name: name}, nil
	}
	path := parts[1]
	if path == "" {
		return Spec{}, fmt.Errorf("output spec %q: empty path after '='", raw)
	}
	if path == stdoutToken {
		return Spec{Name: name}, nil
	}
	return Spec{Name: name, Path: path}, nil
}

// ParseSpecs parses all raw `-o` values.
func ParseSpecs(raws []string) ([]Spec, error) {
	specs := make([]Spec, 0, len(raws))
	for _, raw := range raws {
		s, err := ParseSpec(raw)
		if err != nil {
			return nil, err
		}
		specs = append(specs, s)
	}
	return specs, nil
}

// Validate checks structural rules that govern a spec set as a whole:
//   - at least one entry
//   - at most one entry writes to stdout (otherwise outputs would interleave)
//   - no two entries write to the same absolute file path
//
// Encoder-name validity is checked later by New, against the Encoders set
// the caller supplies — this layer is intentionally encoder-agnostic.
func Validate(specs []Spec) error {
	if len(specs) == 0 {
		return fmt.Errorf("no output specs")
	}

	var stdoutCount int
	seenPaths := map[string]string{} // abs path -> original spec string for diagnostics

	for _, s := range specs {
		if s.IsStdout() {
			stdoutCount++
			continue
		}
		abs, err := filepath.Abs(s.Path)
		if err != nil {
			return fmt.Errorf("output %q: cannot resolve path: %w", s.Name, err)
		}
		if prev, dup := seenPaths[abs]; dup {
			return fmt.Errorf("two outputs write to the same file %q (%s and %s)", abs, prev, formatSpec(s))
		}
		seenPaths[abs] = formatSpec(s)
	}
	if stdoutCount > 1 {
		return fmt.Errorf("at most one output may write to stdout; got %d", stdoutCount)
	}
	return nil
}

func formatSpec(s Spec) string {
	if s.IsStdout() {
		return s.Name
	}
	return s.Name + "=" + s.Path
}
