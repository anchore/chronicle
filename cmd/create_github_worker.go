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
	summer, err := github.NewSummarizer(appConfig.CliOptions.RepoPath, ghConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create summarizer: %w", err)
	}

	changeTypeTitles, err := getGithubSupportedChanges()
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get change type titles from github config: %w", err)
	}

	untilTag, err := findGithubChangelogEndTag(summer)
	if err != nil {
		return nil, nil, err
	}

	log.Infof("until tag=%q", untilTag)

	var speculator release.VersionSpeculator
	if appConfig.SpeculateNextVersion {
		speculator = github.NewVersionSpeculator(appConfig.CliOptions.RepoPath, release.SpeculationBehavior{
			EnforceV0:           appConfig.EnforceV0,
			NoChangesBumpsPatch: true,
		})
	}

	changelogConfig := release.ChangelogConfig{
		RepoPath:          appConfig.CliOptions.RepoPath,
		SinceTag:          appConfig.SinceTag,
		UntilTag:          untilTag,
		VersionSpeculator: speculator,
		ChangeTypeTitles:  changeTypeTitles,
	}

	return release.Changelog(summer, changelogConfig)
}

func findGithubChangelogEndTag(summer release.Summarizer) (string, error) {
	if appConfig.UntilTag != "" {
		return appConfig.UntilTag, nil
	}

	//commitRef, err := git.HeadCommit(appConfig.CliOptions.RepoPath)
	//if err != nil {
	//	return "", fmt.Errorf("problem while attempting to find head ref: %w", err)
	//}

	// check if the current commit is tagged, then use that
	currentTag, err := git.HeadTag(appConfig.CliOptions.RepoPath)
	if err != nil {
		return "", fmt.Errorf("problem while attempting to find head tag: %w", err)
	}
	if currentTag != "" {
		if taggedRelease, err := summer.Release(currentTag); err != nil {
			// TODO: assert the error specifically confirms that the release does not exist, not just any error
			// no release found, assume that this is the correct release info
			return "", fmt.Errorf("unable to fetch release=%q : %w", currentTag, err)
		} else if taggedRelease != nil {
			log.Debugf("found existing tag=%q however, it already has an associated release. ignoring...", currentTag)
			//return commitRef, nil
			return "", nil
		}

		log.Debugf("found existing tag=%q at HEAD which does not have an associated release", currentTag)

		// a tag was found and there is no existing release for this tag
		return currentTag, nil
	}

	// fallback to referencing the commit directly
	//return commitRef, nil
	return "", nil

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
