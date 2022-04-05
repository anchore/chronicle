package cmd

import (
	"fmt"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/chronicle/release/releasers/github"
	"github.com/anchore/chronicle/internal/git"
	"github.com/anchore/chronicle/internal/log"
)

func createChangelogFromGithub() (*release.Release, *release.Description, error) {
	ghConfig, err := appConfig.Github.ToGithubConfig()
	if err != nil {
		return nil, nil, err
	}

	gitter, err := git.New(appConfig.CliOptions.RepoPath)
	if err != nil {
		return nil, nil, err
	}

	summer, err := github.NewSummarizer(gitter, ghConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create summarizer: %w", err)
	}

	changeTypeTitles, err := getGithubSupportedChanges()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get change type titles from github config: %w", err)
	}

	var untilTag = appConfig.UntilTag
	if untilTag == "" {
		untilTag, err = github.FindChangelogEndTag(summer, gitter)
		if err != nil {
			return nil, nil, err
		}
	}

	log.Infof("until tag=%q", untilTag)

	var speculator release.VersionSpeculator
	if appConfig.SpeculateNextVersion {
		speculator = github.NewVersionSpeculator(gitter, release.SpeculationBehavior{
			EnforceV0:           appConfig.EnforceV0,
			NoChangesBumpsPatch: true,
		})
	}

	changelogConfig := release.ChangelogInfoConfig{
		RepoPath:          appConfig.CliOptions.RepoPath,
		SinceTag:          appConfig.SinceTag,
		UntilTag:          untilTag,
		VersionSpeculator: speculator,
		ChangeTypeTitles:  changeTypeTitles,
	}

	return release.ChangelogInfo(summer, changelogConfig)
}

func getGithubSupportedChanges() ([]change.TypeTitle, error) {
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
