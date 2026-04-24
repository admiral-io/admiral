package server

import (
	"testing"

	goversion "github.com/caarlos0/go-version"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testVersionInfo() goversion.Info {
	return goversion.GetVersionInfo()
}

func TestNewRotateKeysCmd(t *testing.T) {
	opts := &globalOpts{}
	rc := newRotateKeysCmd(opts)

	require.NotNil(t, rc.cmd)
	assert.Equal(t, "rotate-keys", rc.cmd.Use)
	assert.False(t, rc.opts.status, "status flag defaults to false")
}

func TestNewRotateKeysCmd_StatusFlag(t *testing.T) {
	opts := &globalOpts{}
	rc := newRotateKeysCmd(opts)

	rc.cmd.SetArgs([]string{"--status"})
	// Parse flags only, don't execute (RunE requires config/db).
	err := rc.cmd.ParseFlags([]string{"--status"})
	require.NoError(t, err)
	assert.True(t, rc.opts.status)
}

func TestNewRotateKeysCmd_RegisteredOnRoot(t *testing.T) {
	root := newRootCmd(testVersionInfo(), func(int) {})

	var found bool
	for _, sub := range root.cmd.Commands() {
		if sub.Use == "rotate-keys" {
			found = true
			break
		}
	}
	assert.True(t, found, "rotate-keys should be registered as a subcommand")
}

func TestNewRotateKeysCmd_NoArgs(t *testing.T) {
	opts := &globalOpts{}
	rc := newRotateKeysCmd(opts)

	err := rc.cmd.Args(rc.cmd, []string{"extra"})
	assert.Error(t, err, "rotate-keys accepts no positional arguments")
}
