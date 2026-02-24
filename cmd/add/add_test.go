package add

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetAddCmd_Returns(t *testing.T) {
	cmd := GetAddCmd()

	require.NotNil(t, cmd)
}

func TestGetAddCmd_Singleton(t *testing.T) {
	cmd1 := GetAddCmd()
	cmd2 := GetAddCmd()

	assert.Same(t, cmd1, cmd2)
}

func TestGetAddCmd_Subcommands(t *testing.T) {
	cmd := GetAddCmd()
	subcommands := cmd.Commands()

	// Should have item and location subcommands
	require.Len(t, subcommands, 2)

	subcommandNames := make([]string, len(subcommands))
	for i, sub := range subcommands {
		subcommandNames[i] = sub.Name()
	}

	assert.Contains(t, subcommandNames, "item")
	assert.Contains(t, subcommandNames, "location")
}
