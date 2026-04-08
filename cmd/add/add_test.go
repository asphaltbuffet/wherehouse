package add

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewAddCmd_ReturnsNonNil(t *testing.T) {
	cmd := NewAddCmd()
	require.NotNil(t, cmd)
}
