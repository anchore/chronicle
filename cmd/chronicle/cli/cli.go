package cli

import (
	"github.com/anchore/chronicle/chronicle"
	"github.com/anchore/chronicle/cmd/chronicle/cli/commands"
	"github.com/anchore/clio"
)

func New(id clio.Identification) clio.Application {
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
	root.AddCommand(clio.VersionCommand(id))

	return app
}
