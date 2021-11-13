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

func createChangelogFromGithub() (*release.Description, error) {
	summer, err := github.NewSummarizer(appConfig.CliOptions.RepoPath, appConfig.Github.ToGithubConfig())
	if err != nil {
		return nil, fmt.Errorf("unable to create summarizer: %w", err)
	}

	var lastRelease *release.Release
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

	log.Infof("since tag=%q date=%q", lastRelease.Version, internal.FormatDateTime(lastRelease.Date))

	releaseTag, releaseCommit, err := getCurrentReleaseInfo(appConfig.UntilTag, appConfig.CliOptions.RepoPath)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("unable to summarize changes: %w", err)
	}

	logChanges(changes)

	var supportedChanges []change.TypeTitle
	for _, c := range appConfig.Github.Changes {
		supportedChanges = append(supportedChanges, change.TypeTitle{
			ChangeType: c.Type,
			Title:      c.Title,
		})
	}

	return &release.Description{
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
