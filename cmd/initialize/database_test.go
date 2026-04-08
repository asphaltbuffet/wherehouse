package initialize

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetDatabaseCmd_Singleton(t *testing.T) {
	cmd := NewInitializeDatabaseCmd()
	require.NotNil(t, cmd)
}
