package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConfigCheck(t *testing.T) {
	cmd := NewConfigCmd()
	require.NotNil(t, cmd)
}
