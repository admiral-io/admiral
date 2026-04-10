package main

import (
	_ "embed"
	"os"

	goversion "github.com/caarlos0/go-version"

	"go.admiral.io/admiral/cmd/server"
)

//go:embed art.txt
var asciiArt string

//nolint:gochecknoglobals
var (
	version   = ""
	commit    = ""
	treeState = ""
	date      = ""
	builtBy   = ""

	website = "https://admiral.io"
)

func main() {
	server.Execute(
		buildInfo(version, commit, date, builtBy, treeState),
		os.Exit,
		os.Args[1:],
	)
}

func buildInfo(version, commit, date, builtBy, treeState string) goversion.Info {
	return goversion.GetVersionInfo(
		goversion.WithAppDetails("admiral-server", "build faster, deploy smarter.", website),
		goversion.WithASCIIName(asciiArt),
		func(i *goversion.Info) {
			if commit != "" {
				i.GitCommit = commit
			}
			if treeState != "" {
				i.GitTreeState = treeState
			}
			if date != "" {
				i.BuildDate = date
			}
			if version != "" {
				i.GitVersion = version
			}
			if builtBy != "" {
				i.BuiltBy = builtBy
			}
		},
	)
}
