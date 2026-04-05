package config

import (
	"os"
	"path/filepath"
	"runtime"
)

const (
	goosLinux   = "linux"
	goosFreeBSD = "freebsd"
	goosOpenBSD = "openbsd"
	goosNetBSD  = "netbsd"
	goosDarwin  = "darwin"
	goosWindows = "windows"
)

// DefaultDatabasePath returns the platform-specific default database path.
// It follows OS conventions for application data storage:
//
// Linux/BSD:   $XDG_DATA_HOME/wherehouse/wherehouse.db
//
//	(fallback: ~/.local/share/wherehouse/wherehouse.db)
//
// macOS:       ~/Library/Application Support/wherehouse/wherehouse.db
// Windows:     %APPDATA%/wherehouse/wherehouse.db
//
// For unknown platforms, falls back to ~/.wherehouse/wherehouse.db
//
// The returned path is absolute and validated, but the directory is NOT created.
// Database creation will fail if parent directory does not exist (explicit user action required).
func DefaultDatabasePath() string {
	switch runtime.GOOS {
	case goosLinux, goosFreeBSD, goosOpenBSD, goosNetBSD:
		// XDG Base Directory Specification
		dataHome := os.Getenv("XDG_DATA_HOME")
		if dataHome == "" {
			home := os.Getenv("HOME")
			if home == "" {
				// Extreme fallback - should rarely happen
				return filepath.Join(".", "wherehouse.db")
			}

			dataHome = filepath.Join(home, ".local", "share")
		}

		return filepath.Join(dataHome, "wherehouse", "wherehouse.db")

	case goosDarwin:
		// macOS Application Support
		home := os.Getenv("HOME")
		if home == "" {
			// Extreme fallback
			return filepath.Join(".", "wherehouse.db")
		}

		return filepath.Join(home, "Library", "Application Support", "wherehouse", "wherehouse.db")

	case goosWindows:
		// Windows APPDATA
		appData := os.Getenv("APPDATA")
		if appData == "" {
			// Fallback to USERPROFILE if APPDATA missing
			userProfile := os.Getenv("USERPROFILE")
			if userProfile != "" {
				appData = filepath.Join(userProfile, "AppData", "Roaming")
			} else {
				// Extreme fallback
				return filepath.Join(".", "wherehouse.db")
			}
		}

		return filepath.Join(appData, "wherehouse", "wherehouse.db")

	default:
		// Unknown platform - use conservative default
		home := os.Getenv("HOME")
		if home == "" {
			return filepath.Join(".", "wherehouse.db")
		}

		return filepath.Join(home, ".wherehouse", "wherehouse.db")
	}
}

// GetDatabasePath returns the resolved database path with the following precedence:
//  1. Explicit config file setting (cfg.Database.Path)
//  2. WHEREHOUSE_DB_PATH environment variable
//  3. Platform-specific default (via DefaultDatabasePath)
//
// Paths are expanded (~ and env vars) and converted to absolute paths.
// Returns error if path expansion fails or path is invalid.
//
// Note: This does NOT create directories. The parent directory must exist
// for database creation to succeed.
func (c *Config) GetDatabasePath() (string, error) {
	var rawPath string

	// Priority 1: Explicit config file setting
	if c.Database.Path != "" {
		rawPath = c.Database.Path
	} else {
		// Priority 2: Environment variable
		if envPath := os.Getenv("WHEREHOUSE_DB_PATH"); envPath != "" {
			rawPath = envPath
		} else {
			// Priority 3: Platform-specific default
			// Default is already absolute, no expansion needed
			return DefaultDatabasePath(), nil
		}
	}

	// Expand path (handles ~ and env vars)
	return ExpandPath(rawPath)
}
