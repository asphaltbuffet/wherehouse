package version

import "fmt"

const (
	// ApplicationName is the name of the application.
	ApplicationName = "wherehouse"
)

var (
	// Version is the semantic version (injected via ldflags).
	// Development default: "v0.1.0-dev".
	// Release builds inject: "v0.1.0".
	Version = "v0.1.0-dev"

	// GitCommit is the git commit hash (short form).
	// Injected via ldflags during build.
	GitCommit = "unknown"

	// BuildDate is the build timestamp (ISO 8601 format).
	// Injected via ldflags during build.
	BuildDate = "unknown"
)

// FullVersion returns the complete version string including commit hash.
// Format: v0.1.0-dev+abc1234 or v0.1.0+abc1234 for releases.
// This is the primary version string displayed to users.
func FullVersion() string {
	return fmt.Sprintf("%s+%s", Version, GitCommit)
}

// ShortVersion returns only the semantic version without commit hash.
// Format: v0.1.0-dev or v0.1.0 for releases.
// Useful for version comparisons and compatibility checks.
func ShortVersion() string {
	return Version
}

// BuildInfo returns build metadata including commit and timestamp.
// Format: abc1234 (2026-02-20T12:34:56Z)
// Returns empty string if no build information available.
func BuildInfo() string {
	if GitCommit == "unknown" && BuildDate == "unknown" {
		return ""
	}
	if BuildDate == "unknown" {
		return GitCommit
	}
	return fmt.Sprintf("%s (%s)", GitCommit, BuildDate)
}
