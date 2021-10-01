package cmd

import (
	"fmt"

	"github.com/anchore/chronicle/internal/config"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

var persistentOpts = config.CliOnlyOptions{}

var rootCmd = &cobra.Command{
	Short:             createCmd.Short,
	Long:              createCmd.Long,
	Args:              createCmd.Args,
	Example:           createCmd.Example,
	SilenceUsage:      true,
	SilenceErrors:     true,
	PreRunE:           createCmd.PreRunE,
	RunE:              createCmd.RunE,
	ValidArgsFunction: createCmd.ValidArgsFunction,
}

func init() {
	if err := setGlobalFlags(rootCmd.PersistentFlags()); err != nil {
		panic(fmt.Sprintf("unable to set global flags: %+v", err))
	}

	setCreateFlags(rootCmd.Flags())
}

func setGlobalFlags(flags *pflag.FlagSet) error {
	flags.StringVarP(&persistentOpts.ConfigPath, "config", "c", "", "application config file")

	flag := "quiet"
	flags.BoolP(
		flag, "q", false,
		"suppress all logging output",
	)
	if err := viper.BindPFlag(flag, flags.Lookup(flag)); err != nil {
		return err
	}

	flags.CountVarP(&persistentOpts.Verbosity, "verbose", "v", "increase verbosity (-v = info, -vv = debug)")

	return nil
}
