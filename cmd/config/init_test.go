package config

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetInitCmd_Singleton(t *testing.T) {
	cmd := NewInitCmd()
	require.NotNil(t, cmd)
}
