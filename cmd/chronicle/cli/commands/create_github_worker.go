package commands

import (
	"fmt"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/chronicle/release/releasers/github"
	"github.com/anchore/chronicle/internal/git"
	"github.com/anchore/chronicle/internal/log"
)

func createChangelogFromGithub(appConfig *createConfig) (*release.Release, *release.Description, error) {
	// pre-flight: the trunk format encodes commit-level data that is only
	// populated when consider-pr-merge-commits is enabled. Fail fast before
	// any GitHub API work if the combination is invalid.
	if err := checkTrunkPrerequisites(appConfig); err != nil {
		return nil, nil, err
	}

	ghConfig := appConfig.Github.ToGithubConfig()

	gitter, err := git.New(appConfig.RepoPath)
	if err != nil {
		return nil, nil, err
	}

	summer, err := github.NewSummarizer(gitter, ghConfig)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to create summarizer: %w", err)
	}

	changeTypeTitles := getGithubSupportedChanges(appConfig)

	var untilTag = appConfig.UntilTag
	if untilTag == "" {
		untilTag, err = github.FindChangelogEndTag(summer, gitter)
		if err != nil {
			return nil, nil, err
		}
	}

	if untilTag != "" {
		log.WithFields("tag", untilTag).Info("until the given tag")
	} else {
		log.Info("until the current revision (no end tag)")
	}

	var speculator release.VersionSpeculator
	if appConfig.SpeculateNextVersion {
		speculator = github.NewVersionSpeculator(gitter, release.SpeculationBehavior{
			EnforceV0:           bool(appConfig.EnforceV0),
			NoChangesBumpsPatch: true,
		})
	}

	changelogConfig := release.ChangelogInfoConfig{
		RepoPath:          appConfig.RepoPath,
		SinceTag:          appConfig.SinceTag,
		UntilTag:          untilTag,
		VersionSpeculator: speculator,
		ChangeTypeTitles:  changeTypeTitles,
	}

	return release.ChangelogInfo(summer, changelogConfig)
}

// checkTrunkPrerequisites returns an error when the trunk output format is
// selected but consider-pr-merge-commits is disabled. The trunk encoder
// requires commit-level data that is only collected when that setting is on.
func checkTrunkPrerequisites(appConfig *createConfig) error {
	specs, err := appConfig.Specs()
	if err != nil {
		return err
	}

	for _, spec := range specs {
		if spec.Name == "trunk" && !appConfig.Github.ConsiderPRMergeCommits {
			return fmt.Errorf(`the "trunk" output format requires "consider-pr-merge-commits=true"; either enable it (or pass --consider-pr-merge-commits) or remove "-o trunk"`)
		}
	}
	return nil
}

func getGithubSupportedChanges(appConfig *createConfig) []change.TypeTitle {
	var supportedChanges []change.TypeTitle
	for _, c := range appConfig.Github.Changes {
		// TODO: this could be one source of truth upstream
		k := change.ParseSemVerKind(c.SemVerKind)
		t := change.NewType(c.Type, k)
		supportedChanges = append(supportedChanges, change.TypeTitle{
			ChangeType: t,
			Title:      c.Title,
		})
	}
	return supportedChanges
}
