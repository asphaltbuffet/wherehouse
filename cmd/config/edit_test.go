package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetEditCmd_Singleton(t *testing.T) {
	cmd1 := GetEditCmd()
	require.NotNil(t, cmd1)

	cmd2 := GetEditCmd()

	assert.Same(t, cmd1, cmd2)
}
