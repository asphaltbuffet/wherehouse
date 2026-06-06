package apptesting_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/asphaltbuffet/wherehouse/internal/apptesting"
)

func TestNewTestConfig_Defaults(t *testing.T) {
	cfg := apptesting.NewTestConfig(t)
	assert.False(t, cfg.IsJSON())
	assert.False(t, cfg.IsQuiet())
}

func TestNewTestConfig_WithJSON(t *testing.T) {
	cfg := apptesting.NewTestConfig(t, apptesting.WithJSON())
	assert.True(t, cfg.IsJSON())
}

func TestNewTestConfig_WithQuiet(t *testing.T) {
	cfg := apptesting.NewTestConfig(t, apptesting.WithQuiet())
	assert.True(t, cfg.IsQuiet())
}
