package initialize

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetDatabaseCmd_Singleton(t *testing.T) {
	cmd1 := GetDatabaseCmd()
	require.NotNil(t, cmd1)

	cmd2 := GetDatabaseCmd()

	assert.Same(t, cmd1, cmd2)
}
