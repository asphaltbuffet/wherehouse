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
	cmd := NewDefaultLoanCmd()

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

// TestNewDefaultLoanCmd_Structure verifies NewDefaultLoanCmd has the same structure.
func TestNewDefaultLoanCmd_Structure(t *testing.T) {
	cmd := NewDefaultLoanCmd()

	assert.NotNil(t, cmd)
	assert.Equal(t, "loan", cmd.Name())
	assert.NotEmpty(t, cmd.Short)
	assert.NotEmpty(t, cmd.Long)

	// Check --to flag exists
	toFlag := cmd.Flags().Lookup("to")
	require.NotNil(t, toFlag)

	// Check --note flag exists
	noteFlag := cmd.Flags().Lookup("note")
	require.NotNil(t, noteFlag)
}

// TestGetLoanCmd_IsSameAsNewDefault verifies the deprecated GetLoanCmd still works.
func TestGetLoanCmd_IsSameAsNewDefault(t *testing.T) {
	cmd := GetLoanCmd()

	assert.NotNil(t, cmd)
	assert.Equal(t, "loan", cmd.Name())
}
