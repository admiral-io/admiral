package server

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"go.admiral.io/admiral/cmd/assets"
	"go.admiral.io/admiral/internal/config"
	"go.admiral.io/admiral/internal/gateway"
)

type startCmd struct {
	cmd *cobra.Command
}

func newStartCmd(globals *globalOpts) *startCmd {
	sc := &startCmd{}
	cmd := &cobra.Command{
		Use:     "start",
		Aliases: []string{"s", "serve", "run"},
		Short:   "Start the Admiral server",
		Long: `Start the Admiral server with the specified configuration.

The server exposes gRPC and HTTP APIs for managing applications, environments,
clusters, and infrastructure components. It connects to PostgreSQL, the
configured identity provider, and optional storage backends.`,
		Example: `  # Start with default configuration
  admiral-server start

  # Start with custom config and debug mode
  admiral-server start --config /path/to/config.yaml --debug

  # Start with additional environment files
  admiral-server start --env .env.local --env .env.production`,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			if globals.configFile == "" {
				return wrapErrorWithCode(
					errors.New("configuration file is required"),
					1,
					"missing configuration file",
				)
			}

			if _, err := os.Stat(globals.configFile); os.IsNotExist(err) {
				return wrapErrorWithCode(
					err,
					1,
					fmt.Sprintf("configuration file does not exist: %s", globals.configFile),
				)
			}

			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Build(globals.configFile, globals.envVarFiles, globals.debug)
			if err != nil {
				return err
			}
			return gateway.Run(cfg, gateway.CoreComponentFactory, assets.VirtualFS)
		},
	}

	sc.cmd = cmd
	return sc
}
