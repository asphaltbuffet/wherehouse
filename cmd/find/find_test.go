package find

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewFindCmd_ReturnsNonNil(t *testing.T) {
	cmd := NewFindCmd()
	require.NotNil(t, cmd)
}
