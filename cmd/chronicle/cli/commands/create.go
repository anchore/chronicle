package commands

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/anchore/chronicle/chronicle/release"
	"github.com/anchore/chronicle/chronicle/release/change"
	"github.com/anchore/chronicle/internal/bus"
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
	// fast-fail on misconfigured -o values before the worker hits the
	// network; sink construction (which creates temp files) is still deferred
	// until after the worker succeeds so failures don't leak temp files.
	if err := appConfig.Check(); err != nil {
		return err
	}

	startRelease, description, err := selectWorker(appConfig.RepoPath)(appConfig)
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
	if err := w.Close(); err != nil {
		return err
	}

	// notify for any non-stdout file sinks the user requested. Specs() is
	// re-parsed (rather than tracked through the writer) because the writer
	// abstraction does not surface its specs and we don't want to break that
	// boundary just for a status line.
	notifyFileSinks(appConfig, description)

	// emit the post-teardown summary block. PreviousVersion / NextVersion are
	// derived from what the worker resolved; ReportSummary skips the version
	// transition line when NextVersion is empty (speculation off).
	bus.ReportSummary(summaryOpts(startRelease, description, appConfig.SpeculateNextVersion))

	return nil
}

// notifyFileSinks emits a Notify per non-stdout output sink. The version
// encoder gets a tailored phrasing ("wrote version ..."); other formats use a
// generic "wrote <name> ..." phrasing.
func notifyFileSinks(appConfig *createConfig, description *release.Description) {
	if description == nil {
		return
	}
	specs, err := appConfig.Specs()
	if err != nil {
		return
	}
	for _, s := range specs {
		if s.IsStdout() {
			continue
		}
		if s.Name == "version" {
			bus.Notify(fmt.Sprintf("wrote version %q to %s", description.Version, s.Path))
		} else {
			bus.Notify(fmt.Sprintf("wrote %s to %s", s.Name, s.Path))
		}
	}
}

// summaryOpts builds the SummaryOpts for the final report. NextVersion is
// only set when speculation produced a version distinct from the previous
// release; that gate ensures the version transition line is omitted in
// modes where it would be misleading.
func summaryOpts(startRelease *release.Release, desc *release.Description, speculate bool) bus.SummaryOpts {
	opts := bus.SummaryOpts{Description: desc}
	if startRelease != nil {
		opts.PreviousVersion = startRelease.Version
	}
	if speculate && desc != nil && desc.Speculated {
		opts.NextVersion = desc.Version
		opts.BumpKind = change.Significance(desc.Changes)
	}
	return opts
}

//nolint:revive
func selectWorker(repo string) func(*createConfig) (*release.Release, *release.Description, error) {
	// TODO: we only support github, but this is the spot to add support for other providers such as GitLab or Bitbucket or other VCSs altogether, such as subversion.
	return createChangelogFromGithub
}
