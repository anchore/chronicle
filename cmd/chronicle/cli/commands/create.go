package commands

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/anchore/chronicle/chronicle/dependency"
	"github.com/anchore/chronicle/chronicle/event"
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
			// cmd.Context() is the clio-provided context cancelled on SIGINT, so
			// threading it lets a Ctrl-C interrupt the (network-bound) dependency
			// scan rather than detaching it on context.Background().
			return runCreate(cmd.Context(), appConfig)
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

func runCreate(ctx context.Context, appConfig *createConfig) error {
	// fast-fail on misconfigured -o values before the worker hits the
	// network; sink construction (which creates temp files) is still deferred
	// until after the worker succeeds so failures don't leak temp files.
	if err := appConfig.Check(); err != nil {
		return err
	}

	// vulnerability annotation operates on the dependency diff, so it has
	// nothing to act on without an ecosystem (syft cataloger selector) to scan.
	if appConfig.Dependencies.AnnotateVulnerabilities && !appConfig.Dependencies.Enabled() {
		return errors.New("--vulnerabilities requires at least one dependency ecosystem to scan; set --dependencies (e.g. --dependencies language)")
	}

	// fail loudly on a misspelled min-severity rather than silently treating it
	// as "no filter" (which is how the annotator interprets an unknown value).
	if !dependency.ValidSeverity(appConfig.Dependencies.MinSeverity) {
		return fmt.Errorf("invalid dependencies.min-severity %q; valid values: negligible, low, medium, high, critical", appConfig.Dependencies.MinSeverity)
	}

	startRelease, description, err := selectWorker(appConfig.RepoPath)(ctx, appConfig)
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

	// publish the raw figures for the post-teardown recap block. The UI renders
	// it; NextVersion empty means speculation was off and the UI omits the
	// version-transition line.
	bus.PublishSummary(summaryEvent(startRelease, description, appConfig.SpeculateNextVersion))

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

// summaryEvent flattens the resolved release into the raw figures the UI needs
// for the recap block: the repo identity, a per-change-type tally, and the
// version transition. NextVersion is only set when speculation produced a
// version distinct from the previous release; that gate ensures the UI omits
// the version-transition line in modes where it would be misleading.
func summaryEvent(startRelease *release.Release, desc *release.Description, speculate bool) event.Summary {
	s := event.Summary{Repo: bus.Repo()}
	if startRelease != nil {
		s.PreviousVersion = startRelease.Version
	}
	if desc != nil {
		// emit every supported change type with its raw count (including zeros);
		// the UI decides which tiers to show and how.
		for _, tt := range desc.SupportedChanges {
			s.Changes = append(s.Changes, event.SummaryChange{
				Name:  tt.ChangeType.Name,
				Kind:  tt.ChangeType.Kind,
				Count: len(desc.Changes.ByChangeType(tt.ChangeType)),
			})
		}
	}
	if speculate && desc != nil && desc.Speculated {
		s.NextVersion = desc.Version
		s.BumpKind = change.Significance(desc.Changes)
	}
	return s
}

//nolint:revive
func selectWorker(repo string) func(context.Context, *createConfig) (*release.Release, *release.Description, error) {
	// TODO: we only support github, but this is the spot to add support for other providers such as GitLab or Bitbucket or other VCSs altogether, such as subversion.
	return createChangelogFromGithub
}
