package commands

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/internal/git"
	"github.com/anchore/chronicle/internal/log"
	"github.com/anchore/clio"
)

func Create(app clio.Application) *cobra.Command {
	appConfig := defaultCreateConfig()

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
		Args: repoPathArgs(appConfig),
		RunE: func(cmd *cobra.Command, _ []string) error {
			// ensure errors are printed to stderr since most output is redirected to CHANGELOG.md more often than not
			cmd.SetErr(os.Stderr)
			return runCreate(appConfig)
		},
	}, appConfig)
}

// repoPathArgs returns a cobra Args validator that resolves an optional [PATH] argument and writes
// it onto the given config. It must be a factory (rather than a method on createConfig or a free
// function that takes config from a closure of a different command) so that root and create each
// pin the validator to their own config — sharing one validator across both commands previously
// caused root invocations to leave RepoPath empty.
func repoPathArgs(cfg *createConfig) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if err := cobra.MaximumNArgs(1)(cmd, args); err != nil {
			_ = cmd.Help()
			return err
		}
		var repo = "./"
		if len(args) == 1 {
			repo = args[0]
		} else {
			log.Infof("no repository path given, assuming %q", repo)
		}
		// validate now (rather than deferring to git.New deep in the worker) so the user gets a
		// clear, early failure with the underlying go-git reason, including for the implicit "./".
		if _, err := git.New(repo); err != nil {
			return err
		}
		cfg.RepoPath = repo
		return nil
	}
}

func runCreate(appConfig *createConfig) error {
	// run the worker before constructing the writer so that file sinks are
	// only opened (and temp files created) once we know the description is
	// available; this avoids leaving orphaned temp files behind on failure.
	_, description, err := selectWorker(appConfig.RepoPath)(appConfig)
	if err != nil {
		return err
	}

	w, err := appConfig.Writer()
	if err != nil {
		return err
	}

	// don't `defer w.Close()` — Close performs the atomic rename for file
	// sinks, and a deferred call would swallow that error.
	if err := w.Write(appConfig.Title, *description); err != nil {
		_ = w.Close()
		return err
	}
	return w.Close()
}

//nolint:revive
func selectWorker(repo string) func(*createConfig) (*release.Release, *release.Description, error) {
	// TODO: we only support github, but this is the spot to add support for other providers such as GitLab or Bitbucket or other VCSs altogether, such as subversion.
	return createChangelogFromGithub
}
