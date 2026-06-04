package toolchain

import (
	"fmt"
	"sort"
	"strings"

	"github.com/anchore/chronicle/chronicle/dependency"
	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/internal/git"
	"github.com/anchore/chronicle/internal/log"
)

// fileLister is the slice of git.Interface that detection depends on: reading file content at a
// ref without a working-tree checkout.
type fileLister interface {
	ListFilesAtRef(ref string, match func(path string) bool) ([]git.FileBlob, error)
}

// Detect reads the configured toolchain source files at sinceRef and untilRef, diffs the declared
// requirements, and returns the changes plus any reconciliation warnings. It degrades gracefully:
// per-file parse failures are logged and skipped, listing failures abort detection without an
// error, and a run that finds nothing returns (nil, nil) — so changelog generation always
// continues regardless of toolchain detection outcome.
func Detect(gitter fileLister, cfg Config, sinceRef, untilRef string) (*release.ToolchainData, error) {
	if !cfg.Enabled {
		return nil, nil
	}
	if sinceRef == "" {
		// no baseline to compare against (changelog starts at the beginning of history).
		log.Debug("toolchain detection skipped: no since ref to compare against")
		return nil, nil
	}

	detectors := selectedDetectors(cfg.Ecosystems)
	if len(detectors) == 0 {
		return nil, nil
	}

	m := newMatcher(detectors, cfg)

	sinceFiles, err := gitter.ListFilesAtRef(sinceRef, m.match)
	if err != nil {
		log.WithFields("error", err, "ref", sinceRef).Warn("toolchain detection: unable to list files at since ref; skipping")
		return nil, nil
	}
	untilFiles, err := gitter.ListFilesAtRef(untilRef, m.match)
	if err != nil {
		log.WithFields("error", err, "ref", untilRef).Warn("toolchain detection: unable to list files at until ref; skipping")
		return nil, nil
	}

	since := extractRequirements(m, sinceFiles)
	until := extractRequirements(m, untilFiles)

	byTool := make(map[dependency.Ecosystem]Detector, len(detectors))
	for _, d := range detectors {
		byTool[d.Tool()] = d
	}

	return diffRequirements(since, until, byTool), nil
}

// reqKey uniquely identifies one requirement source across the two refs being compared.
type reqKey struct {
	tool   dependency.Ecosystem
	file   string
	source string
}

func extractRequirements(m *matcher, files []git.FileBlob) map[reqKey]string {
	out := make(map[reqKey]string)
	for _, f := range files {
		d := m.detectorFor(f.Path)
		if d == nil {
			continue
		}
		reqs, err := d.Requirement(f.Path, f.Content)
		if err != nil {
			// graceful degradation: a single unparseable file must not break detection.
			log.WithFields("error", err, "file", f.Path, "tool", d.Tool()).Debug("toolchain detection: unable to parse file; skipping")
			continue
		}
		for _, r := range reqs {
			out[reqKey{tool: d.Tool(), file: f.Path, source: r.source}] = r.version
		}
	}
	return out
}

// diffRequirements emits an update for every source that exists at BOTH refs and whose declared
// version changed. Sources added or removed between refs are deliberately not reported as changes
// (that would be noise from files appearing/disappearing rather than a requirement bump).
func diffRequirements(since, until map[reqKey]string, byTool map[dependency.Ecosystem]Detector) *release.ToolchainData {
	var updates []release.ToolchainUpdate
	for k, toVer := range until {
		fromVer, ok := since[k]
		if !ok || fromVer == toVer {
			continue
		}
		updates = append(updates, release.ToolchainUpdate{
			Tool:      k.tool,
			Source:    k.source,
			File:      k.file,
			From:      fromVer,
			To:        toVer,
			Direction: direction(byTool[k.tool], fromVer, toVer),
		})
	}

	if len(updates) == 0 {
		return nil
	}

	sort.Slice(updates, func(i, j int) bool {
		if updates[i].Tool != updates[j].Tool {
			return updates[i].Tool < updates[j].Tool
		}
		if updates[i].File != updates[j].File {
			return updates[i].File < updates[j].File
		}
		return updates[i].Source < updates[j].Source
	})

	return &release.ToolchainData{Updates: updates, Warnings: reconcile(updates)}
}

// direction asks the owning detector whether the version moved up or down. It returns an unknown
// direction when there is no detector or the versions are not comparable.
func direction(d Detector, from, to string) release.ToolchainDirection {
	if d == nil {
		return release.ToolchainDirectionUnknown
	}
	cmp, ok := d.Compare(from, to)
	switch {
	case !ok:
		return release.ToolchainDirectionUnknown
	case cmp < 0:
		return release.ToolchainDowngrade
	case cmp > 0:
		return release.ToolchainUpgrade
	default:
		return release.ToolchainDirectionUnknown
	}
}

// reconcile warns when sources within a single ecosystem disagree on the resulting version (e.g.
// two modules bump the minimum Go version to different values). Identical transitions across
// multiple files are not a conflict and produce no warning.
func reconcile(updates []release.ToolchainUpdate) []release.ToolchainWarning {
	byTool := make(map[dependency.Ecosystem][]release.ToolchainUpdate)
	var toolOrder []dependency.Ecosystem
	for _, u := range updates {
		if _, ok := byTool[u.Tool]; !ok {
			toolOrder = append(toolOrder, u.Tool)
		}
		byTool[u.Tool] = append(byTool[u.Tool], u)
	}

	var warnings []release.ToolchainWarning
	for _, tool := range toolOrder {
		ups := byTool[tool]

		states := make(map[string][]string) // resulting version -> files
		var stateOrder []string
		for _, u := range ups {
			if _, ok := states[u.To]; !ok {
				stateOrder = append(stateOrder, u.To)
			}
			states[u.To] = append(states[u.To], u.File)
		}

		if len(stateOrder) <= 1 {
			continue
		}

		var parts, files []string
		for _, v := range stateOrder {
			fs := states[v]
			files = append(files, fs...)
			parts = append(parts, fmt.Sprintf("%s (%s)", v, strings.Join(fs, ", ")))
		}
		sort.Strings(files)
		warnings = append(warnings, release.ToolchainWarning{
			Tool:    tool,
			Message: fmt.Sprintf("%s sources disagree on the resulting version: %s", tool.Label(), strings.Join(parts, "; ")),
			Files:   files,
		})
	}
	return warnings
}

func selectedDetectors(ecosystems []dependency.Ecosystem) []Detector {
	if len(ecosystems) == 0 {
		ds := make([]Detector, 0, len(registry))
		for _, name := range KnownEcosystems() {
			ds = append(ds, registry[name])
		}
		return ds
	}

	var ds []Detector
	for _, name := range ecosystems {
		if d, ok := registry[name]; ok {
			ds = append(ds, d)
			continue
		}
		log.WithFields("ecosystem", name).Warn("toolchain detection: unknown ecosystem requested; skipping")
	}
	return ds
}
