package release

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/internal"
	"github.com/anchore/chronicle/internal/log"
)

type ChangelogInfoConfig struct {
	VersionSpeculator
	RepoPath         string
	SinceTag         string
	UntilTag         string
	ChangeTypeTitles []change.TypeTitle
}

// ChangelogInfo identifies the last release (the start of the changelog) and returns a description of the current (potentially speculative) release.
func ChangelogInfo(summer Summarizer, config ChangelogInfoConfig) (*Release, *Description, error) {
	startRelease, err := getChangelogStartingRelease(summer, config.SinceTag)
	if err != nil {
		return nil, nil, err
	}

	var startReleaseVersion string
	if startRelease != nil {
		log.WithFields("tag", startRelease.Version, "release-timestamp", internal.FormatDateTime(startRelease.Date)).Info("since")
		startReleaseVersion = startRelease.Version
	} else {
		log.Info("since the beginning of git history")
	}

	releaseVersion, changes, err := changelogChanges(startReleaseVersion, summer, config)
	if err != nil {
		return nil, nil, err
	}

	var releaseDisplayVersion = releaseVersion
	if releaseVersion == "" {
		releaseDisplayVersion = "(Unreleased)"
	}

	logChanges(changes)

	return startRelease, &Description{
		Release: Release{
			Version: releaseDisplayVersion,
			Date:    time.Now(),
		},
		VCSReferenceURL:  summer.ReferenceURL(releaseVersion),
		VCSChangesURL:    summer.ChangesURL(startReleaseVersion, releaseVersion),
		Changes:          changes,
		SupportedChanges: config.ChangeTypeTitles,
		Notice:           "", // TODO...
	}, nil
}

func changelogChanges(startReleaseVersion string, summer Summarizer, config ChangelogInfoConfig) (string, []change.Change, error) {
	endReleaseVersion := config.UntilTag

	changes, err := summer.Changes(startReleaseVersion, config.UntilTag)
	if err != nil {
		return "", nil, fmt.Errorf("unable to summarize changes: %w", err)
	}

	if config.VersionSpeculator != nil {
		if endReleaseVersion == "" {
			specEndReleaseVersion, err := speculateNextVersion(config.VersionSpeculator, startReleaseVersion, changes)
			if err != nil {
				log.Warnf("unable to speculate next release version: %+v", err)
			} else {
				endReleaseVersion = specEndReleaseVersion
			}
		} else {
			log.Infof("not speculating next version current head tag=%q", endReleaseVersion)
		}
	}

	return endReleaseVersion, changes, nil
}

func speculateNextVersion(speculator VersionSpeculator, startReleaseVersion string, changes []change.Change) (string, error) {
	// TODO: make this behavior configurable (follow semver on change or bump patch only)
	nextIdealVersion, err := speculator.NextIdealVersion(startReleaseVersion, changes)
	if err != nil {
		return "", err
	}
	nextUniqueVersion, err := speculator.NextUniqueVersion(startReleaseVersion, changes)
	if err != nil {
		return "", err
	}
	if nextUniqueVersion != nextIdealVersion {
		log.Debugf("speculated a release version that matches an existing tag=%q, selecting the next best version...", nextIdealVersion)
	}
	log.WithFields("version", nextUniqueVersion).Info("🔮 speculative release version")
	return nextUniqueVersion, nil
}

func getChangelogStartingRelease(summer Summarizer, sinceTag string) (*Release, error) {
	var lastRelease *Release
	var err error
	if sinceTag != "" {
		lastRelease, err = summer.Release(sinceTag)
		if err != nil {
			return nil, fmt.Errorf("unable to fetch specific release: %w", err)
		} else if lastRelease == nil {
			return nil, errors.New("unable to fetch release")
		}
	} else {
		lastRelease, err = summer.LastRelease()
		if err != nil {
			return nil, fmt.Errorf("unable to determine last release: %w", err)
		}
		if lastRelease == nil {
			// no prior release found, signal "since the beginning of time"
			log.Info("no prior GitHub release found; producing changelog from the beginning of git history")
			return nil, nil
		}
	}
	return lastRelease, nil
}

func logChanges(changes change.Changes) {
	byType := make(map[string][]change.Change)
	lookup := make(map[string]change.Type)
	for _, c := range changes {
		for _, ty := range c.ChangeTypes {
			byType[ty.Name] = append(byType[ty.Name], c)
			lookup[ty.Name] = ty
		}
	}

	typeNames := make([]string, 0, len(byType))
	for name := range byType {
		typeNames = append(typeNames, name)
	}
	sort.Strings(typeNames)

	// INFO rollup: total + a flat field per change type so structured loggers see each as its
	// own key (rather than a pre-folded "by-type=…" string).
	fields := []interface{}{"count", len(changes)}
	for _, tyName := range typeNames {
		t := lookup[tyName]
		var val string
		if t.Kind != change.SemVerUnknown {
			val = fmt.Sprintf("%d (%s)", len(byType[tyName]), t.Kind)
		} else {
			val = fmt.Sprintf("%d", len(byType[tyName]))
		}
		fields = append(fields, tyName, val)
	}
	log.WithFields(fields...).Info("📝 discovered changes")

	// DEBUG evidence tree: which changes fell into which type
	for tIdx, tyName := range typeNames {
		typeBranch := "├──"
		entryIndent := "│   "
		if tIdx == len(typeNames)-1 {
			typeBranch = "└──"
			entryIndent = "    "
		}
		t := lookup[tyName]
		if t.Kind != change.SemVerUnknown {
			log.Debugf("  %s %s (%d, %s bump)", typeBranch, tyName, len(byType[tyName]), t.Kind)
		} else {
			log.Debugf("  %s %s (%d)", typeBranch, tyName, len(byType[tyName]))
		}
		entries := byType[tyName]
		for eIdx, c := range entries {
			leaf := "├──"
			if eIdx == len(entries)-1 {
				leaf = "└──"
			}
			log.Debugf("  %s%s %s %s", entryIndent, leaf, primaryRef(c), c.Text)
		}
	}
}

// primaryRef returns the most identifying reference for a change (e.g. "#19"), or "" if none.
func primaryRef(c change.Change) string {
	for _, r := range c.References {
		if strings.HasPrefix(r.Text, "#") {
			return r.Text
		}
	}
	if len(c.References) > 0 {
		return c.References[0].Text
	}
	return ""
}
