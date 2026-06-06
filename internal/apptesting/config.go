package apptesting

import (
	"testing"

	"github.com/asphaltbuffet/wherehouse/internal/config"
)

// ConfigOption mutates a Config for test purposes.
type ConfigOption func(*config.Config)

// WithJSON sets the output format to JSON.
func WithJSON() ConfigOption {
	return func(cfg *config.Config) {
		cfg.Output.DefaultFormat = config.OutputFormatJSON
	}
}

// NewTestConfig returns a default Config with the given options applied.
func NewTestConfig(t *testing.T, opts ...ConfigOption) *config.Config {
	t.Helper()
	cfg := config.GetDefaults()
	for _, o := range opts {
		o(cfg)
	}
	return cfg
}
