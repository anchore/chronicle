package cmd

import (
	"fmt"
	"time"

	"github.com/anchore/chronicle/internal"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/chronicle/release/summarizer/github"
	"github.com/anchore/chronicle/internal/git"
	"github.com/anchore/chronicle/internal/log"
)

func createChangelogFromGithub() (*release.Release, *release.Description, error) {
	config, err := appConfig.Github.ToGithubConfig()
	if err != nil {
		return nil, nil, err
	}
	summer, err := github.NewSummarizer(appConfig.CliOptions.RepoPath, config)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create summarizer: %w", err)
	}

	lastRelease, err := getLatestRelease(summer)
	if err != nil {
		return nil, nil, err
	}

	log.Infof("since tag=%q date=%q", lastRelease.Version, internal.FormatDateTime(lastRelease.Date))

	releaseVersion, releaseDisplayVersion, changes, err := getChanges(lastRelease.Version, summer)
	if err != nil {
		return nil, nil, err
	}

	logChanges(changes)

	supportedChanges, err := getSupportedChanges()
	if err != nil {
		return nil, nil, err
	}

	return lastRelease, &release.Description{
		Release: release.Release{
			Version: releaseDisplayVersion,
			Date:    time.Now(),
		},
		VCSReferenceURL:  summer.ReferenceURL(releaseVersion),
		VCSChangesURL:    summer.ChangesURL(lastRelease.Version, releaseVersion),
		Changes:          changes,
		SupportedChanges: supportedChanges,
		Notice:           "", // TODO...
	}, nil
}

func getChanges(lastReleaseVersion string, summer release.Summarizer) (string, string, []change.Change, error) {
	releaseTag, releaseCommit, err := getCurrentReleaseInfo(appConfig.UntilTag, appConfig.CliOptions.RepoPath)
	if err != nil {
		return "", "", nil, err
	}
	releaseVersion := releaseTag
	releaseDisplayVersion := releaseTag
	if releaseTag == "" {
		releaseDisplayVersion = "(Unreleased)"
		releaseVersion = releaseCommit
	}

	log.Infof("until tag=%q commit=%q", releaseTag, releaseCommit)

	changes, err := summer.Changes(lastReleaseVersion, releaseTag)
	if err != nil {
		return "", "", nil, fmt.Errorf("unable to summarize changes: %w", err)
	}

	if appConfig.SpeculateNextVersion {
		if releaseTag == "" {
			nextReleaseVersion, err := release.FindNextVersion(lastReleaseVersion, changes, appConfig.EnforceV0)
			if err != nil {
				log.Warnf("unable to speculate next release version: %+v", err)
			} else {
				releaseTag = nextReleaseVersion
				releaseVersion = nextReleaseVersion
				releaseDisplayVersion = nextReleaseVersion
				log.Infof("speculative release version=%q", releaseTag)
			}
		} else {
			log.Infof("not speculating next version since tag=%q was discovered")
		}
	}

	return releaseVersion, releaseDisplayVersion, changes, nil
}

func getLatestRelease(summer release.Summarizer) (*release.Release, error) {
	var lastRelease *release.Release
	var err error
	if appConfig.SinceTag != "" {
		lastRelease, err = summer.Release(appConfig.SinceTag)
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

func getSupportedChanges() ([]change.TypeTitle, error) {
	var supportedChanges []change.TypeTitle
	for _, c := range appConfig.Github.Changes {
		// TODO: this could be one source of truth upstream
		k := change.ParseSemVerKind(c.SemVerKind)
		if k == change.SemVerUnknown {
			return nil, fmt.Errorf("unable to parse semver kind: %q", c.SemVerKind)
		}
		t := change.NewType(c.Type, k)
		supportedChanges = append(supportedChanges, change.TypeTitle{
			ChangeType: t,
			Title:      c.Title,
		})
	}
	return supportedChanges, nil
}

func getCurrentReleaseInfo(explicitReleaseVersion, repoPath string) (string, string, error) {
	if explicitReleaseVersion != "" {
		return explicitReleaseVersion, explicitReleaseVersion, nil
	}

	commitRef, err := git.HeadCommit(repoPath)
	if err != nil {
		return "", "", fmt.Errorf("problem while attempting to find head ref: %w", err)
	}

	// check if the current commit is tagged, then use that
	releaseTag, err := git.HeadTag(repoPath)
	if err != nil {
		return "", "", fmt.Errorf("problem while attempting to find head tag: %w", err)
	}
	if releaseTag != "" {
		// a tag was found, reference it
		return releaseTag, commitRef, nil
	}

	// fallback to referencing the commit directly
	return "", commitRef, nil
}
