package cmd

import (
	"fmt"
	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/format"
	"github.com/anchore/chronicle/internal/git"
	"github.com/anchore/chronicle/internal/log"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"os"
)

var createCmd = &cobra.Command{
	Use:   "create [PATH]",
	Short: "Generate a changelog from GitHub issues and PRs",
	Long: `Generate a changelog from GitHub issues and PRs.

chronicle [flags] [PATH]

Create a changelog representing the changes from tag v0.14.0 until the present (for ./)
	chronicle --since-tag v0.14.0

Create a changelog representing the changes from tag v0.14.0 until v0.18.0 (for ../path/to/repo)
	chronicle --since-tag v0.14.0 --until-tag v0.18.0 ../path/to/repo

`,
	Args: cobra.MaximumNArgs(1),
	RunE: runCreate,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		var repo = "./"
		if len(args) == 1 {
			if !git.IsRepository(args[0]) {
				return fmt.Errorf("given path is not a git repository: %s", args[0])
			}
			repo = args[0]
		} else {
			log.Infof("no repository path given, assuming %q", repo)
		}
		appConfig.CliOptions.RepoPath = repo
		return nil
	},
}

func init() {
	setCreateFlags(createCmd.Flags())

	rootCmd.AddCommand(createCmd)
}

func setCreateFlags(flags *pflag.FlagSet) {
	flags.StringP(
		"output", "o", string(format.Default()),
		fmt.Sprintf("output format to use: %+v", format.All()),
	)

	flags.StringP(
		"since-tag", "s", "",
		"tag to start changelog processing from (inclusive)",
	)

	flags.StringP(
		"until-tag", "u", "",
		"tag to end changelog processing at (inclusive)",
	)

	flags.BoolP(
		"speculate-next-version", "n", false,
		"guess the next release version based off of issues and PRs in cases where there is no semver tag after --since-tag (cannot use with --until-tag)",
	)

	flags.StringP(
		"title", "t", "Changelog",
		"The title of the changelog output",
	)
}

func bindCreateConfigOptions(flags *pflag.FlagSet) error {
	for _, flag := range []string{
		"output",
		"since-tag",
		"until-tag",
		"title",
		"speculate-next-version",
	} {
		if err := viper.BindPFlag(flag, flags.Lookup(flag)); err != nil {
			return err
		}
	}
	return nil
}

func runCreate(cmd *cobra.Command, args []string) error {
	worker := selectWorker(appConfig.CliOptions.RepoPath)

	_, description, err := worker()
	if err != nil {
		return err
	}

	f := format.FromString(appConfig.Output)
	if f == nil {
		return fmt.Errorf("unable to parse output format: %q", appConfig.Output)
	}

	presenterTask, err := selectPresenter(*f)
	if err != nil {
		return err
	}

	p, err := presenterTask(*description)
	if err != nil {
		return err
	}

	return p.Present(os.Stdout)
}

func selectWorker(repo string) func() (*release.Release, *release.Description, error) {
	// TODO: we only support github, but this is the spot to add support for other providers such as GitLab or Bitbucket or other VCSs altogether, such as subversion.
	return createChangelogFromGithub
}
