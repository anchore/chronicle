package commands

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/anchore/clio"
)

func Root(app clio.Application, createCmd *cobra.Command) *cobra.Command {
	appConfig := defaultCreateConfig()

	cmd := app.SetupRootCommand(&cobra.Command{
		Short: createCmd.Short,
		Long:  createCmd.Long,
		// pin the Args validator to root's own config — using createCmd.Args here would close over
		// create's config and leave root's RepoPath empty.
		Args: repoPathArgs(appConfig),
		RunE: func(cmd *cobra.Command, _ []string) error {
			// ensure errors are printed to stderr since most output is redirected to CHANGELOG.md more often than not
			cmd.SetErr(os.Stderr)
			return runCreate(appConfig)
		},
	}, appConfig)

	return cmd
}
