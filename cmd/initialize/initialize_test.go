package initialize

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewInitializeCmd_Singleton(t *testing.T) {
	cmd := NewInitializeCmd()
	require.NotNil(t, cmd)
}
