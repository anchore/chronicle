package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/format"
	"github.com/anchore/chronicle/cmd/chronicle/cli/options"
	"github.com/anchore/chronicle/internal/git"
	"github.com/anchore/chronicle/internal/log"
	"github.com/anchore/clio"
)

func Create(app clio.Application) *cobra.Command {
	appConfig := options.NewCreate()

	return app.SetupCommand(&cobra.Command{
		Use:   "create [PATH]",
		Short: "Generate a changelog from GitHub issues and PRs",
		Long: `Generate a changelog from GitHub issues and PRs.

chronicle [flags] [PATH]

Create a changelog representing the changes from tag v0.14.0 until the present (for ./)
	chronicle --since-tag v0.14.0

Create a changelog representing the changes from tag v0.14.0 until v0.18.0 (for ../path/to/repo)
	chronicle --since-tag v0.14.0 --until-tag v0.18.0 ../path/to/repo

`,
		Args: func(cmd *cobra.Command, args []string) error {
			if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
				_ = cmd.Help()
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
			appConfig.RepoPath = repo
			return nil
		},
		RunE: func(_ *cobra.Command, _ []string) error {
			return runCreate(appConfig)
		},
	}, appConfig)
}

func runCreate(appConfig *options.Create) error {
	worker := selectWorker(appConfig.RepoPath)

	_, description, err := worker(appConfig)
	if err != nil {
		return err
	}

	if appConfig.VersionFile != "" {
		f, err := os.OpenFile(appConfig.VersionFile, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0644)
		if err != nil {
			return fmt.Errorf("unable to open version file %q: %w", appConfig.VersionFile, err)
		}
		if _, err := f.WriteString(description.Version); err != nil {
			return fmt.Errorf("unable to write version to file %q: %w", appConfig.VersionFile, err)
		}
	}

	f := format.FromString(appConfig.Output)
	if f == nil {
		return fmt.Errorf("unable to parse output format: %q", appConfig.Output)
	}

	presenterTask, err := selectPresenter(*f)
	if err != nil {
		return err
	}

	p, err := presenterTask(appConfig.Title, *description)
	if err != nil {
		return err
	}

	return p.Present(os.Stdout)
}

func selectWorker(repo string) func(*options.Create) (*release.Release, *release.Description, error) {
	// TODO: we only support github, but this is the spot to add support for other providers such as GitLab or Bitbucket or other VCSs altogether, such as subversion.
	return createChangelogFromGithub
}
