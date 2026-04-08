package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetPathCmd_Singleton(t *testing.T) {
	cmd := NewPathCmd()
	require.NotNil(t, cmd)
}
