package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/internal/git"
	"github.com/anchore/chronicle/internal/log"
	"github.com/coreos/go-semver/semver"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
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
	worker := selectWorker(appConfig.CliOptions.RepoPath)

	release, description, err := worker()
	if err != nil {
		return err
	}

	var breaking, feature, patch bool
	for _, c := range description.Changes {
		for _, chTy := range c.ChangeTypes {
			switch chTy.Kind {
			case change.SemVerMajor:
				if appConfig.EnforceV0 {
					feature = true
				} else {
					breaking = true
				}
			case change.SemVerMinor:
				feature = true
			case change.SemVerPatch:
				patch = true
			}
		}
	}

	v := semver.New(strings.TrimLeft(release.Version, "v"))
	original := *v

	if patch {
		v.BumpPatch()
	}

	if feature {
		v.BumpMinor()
	}

	if breaking {
		v.BumpMajor()
	}

	if v.String() == original.String() {
		return fmt.Errorf("no changes found that affect the version (changes=%d)", len(description.Changes))
	}

	prefix := ""
	if strings.HasPrefix("v", release.Version) {
		prefix = "v"
	}

	_, err = os.Stdout.Write([]byte(prefix + v.String()))

	return err
}
