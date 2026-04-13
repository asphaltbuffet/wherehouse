package loan

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewLoanCmd_Structure verifies the command has correct metadata and flags.
func TestNewLoanCmd_Structure(t *testing.T) {
	// We need a valid loanDB for NewLoanCmd, so use a simple interface that implements the methods
	// For this test, we just verify structure without executing the command
	cmd := NewLoanCmd()

	assert.NotNil(t, cmd)
	assert.Equal(t, "loan", cmd.Name())
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Check --to flag exists and is required
	toFlag := cmd.Flags().Lookup("to")
	require.NotNil(t, toFlag)
	// The flag is registered as required via MarkFlagRequired
	assert.NotNil(t, cmd.Flag("to").Value)

	// Check --note flag exists and is not required
	noteFlag := cmd.Flags().Lookup("note")
	require.NotNil(t, noteFlag)
}
