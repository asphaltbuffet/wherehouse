package add

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestGetItemCmd(t *testing.T) {
	cmd := NewAddItemCmd()
	require.NotNil(t, cmd)
}
