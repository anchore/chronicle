package cmd

import (
	"fmt"
	"os"
	"sort"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/chronicle/release/presenter"
	"github.com/anchore/chronicle/internal/git"
	"github.com/anchore/chronicle/internal/log"
	"github.com/scylladb/go-set/strset"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var createCmd = &cobra.Command{
	Use:   "create [PATH]",
	Short: "generate a changelog",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runCreate,
	PreRunE: func(cmd *cobra.Command, args []string) error {
		var repo = "./"
		if len(args) == 1 {
			if !git.IsRepository(args[0]) {
				return fmt.Errorf("given path is not a git repository: %s", args[0])
			}
			repo = args[0]
		}
		appConfig.CliOptions.RepoPath = repo
		return nil
	},
}

func init() {
	setCreateFlags(createCmd.Flags())
}

func setCreateFlags(flags *pflag.FlagSet) {
	flags.StringP(
		"output", "o", string(presenter.Default()),
		fmt.Sprintf("output format to use: %+v", presenter.All()),
	)

	flags.StringP(
		"since-tag", "s", "",
		"tag to start changelog processing from (inclusive)",
	)

	flags.StringP(
		"until-tag", "u", "",
		"tag to end changelog processing at (inclusive)",
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
	} {
		if err := viper.BindPFlag(flag, flags.Lookup(flag)); err != nil {
			return err
		}
	}
	return nil
}

func runCreate(cmd *cobra.Command, args []string) error {
	worker := selectWorker(appConfig.CliOptions.RepoPath)

	description, err := worker()
	if err != nil {
		return err
	}

	format := presenter.FromString(appConfig.Output)
	if format == nil {
		return fmt.Errorf("unable to parse output format: %q", appConfig.Output)
	}

	presenterTask, err := selectPresenter(*format)
	if err != nil {
		return err
	}

	p, err := presenterTask(*description)
	if err != nil {
		return err
	}

	return p.Present(os.Stdout)
}

func selectWorker(repo string) func() (*release.Description, error) {
	// TODO: we only support github, but this is the spot to add support for other providers such as GitLab or Bitbucket or other VCSs altogether, such as subversion.
	return createChangelogFromGithub
}

func logChanges(changes change.Changes) {
	log.Infof("discovered changes: %d", len(changes))

	set := strset.New()
	count := make(map[change.Type]int)
	for _, c := range changes {
		for _, ty := range c.ChangeTypes {
			_, exists := count[ty]
			if !exists {
				count[ty] = 0
			}
			count[ty]++
			set.Add(string(ty))
		}
	}

	types := set.List()
	sort.Strings(types)

	for idx, ty := range types {
		var branch = "├──"
		if idx == len(types)-1 {
			branch = "└──"
		}
		log.Debugf("  %s %s: %d", branch, ty, count[change.Type(ty)])
	}
}
