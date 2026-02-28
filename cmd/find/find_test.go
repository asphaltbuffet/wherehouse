package find

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFindCmd_Singleton(t *testing.T) {
	cmd1 := GetFindCmd()
	require.NotNil(t, cmd1)

	cmd2 := GetFindCmd()

	assert.Same(t, cmd1, cmd2)
}
