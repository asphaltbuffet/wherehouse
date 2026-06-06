// Package cli provides shared command-line interface utilities for wherehouse commands.
//
// This package contains reusable helpers for CLI command implementations including:
//   - Database connection management (OpenDatabase)
//   - Output formatting with lipgloss styling (OutputWriter)
//   - User identity resolution (GetActorUserID)
//   - Config and flag context utilities (GetConfig, MustGetConfig)
package cli
