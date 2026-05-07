package cli

import (
	"os"

	"github.com/anchore/chronicle/chronicle"
	"github.com/anchore/chronicle/cmd/chronicle/cli/commands"
	"github.com/anchore/chronicle/cmd/chronicle/internal/ui"
	"github.com/anchore/chronicle/internal/bus"
	"github.com/anchore/clio"
)

func New(id clio.Identification) clio.Application {
	clioCfg := clio.NewSetupConfig(id).
		WithGlobalConfigFlag().   // add persistent -c <path> for reading an application config from
		WithGlobalLoggingFlags(). // add persistent -v and -q flags tied to the logging config
		WithConfigInRootHelp().   // --help on the root command renders the full application config in the help text
		WithUIConstructor(
			// select a UI based on the logging configuration and state of stdin (if stdin is a tty)
			func(cfg clio.Config) (*clio.UICollection, error) {
				noUI := ui.None()
				if !cfg.Log.AllowUI(os.Stdin) {
					return clio.NewUICollection(noUI), nil
				}

				return clio.NewUICollection(
					ui.New(id.Version, "", false, cfg.Log.Quiet),
					noUI,
				), nil
			},
		).
		WithInitializers(
			func(state *clio.State) error {
				// clio is setting up and providing the bus, redact store, and logger to the application. Once loaded,
				// we can hoist them into the internal packages for global use.
				chronicle.SetBus(state.Bus)
				chronicle.SetLogger(state.Logger)
				bus.Set(state.Bus)
				return nil
			},
		)

	app := clio.New(*clioCfg)

	create := commands.Create(app)

	root := commands.Root(app, create)

	root.AddCommand(
		create,
		commands.NextVersion(app),
		clio.VersionCommand(id),
		clio.ConfigCommand(app, nil),
	)

	return app
}
