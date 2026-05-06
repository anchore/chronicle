package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/anchore/chronicle/cmd/chronicle/cli/options"
	"github.com/anchore/chronicle/internal/git"
	"github.com/anchore/chronicle/internal/log"
	"github.com/anchore/clio"
)

type nextVersion struct {
	RepoPath  string            `yaml:"repo-path" json:"repo-path" mapstructure:"-"`
	EnforceV0 options.EnforceV0 `yaml:"enforce-v0" json:"enforce-v0" mapstructure:"enforce-v0"`
}

var _ clio.FieldDescriber = (*nextVersion)(nil)

func (c *nextVersion) DescribeFields(descriptions clio.FieldDescriptionSet) {
	descriptions.Add(&c.EnforceV0, "major changes bump minor version for versions < 1.0")
}

func NextVersion(app clio.Application) *cobra.Command {
	cfg := &nextVersion{}

	return app.SetupCommand(&cobra.Command{
		Use:   "next-version [PATH]",
		Short: "Guess the next version based on the changelog diff from the last release",
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
				return err
			}

			var repo = "./"
			if len(args) == 1 {
				if !git.IsRepository(args[0]) {
					return fmt.Errorf("given path is not a git repository: %s", args[0])
				}
				repo = args[0]
			} else {
				log.Infof("no repository path given, assuming %q", repo)
			}
			cfg.RepoPath = repo
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			// ensure errors are printed to stderr since most output is redirected to CHANGELOG.md more often than not
			cmd.SetErr(os.Stderr)
			return runNextVersion(cfg)
		},
	}, cfg)
}

func runNextVersion(cfg *nextVersion) error {
	appConfig := &createConfig{
		EnforceV0: cfg.EnforceV0,
		RepoPath:  cfg.RepoPath,
	}
	appConfig.SpeculateNextVersion = true
	worker := selectWorker(cfg.RepoPath)

	_, description, err := worker(appConfig)
	if err != nil {
		return err
	}

	_, err = os.Stdout.Write([]byte(description.Version))

	return err
}
