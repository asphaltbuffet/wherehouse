package cli

import (
	"context"

	"github.com/asphaltbuffet/wherehouse/internal/config"
)

// GetActorUserID determines the actor user ID from context and config.
// Falls back to OS username if no config is available.
//
// The function checks the following sources in order:
//  1. Configured default identity (cfg.User.DefaultIdentity)
//  2. Username mapping (cfg.User.OSUsernameMap[osUsername])
//  3. OS username (fallback)
//
// Returns the resolved actor user ID string.
func GetActorUserID(ctx context.Context) string {
	// Get config from context
	cfg, ok := ctx.Value(config.ConfigKey).(*config.Config)
	if !ok || cfg == nil {
		// Fallback to OS username if config unavailable
		return config.GetCurrentUsername()
	}

	// Use configured default identity if set
	if cfg.User.DefaultIdentity != "" {
		return cfg.User.DefaultIdentity
	}

	// Get OS username
	osUsername := config.GetCurrentUsername()

	// Check if there's a mapped display name
	if displayName, exists := cfg.User.OSUsernameMap[osUsername]; exists {
		return displayName
	}

	// Default to OS username
	return osUsername
}
