package found

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewFoundCmd_ReturnsNonNil(t *testing.T) {
	cmd := NewFoundCmd()
	require.NotNil(t, cmd)
}
