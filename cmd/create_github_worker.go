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

	var lastRelease *release.Release
	if appConfig.SinceTag != "" {
		lastRelease, err = summer.Release(appConfig.SinceTag)
		if err != nil {
			return nil, nil, fmt.Errorf("unable to fetch specific release: %w", err)
		}
	} else {
		lastRelease, err = summer.LastRelease()
		if err != nil {
			return nil, nil, fmt.Errorf("unable to determine last release: %w", err)
		}
	}

	log.Infof("since tag=%q date=%q", lastRelease.Version, internal.FormatDateTime(lastRelease.Date))

	releaseTag, releaseCommit, err := getCurrentReleaseInfo(appConfig.UntilTag, appConfig.CliOptions.RepoPath)
	if err != nil {
		return nil, nil, err
	}
	releaseVersion := releaseTag
	releaseDisplayVersion := releaseTag
	if releaseTag == "" {
		releaseDisplayVersion = "(Unreleased)"
		releaseVersion = releaseCommit
	}

	log.Infof("until tag=%q commit=%q", releaseTag, releaseCommit)

	changes, err := summer.Changes(lastRelease.Version, releaseTag)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to summarize changes: %w", err)
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
