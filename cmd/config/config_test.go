package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetConfigCmd_Singleton(t *testing.T) {
	cmd := NewConfigCmd()
	require.NotNil(t, cmd)
}
