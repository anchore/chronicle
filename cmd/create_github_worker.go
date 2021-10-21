package cmd

import (
	"fmt"
	"time"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/chronicle/release/summarizer/github"
	"github.com/anchore/chronicle/internal/git"
	"github.com/anchore/chronicle/internal/log"
)

// nolint:funlen
func createChangelogFromGithub() (*release.Description, error) {
	summer, err := github.NewChangeSummarizer(appConfig.CliOptions.RepoPath, appConfig.Github.ToGithubConfig())
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
	log.Infof("since ref=%q date=%q", lastRelease.Version, lastRelease.Date)

	releaseVersion := appConfig.UntilTag
	releaseDisplayVersion := releaseVersion
	if releaseVersion == "" {
		releaseVersion = "(Unreleased)"
		releaseDisplayVersion = releaseVersion
		// check if the current commit is tagged, then use that
		releaseTag, err := git.HeadTagOrCommit(appConfig.CliOptions.RepoPath)
		if err != nil {
			return nil, fmt.Errorf("problem while attempting to find head tag: %w", err)
		}
		if releaseTag != "" {
			releaseVersion = releaseTag
		}
	}

	log.Infof("until ref=%q", releaseVersion)

	changes, err := summer.Changes(lastRelease.Version, appConfig.UntilTag)
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
		VCSTagURL:        summer.TagURL(lastRelease.Version),
		VCSChangesURL:    summer.ChangesURL(lastRelease.Version, appConfig.UntilTag),
		Changes:          changes,
		SupportedChanges: supportedChanges,
		Notice:           "", // TODO...
	}, nil
}
