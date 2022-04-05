package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"

	"github.com/anchore/chronicle/internal/git"
	"github.com/anchore/chronicle/internal/log"
)

var nextVersionCmd = &cobra.Command{
	Use:   "next-version [PATH]",
	Short: "Guess the next version based on the changelog diff from the last release",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runNextVersion,
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
	setNextVersionFlags(nextVersionCmd.Flags())
	if err := bindNextVersionConfigOptions(nextVersionCmd.Flags()); err != nil {
		panic(err)
	}

	rootCmd.AddCommand(nextVersionCmd)
}

func setNextVersionFlags(flags *pflag.FlagSet) {
	flags.BoolP(
		"enforce-v0", "e", false,
		"major changes bump the minor version field for versions < 1.0",
	)
}

func bindNextVersionConfigOptions(flags *pflag.FlagSet) error {
	if err := viper.BindPFlag("enforce-v0", flags.Lookup("enforce-v0")); err != nil {
		return err
	}
	return nil
}

func runNextVersion(cmd *cobra.Command, args []string) error {
	appConfig.SpeculateNextVersion = true
	worker := selectWorker(appConfig.CliOptions.RepoPath)

	_, description, err := worker()
	if err != nil {
		return err
	}

	_, err = os.Stdout.Write([]byte(description.Release.Version))

	return err
}
