package server

import (
	"errors"
	"strings"

	goversion "github.com/caarlos0/go-version"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

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

func Execute(versionInfo goversion.Info, exitFunc func(int), args []string) {
	newRootCmd(versionInfo, exitFunc).Execute(args)
}

func newRootCmd(versionInfo goversion.Info, exit func(int)) *rootCmd {
	opts := &globalOpts{}
	root := &rootCmd{
		exit: exit,
		opts: opts,
	}

	cmd := &cobra.Command{
		Use:   "admiral-server",
		Short: "Run the Admiral server",
		Long: `admiral-server runs the API, web UI, and orchestration engine
used by Admiral SDKs and tooling.`,
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
		newMigrateCmd(opts).cmd,
		newStartCmd(opts).cmd,
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
