package cmd

import (
	"fmt"
	"os"

	"github.com/anchore/chronicle/internal/git"

	"github.com/spf13/pflag"

	"github.com/anchore/chronicle/chronicle/presenter"
	"github.com/anchore/chronicle/chronicle/release"

	"github.com/anchore/chronicle/chronicle/release/summarizer/github"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/anchore/chronicle/internal/config"
)

var persistentOpts = config.CliOnlyOptions{}

var rootCmd = &cobra.Command{
	Use:   "",
	Short: "",
	Args:  cobra.NoArgs,
	RunE:  run,
}

func init() {
	if err := setGlobalFlags(rootCmd.PersistentFlags()); err != nil {
		panic(fmt.Sprintf("unable to set global flags: %+v", err))
	}

	if err := setRootFlags(rootCmd.Flags()); err != nil {
		panic(fmt.Sprintf("unable to set root cmd flags: %+v", err))
	}
}

func setGlobalFlags(flags *pflag.FlagSet) error {
	flags.StringVarP(&persistentOpts.ConfigPath, "config", "c", "", "application config file")

	flag := "quiet"
	flags.BoolP(
		flag, "q", false,
		"suppress all logging output",
	)
	if err := viper.BindPFlag(flag, flags.Lookup(flag)); err != nil {
		return err
	}

	flags.CountVarP(&persistentOpts.Verbosity, "verbose", "v", "increase verbosity (-v = info, -vv = debug)")

	return nil
}

func setRootFlags(flags *pflag.FlagSet) error {
	flag := "repo-path"
	flags.StringP(
		flag, "p", "./",
		"path to the repo to process",
	)
	if err := viper.BindPFlag(flag, flags.Lookup(flag)); err != nil {
		return err
	}

	flag = "since-tag"
	flags.StringP(
		flag, "s", "",
		"tag to start changelog processing from (inclusive)",
	)
	if err := viper.BindPFlag(flag, flags.Lookup(flag)); err != nil {
		return err
	}

	flag = "until-tag"
	flags.StringP(
		flag, "u", "",
		"tag to end changelog processing at (inclusive)",
	)
	if err := viper.BindPFlag(flag, flags.Lookup(flag)); err != nil {
		return err
	}
	return nil
}

func run(cmd *cobra.Command, args []string) error {
	summer, err := github.NewChangeSummarizer(appConfig.RepoPath)
	if err != nil {
		return fmt.Errorf("unable to create summarizer: %w", err)
	}

	var lastRelease *release.Release
	if appConfig.SinceTag != "" {
		lastRelease, err = summer.Release(appConfig.SinceTag)
		if err != nil {
			return fmt.Errorf("unable to fetch specific release: %w", err)
		}
	} else {
		lastRelease, err = summer.LastRelease()
		if err != nil {
			return fmt.Errorf("unable to determine last release: %w", err)
		}
	}

	changes, err := summer.Changes(lastRelease.Version, appConfig.UntilTag)
	if err != nil {
		return fmt.Errorf("unable to summarize changes: %w", err)
	}

	releaseVersion := appConfig.UntilTag
	if releaseVersion == "" {
		releaseVersion = "(unreleased)"
		// check if the current commit is tagged, then use that
		releaseTag, err := git.HeadTag(appConfig.RepoPath)
		if err != nil {
			return fmt.Errorf("problem while attempting to find head tag: %w", err)
		}
		if releaseTag != "" {
			releaseVersion = releaseTag
		}
	}

	pres, err := presenter.NewMarkdownPresenter(presenter.MarkdownConfig{
		Release: *lastRelease,
		Description: release.Description{
			VCSTagURL:     summer.TagURL(lastRelease.Version),
			VCSChangesURL: summer.ChangesURL(lastRelease.Version, appConfig.UntilTag),
			Changes:       changes,
			Notice:        "",
		},
		// TODO: make configurable
		Title: "Changelog",
	})
	if err != nil {
		return err
	}

	return pres.Present(os.Stdout)
}
