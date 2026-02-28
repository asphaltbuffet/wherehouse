package history

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetHistoryCmd_Singleton(t *testing.T) {
	cmd1 := GetHistoryCmd()
	require.NotNil(t, cmd1)

	cmd2 := GetHistoryCmd()

	assert.Same(t, cmd1, cmd2)
}
