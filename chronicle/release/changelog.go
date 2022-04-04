package release

import (
	"fmt"
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/internal"
	"github.com/anchore/chronicle/internal/log"
	"github.com/scylladb/go-set/strset"
	"sort"
	"time"
)

type ChangelogConfig struct {
	VersionSpeculator
	RepoPath         string
	SinceTag         string
	UntilTag         string
	ChangeTypeTitles []change.TypeTitle
}

func Changelog(summer Summarizer, config ChangelogConfig) (*Release, *Description, error) {
	startRelease, err := getChangelogStartingRelease(summer, config.SinceTag)
	if err != nil {
		return nil, nil, err
	}

	// TODO: support when there hasn't been the first release

	log.Infof("since tag=%q date=%q", startRelease.Version, internal.FormatDateTime(startRelease.Date))

	releaseVersion, releaseDisplayVersion, changes, err := changelogChanges(startRelease.Version, summer, config)
	if err != nil {
		return nil, nil, err
	}

	logChanges(changes)

	return startRelease, &Description{
		Release: Release{
			Version: releaseDisplayVersion,
			Date:    time.Now(),
		},
		VCSReferenceURL:  summer.ReferenceURL(releaseVersion),
		VCSChangesURL:    summer.ChangesURL(startRelease.Version, releaseVersion),
		Changes:          changes,
		SupportedChanges: config.ChangeTypeTitles,
		Notice:           "", // TODO...
	}, nil
}

func changelogChanges(startReleaseVersion string, summer Summarizer, config ChangelogConfig) (string, string, []change.Change, error) {
	endReleaseVersion := config.UntilTag
	endReleaseDisplay := config.UntilTag
	if config.UntilTag == "" {
		endReleaseDisplay = "(Unreleased)"
	}

	changes, err := summer.Changes(startReleaseVersion, config.UntilTag)
	if err != nil {
		return "", "", nil, fmt.Errorf("unable to summarize changes: %w", err)
	}

	if config.VersionSpeculator != nil {
		if endReleaseVersion == "" {
			specEndReleaseVersion, err := speculateNextVersion(config.VersionSpeculator, startReleaseVersion, changes)
			if err != nil {
				log.Warnf("unable to speculate next release version: %+v", err)
			} else {
				endReleaseVersion, endReleaseDisplay = specEndReleaseVersion, specEndReleaseVersion
			}
		} else {
			log.Infof("not speculating next version current head tag=%q", endReleaseVersion)
		}
	}

	return endReleaseVersion, endReleaseDisplay, changes, nil
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
	log.Infof("speculative release version=%q", nextUniqueVersion)
	return nextUniqueVersion, nil
}

func getChangelogStartingRelease(summer Summarizer, sinceTag string) (*Release, error) {
	var lastRelease *Release
	var err error
	if sinceTag != "" {
		lastRelease, err = summer.Release(sinceTag)
		if err != nil {
			return nil, fmt.Errorf("unable to fetch specific release: %w", err)
		}
	} else {
		lastRelease, err = summer.LastRelease()
		if err != nil {
			return nil, fmt.Errorf("unable to determine last release: %w", err)
		}
	}
	return lastRelease, nil
}

func logChanges(changes change.Changes) {
	log.Infof("discovered changes: %d", len(changes))

	set := strset.New()
	count := make(map[string]int)
	lookup := make(map[string]change.Type)
	for _, c := range changes {
		for _, ty := range c.ChangeTypes {
			_, exists := count[ty.Name]
			if !exists {
				count[ty.Name] = 0
			}
			count[ty.Name]++
			set.Add(ty.Name)
			lookup[ty.Name] = ty
		}
	}

	typeNames := set.List()
	sort.Strings(typeNames)

	for idx, tyName := range typeNames {
		var branch = "├──"
		if idx == len(typeNames)-1 {
			branch = "└──"
		}
		t := lookup[tyName]
		if t.Kind != change.SemVerUnknown {
			log.Debugf("  %s %s (%s bump): %d", branch, tyName, t.Kind, count[tyName])
		} else {
			log.Debugf("  %s %s: %d", branch, tyName, count[tyName])
		}
	}
}
