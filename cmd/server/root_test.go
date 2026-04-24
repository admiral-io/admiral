package server

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewRootCmd(t *testing.T) {
	root := newRootCmd(testVersionInfo(), func(int) {})

	require.NotNil(t, root.cmd)
	assert.Equal(t, "admiral-server", root.cmd.Use)
}

func TestNewRootCmd_Subcommands(t *testing.T) {
	root := newRootCmd(testVersionInfo(), func(int) {})

	names := make(map[string]bool)
	for _, sub := range root.cmd.Commands() {
		names[sub.Use] = true
	}

	assert.True(t, names["start"], "start subcommand should be registered")
	assert.True(t, names["migrate"], "migrate subcommand should be registered")
	assert.True(t, names["rotate-keys"], "rotate-keys subcommand should be registered")
}

func TestNewRootCmd_PersistentFlags(t *testing.T) {
	root := newRootCmd(testVersionInfo(), func(int) {})

	configFlag := root.cmd.PersistentFlags().Lookup("config")
	require.NotNil(t, configFlag)
	assert.Equal(t, "config.yaml", configFlag.DefValue)
	assert.Equal(t, "c", configFlag.Shorthand)

	debugFlag := root.cmd.PersistentFlags().Lookup("debug")
	require.NotNil(t, debugFlag)
	assert.Equal(t, "false", debugFlag.DefValue)

	envFlag := root.cmd.PersistentFlags().Lookup("env")
	require.NotNil(t, envFlag)
}

func TestEnvFiles(t *testing.T) {
	var ef envFiles

	assert.Equal(t, "", ef.String())
	assert.Equal(t, "envFiles", ef.Type())

	require.NoError(t, ef.Set(".env.dev"))
	require.NoError(t, ef.Set(".env.local"))
	assert.Equal(t, ".env.dev,.env.local", ef.String())
	assert.Len(t, ef, 2)
}

func TestExitError(t *testing.T) {
	inner := assert.AnError
	ee := wrapErrorWithCode(inner, 42, "something broke")

	assert.Equal(t, inner.Error(), ee.Error())
	assert.Equal(t, 42, ee.code)
	assert.Equal(t, "something broke", ee.details)
}
