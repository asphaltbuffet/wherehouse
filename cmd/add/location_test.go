package add

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewAddLocationCmd(t *testing.T) {
	cmd1 := NewAddLocationCmd()
	require.NotNil(t, cmd1)
}
