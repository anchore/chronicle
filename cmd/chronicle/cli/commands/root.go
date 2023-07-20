package commands

import (
	"github.com/spf13/cobra"

	"github.com/anchore/chronicle/cmd/chronicle/cli/options"
	"github.com/anchore/clio"
)

func Root(app clio.Application, createCmd *cobra.Command) *cobra.Command {
	appConfig := options.NewCreate()

	cmd := app.SetupRootCommand(&cobra.Command{
		Short: createCmd.Short,
		Long:  createCmd.Long,
		Args:  createCmd.Args,
		RunE: func(_ *cobra.Command, _ []string) error {
			return runCreate(appConfig)
		},
	}, appConfig)

	return cmd
}
