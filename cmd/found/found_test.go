package found

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFoundCmd_Singleton(t *testing.T) {
	cmd1 := GetFoundCmd()
	require.NotNil(t, cmd1)

	cmd2 := GetFoundCmd()

	assert.Same(t, cmd1, cmd2)
}
