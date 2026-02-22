package config

// applyDefaults sets default values for any unspecified configuration fields.
// This ensures all required fields have sensible defaults.
func applyDefaults(cfg *Config) {
	// Database defaults
	if cfg.Database.Path == "" {
		cfg.Database.Path = DefaultDatabasePath()
	}

	// User defaults - empty string means use OS username
	// OSUsernameMap defaults to empty map (no mappings)
	if cfg.User.OSUsernameMap == nil {
		cfg.User.OSUsernameMap = make(map[string]string)
	}

	// Output defaults
	if cfg.Output.DefaultFormat == "" {
		cfg.Output.DefaultFormat = "human"
	}
	// Quiet defaults to false (already zero value)
}

// GetDefaults returns a Config struct populated with default values.
// Useful for testing and documentation purposes.
func GetDefaults() *Config {
	cfg := &Config{}
	applyDefaults(cfg)
	return cfg
}
