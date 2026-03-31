package server

import (
	"errors"
	"strings"

	goversion "github.com/caarlos0/go-version"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

// sharedOpts is set during command construction and read by subcommands.
var sharedOpts *globalOpts

type rootCmd struct {
	cmd  *cobra.Command
	exit func(int)
	opts *globalOpts
}

type globalOpts struct {
	configFile  string
	envVarFiles envFiles
	debug       bool
}

type envFiles []string

func (f *envFiles) String() string {
	return strings.Join(*f, ",")
}

func (f *envFiles) Set(value string) error {
	*f = append(*f, value)
	return nil
}

func (f *envFiles) Type() string {
	return "envFiles"
}

// Execute initializes and runs the root command.
func Execute(versionInfo goversion.Info, exitFunc func(int), args []string) {
	newRootCmd(versionInfo, exitFunc).Execute(args)
}

func newRootCmd(versionInfo goversion.Info, exit func(int)) *rootCmd {
	opts := &globalOpts{}
	sharedOpts = opts

	root := &rootCmd{
		exit: exit,
		opts: opts,
	}

	cmd := &cobra.Command{
		Use:   "admiral",
		Short: "Orchestrate application deployments across infrastructure",
		Long: `Admiral is a platform orchestrator that helps developers build, deploy,
and manage their applications from development to production.

It provides self-service infrastructure provisioning, hierarchical variable
management, and Kubernetes deployment coordination through a unified interface.`,
		Version:           versionInfo.String(),
		SilenceUsage:      true,
		SilenceErrors:     true,
		Args:              cobra.NoArgs,
		ValidArgsFunction: cobra.NoFileCompletions,
	}
	cmd.SetVersionTemplate("{{.Version}}")
	cmd.CompletionOptions.DisableDefaultCmd = true

	cmd.PersistentFlags().StringVarP(&opts.configFile, "config", "c", "config.yaml", "load configuration from file")
	_ = cmd.MarkFlagFilename("config", "yaml", "yml")
	cmd.PersistentFlags().BoolVar(&opts.debug, "debug", false, "print the final configuration file to stdout")
	cmd.PersistentFlags().Var(&opts.envVarFiles, "env", "path to additional .env files to load")

	cmd.AddCommand(
		newMigrateCmd().Cmd,
		//newStartCmd().Cmd,
	)
	root.cmd = cmd
	return root
}

func (cmd *rootCmd) Execute(args []string) {
	logger, err := zap.NewProduction()
	if err != nil {
		panic("failed to initialize logger: " + err.Error())
	}
	defer func() { _ = logger.Sync() }()

	cmd.cmd.SetArgs(args)

	if err := cmd.cmd.Execute(); err != nil {
		code := 1
		msg := "command failed"

		if eerr, ok := errors.AsType[*exitError](err); ok {
			code = eerr.code
			if eerr.details != "" {
				msg = eerr.details
			}
		}
		logger.Error(msg, zap.Error(err))
		cmd.exit(code)
	}
}
