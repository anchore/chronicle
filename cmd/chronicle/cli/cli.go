package cli

import (
	"github.com/spf13/cobra"

	"github.com/anchore/chronicle/chronicle"
	"github.com/anchore/chronicle/cmd/chronicle/cli/commands"
	"github.com/anchore/chronicle/internal"
	"github.com/anchore/chronicle/internal/version"
	"github.com/anchore/clio"
)

func New() *cobra.Command {
	versionInfo := version.FromBuild()
	id := clio.Identification{
		Name:           internal.ApplicationName,
		Version:        versionInfo.Version,
		GitCommit:      versionInfo.GitCommit,
		GitDescription: versionInfo.GitTreeState,
		BuildDate:      versionInfo.BuildDate,
	}

	clioCfg := clio.NewSetupConfig(id).
		WithGlobalConfigFlag().   // add persistent -c <path> for reading an application config from
		WithGlobalLoggingFlags(). // add persistent -v and -q flags tied to the logging config
		WithConfigInRootHelp().   // --help on the root command renders the full application config in the help text
		WithNoBus().
		WithInitializers(
			func(state *clio.State) error {
				// clio is setting up and providing the bus, redact store, and logger to the application. Once loaded,
				// we can hoist them into the internal packages for global use.
				chronicle.SetBus(state.Bus)
				chronicle.SetLogger(state.Logger)
				return nil
			},
		)

	app := clio.New(*clioCfg)

	create := commands.Create(app)

	root := commands.Root(app, create)

	root.AddCommand(create)
	root.AddCommand(commands.NextVersion(app))
	root.AddCommand(commands.Version())

	return root
}
