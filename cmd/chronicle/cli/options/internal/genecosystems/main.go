// Command genecosystems regenerates ecosystems.gen.yaml from the pinned syft
// module's cataloger capabilities. Run it via `go generate ./...`.
//
// syft exposes no importable API for the ecosystem→glob mapping (the data lives
// in an internal package behind an unexported embed.FS), so we read the public
// per-ecosystem capabilities.yaml files out of the module cache at generate time
// and bake a chronicle-owned, version-pinned copy. Re-run after bumping syft;
// CI compares the result against the committed file to catch drift.
//
// Output is scoped to ecosystems chronicle's registry (dependency.Ecosystem)
// knows, so auto-detection speaks the same canonical vocabulary as the rest of
// the codebase. syft ecosystems with no registry entry are skipped and logged.
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/anchore/chronicle/chronicle/dependency"
)

const outputFile = "ecosystems.gen.yaml"

// auto targets declared language dependencies: the "declared" tag scopes to
// source manifests/lockfiles (excluding installed/image package catalogers) and
// "language" excludes OS packages (deb/rpm), binaries, and IaC/CI ecosystems
// (terraform, github-actions) that aren't source library dependencies.
var requiredSelectors = []string{"declared", "language"}

// capDoc mirrors the parts of syft's capabilities.yaml schema we consume.
type capDoc struct {
	Catalogers []struct {
		Ecosystem string   `yaml:"ecosystem"`
		Selectors []string `yaml:"selectors"`
		Parsers   []struct {
			Detector struct {
				Method   string   `yaml:"method"`
				Criteria []string `yaml:"criteria"`
			} `yaml:"detector"`
		} `yaml:"parsers"`
	} `yaml:"catalogers"`
}

type ecosystemEntry struct {
	Ecosystem string   `yaml:"ecosystem"`
	Globs     []string `yaml:"globs"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "genecosystems:", err)
		os.Exit(1)
	}
}

func run() error {
	dir, version, err := syftModule()
	if err != nil {
		return err
	}

	files, err := filepath.Glob(filepath.Join(dir, "syft", "pkg", "cataloger", "*", "capabilities.yaml"))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no capabilities.yaml files found under %q", dir)
	}

	globsByEco, skipped, err := collectGlobs(files)
	if err != nil {
		return err
	}

	entries := orderedEntries(globsByEco)
	if err := writeOutput(entries, version); err != nil {
		return err
	}

	fmt.Printf("wrote %d ecosystems to %s (from syft %s)\n", len(entries), outputFile, version)
	if len(skipped) > 0 {
		fmt.Printf("skipped %d syft ecosystem(s) with no chronicle registry entry: %s\n",
			len(skipped), strings.Join(sortedKeys(skipped), ", "))
	}
	return nil
}

// collectGlobs parses each capabilities file and unions the declared-language glob
// criteria per chronicle ecosystem. It also returns the syft ecosystems skipped
// because chronicle's registry doesn't know them.
func collectGlobs(files []string) (globsByEco map[dependency.Ecosystem]map[string]struct{}, skipped map[string]struct{}, err error) {
	globsByEco = map[dependency.Ecosystem]map[string]struct{}{}
	skipped = map[string]struct{}{}
	for _, f := range files {
		data, err := os.ReadFile(f)
		if err != nil {
			return nil, nil, err
		}
		var doc capDoc
		if err := yaml.Unmarshal(data, &doc); err != nil {
			return nil, nil, fmt.Errorf("parse %s: %w", f, err)
		}
		collectFromDoc(doc, globsByEco, skipped)
	}
	return globsByEco, skipped, nil
}

// collectFromDoc folds one capabilities document's declared-language catalogers into
// the running glob/skip maps.
func collectFromDoc(doc capDoc, globsByEco map[dependency.Ecosystem]map[string]struct{}, skipped map[string]struct{}) {
	for _, c := range doc.Catalogers {
		if !hasAll(toSet(c.Selectors), requiredSelectors) {
			continue
		}
		globs := globCriteria(c.Parsers)
		if len(globs) == 0 {
			continue
		}
		// map the syft ecosystem onto chronicle's canonical registry so
		// auto-detection only emits ecosystems the rest of the code knows.
		eco, ok := dependency.ParseEcosystem(c.Ecosystem)
		if !ok {
			skipped[c.Ecosystem] = struct{}{}
			continue
		}
		if globsByEco[eco] == nil {
			globsByEco[eco] = map[string]struct{}{}
		}
		for _, g := range globs {
			globsByEco[eco][g] = struct{}{}
		}
	}
}

// orderedEntries emits one entry per ecosystem in canonical registry order, so the
// detection output order has a single source of truth.
func orderedEntries(globsByEco map[dependency.Ecosystem]map[string]struct{}) []ecosystemEntry {
	var entries []ecosystemEntry
	for _, eco := range dependency.Ecosystems() {
		if globs, ok := globsByEco[eco]; ok {
			entries = append(entries, ecosystemEntry{Ecosystem: eco.String(), Globs: sortedKeys(globs)})
		}
	}
	return entries
}

// writeOutput marshals the entries with the generated-file header to outputFile.
func writeOutput(entries []ecosystemEntry, version string) error {
	body, err := yaml.Marshal(struct {
		Ecosystems []ecosystemEntry `yaml:"ecosystems"`
	}{entries})
	if err != nil {
		return err
	}
	header := fmt.Sprintf("# Code generated from github.com/anchore/syft %s by genecosystems; DO NOT EDIT.\n"+
		"# Regenerate with `go generate ./...` after bumping syft.\n", version)
	return os.WriteFile(outputFile, append([]byte(header), body...), 0600)
}

// syftModule returns the module cache directory and version of the pinned syft
// dependency, avoiding any assumptions about GOMODCACHE layout. `go mod download`
// both fetches the module and reports where it landed — a fresh checkout or a
// stale CI module cache won't have syft extracted, and `go list -m` alone reports
// an empty Dir in that case rather than downloading it.
func syftModule() (dir, version string, err error) {
	out, err := exec.Command("go", "mod", "download", "-json", "github.com/anchore/syft").Output()
	if err != nil {
		return "", "", fmt.Errorf("downloading syft module: %w", err)
	}
	var m struct {
		Dir     string
		Version string
	}
	if err := json.Unmarshal(out, &m); err != nil || m.Dir == "" || m.Version == "" {
		return "", "", fmt.Errorf("unexpected `go mod download` output: %q", string(out))
	}
	return m.Dir, m.Version, nil
}

func globCriteria(parsers []struct {
	Detector struct {
		Method   string   `yaml:"method"`
		Criteria []string `yaml:"criteria"`
	} `yaml:"detector"`
}) []string {
	var out []string
	for _, p := range parsers {
		if p.Detector.Method == "glob" {
			out = append(out, p.Detector.Criteria...)
		}
	}
	return out
}

func toSet(vals []string) map[string]struct{} {
	s := make(map[string]struct{}, len(vals))
	for _, v := range vals {
		s[v] = struct{}{}
	}
	return s
}

func hasAll(set map[string]struct{}, keys []string) bool {
	for _, k := range keys {
		if _, ok := set[k]; !ok {
			return false
		}
	}
	return true
}

func sortedKeys(set map[string]struct{}) []string {
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
