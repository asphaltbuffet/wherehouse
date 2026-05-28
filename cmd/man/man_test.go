package man_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/asphaltbuffet/wherehouse/cmd/man"
)

func TestNewManCmd_Name(t *testing.T) {
	cmd := man.NewManCmd()

	assert.Equal(t, "man", cmd.Name())
}

func TestNewManCmd_IsHidden(t *testing.T) {
	cmd := man.NewManCmd()

	assert.True(t, cmd.Hidden)
}

func TestNewManCmd_NoArgs(t *testing.T) {
	cmd := man.NewManCmd()

	err := cmd.Args(cmd, []string{"unexpected"})
	assert.Error(t, err)
}

func TestNewManCmd_WritesNonEmptyOutput(t *testing.T) {
	cmd := man.NewManCmd()

	var buf bytes.Buffer
	cmd.SetOut(&buf)

	err := cmd.RunE(cmd, nil)
	require.NoError(t, err)
	assert.NotEmpty(t, buf.String())
}
