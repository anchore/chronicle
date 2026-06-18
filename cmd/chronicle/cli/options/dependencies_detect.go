package options

import (
	_ "embed"
	"fmt"
	"os"
	"strings"
	"sync"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"

	"github.com/anchore/chronicle/chronicle/dependency"
)

// autoEcosystem expands to the ecosystems detected at the repo root; noneEcosystem
// disables the feature entirely and wins over every other value (including auto).
const (
	autoEcosystem = "auto"
	noneEcosystem = "none"
)

//go:generate go run ./internal/genecosystems

// ecosystemCapabilities is the chronicle-owned, version-pinned copy of syft's
// declared-language cataloger globs (per ecosystem), baked from the syft module
// cache by `go generate` (see internal/genecosystems). syft offers no importable
// API for this mapping, so we carry our own copy and re-derive it on each syft
// bump rather than hand-maintaining glob patterns. Entries are scoped to
// chronicle's dependency.Ecosystem registry and listed in canonical order.
//
//go:embed ecosystems.gen.yaml
var ecosystemCapabilities []byte

// ecosystemGlobs pairs a chronicle ecosystem identifier with the root-manifest
// globs that imply it.
type ecosystemGlobs struct {
	Ecosystem string   `yaml:"ecosystem"`
	Globs     []string `yaml:"globs"`
}

var (
	loadEcosystemsOnce sync.Once
	loadedEcosystems   []ecosystemGlobs
	loadEcosystemsErr  error
)

// detectionTable parses the embedded capabilities once and caches the result.
func detectionTable() ([]ecosystemGlobs, error) {
	loadEcosystemsOnce.Do(func() {
		var doc struct {
			Ecosystems []ecosystemGlobs `yaml:"ecosystems"`
		}
		if err := yaml.Unmarshal(ecosystemCapabilities, &doc); err != nil {
			loadEcosystemsErr = fmt.Errorf("unable to parse embedded ecosystem capabilities: %w", err)
			return
		}
		loadedEcosystems = doc.Ecosystems
	})
	return loadedEcosystems, loadEcosystemsErr
}

// detectEcosystems inspects the top level of root for the manifest globs syft's
// declared-language catalogers match, and returns the canonical syft selectors
// (dependency.Ecosystem.Selector) for the ecosystems present, in canonical order.
// Detection is non-recursive: it decides which ecosystems to enable, and syft
// handles the deep scan from there. Because the globs come from syft, detection
// tracks what syft would actually catalog — e.g. JavaScript keys off lockfiles,
// not package.json. Returns nil when nothing matches.
func detectEcosystems(root string) ([]string, error) {
	table, err := detectionTable()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("unable to read project root %q for ecosystem detection: %w", root, err)
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			names = append(names, e.Name())
		}
	}

	var out []string
	for _, eco := range table {
		// route through the registry so the emitted selector is canonical and
		// any ecosystem the registry no longer knows is skipped rather than
		// passed to syft as an unknown selector.
		known, ok := dependency.ParseEcosystem(eco.Ecosystem)
		if !ok {
			continue
		}
		if anyGlobMatches(eco.Globs, names) {
			out = append(out, known.Selector())
		}
	}
	return out, nil
}

// anyGlobMatches reports whether any glob matches any of the root-level filenames.
// syft globs are "**/"-prefixed (recursive); doublestar matches those against a
// bare root filename (e.g. "**/go.mod" matches "go.mod"), so root-only detection
// works without walking the tree.
func anyGlobMatches(globs, names []string) bool {
	for _, g := range globs {
		for _, n := range names {
			if ok, _ := doublestar.Match(g, n); ok {
				return true
			}
		}
	}
	return false
}

// ResolveEcosystems expands the configured ecosystem sentinels against the project
// root and rewrites Ecosystems with concrete syft selectors. The "none" sentinel wins
// over everything (clears the list, disabling the feature); otherwise "auto" is
// replaced by the detected selectors and merged with any explicit ones (deduped,
// detected first in canonical order, explicit order preserved). hadAuto reports whether
// an auto token was present so the caller can log the detection outcome.
func (c *Dependencies) ResolveEcosystems(root string) (resolved []string, hadAuto bool, err error) {
	cleaned := c.CleanedEcosystems()

	for _, e := range cleaned {
		if strings.EqualFold(e, noneEcosystem) {
			c.Ecosystems = nil
			return nil, false, nil
		}
	}

	var out []string
	seen := make(map[string]struct{})
	add := func(selector string) {
		if _, ok := seen[selector]; ok {
			return
		}
		seen[selector] = struct{}{}
		out = append(out, selector)
	}

	for _, e := range cleaned {
		if !strings.EqualFold(e, autoEcosystem) {
			continue
		}
		hadAuto = true
		detected, derr := detectEcosystems(root)
		if derr != nil {
			return nil, hadAuto, derr
		}
		for _, d := range detected {
			add(d)
		}
	}

	// keep explicit selectors after the detected set, preserving their order.
	for _, e := range cleaned {
		if strings.EqualFold(e, autoEcosystem) {
			continue
		}
		add(e)
	}

	c.Ecosystems = out
	return out, hadAuto, nil
}
