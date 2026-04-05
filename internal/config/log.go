package config

import (
	"os"
	"path/filepath"
	"runtime"
)

// DefaultLogPath returns the platform-specific default log file path.
// It follows XDG Base Directory Specification (Linux primary target):
//
// Linux/BSD:   $XDG_STATE_HOME/wherehouse/wherehouse.log
//
//	(fallback: ~/.local/state/wherehouse/wherehouse.log)
//
// macOS:       ~/Library/Logs/wherehouse/wherehouse.log
// Windows:     %LOCALAPPDATA%\wherehouse\wherehouse.log
//
//	(fallback: %USERPROFILE%\AppData\Local\wherehouse\wherehouse.log)
//
// Other:       ~/.wherehouse/wherehouse.log
//
// The returned path is absolute. The directory is NOT created here.
func DefaultLogPath() string {
	switch runtime.GOOS {
	case goosLinux, goosFreeBSD, goosOpenBSD, goosNetBSD:
		stateHome := os.Getenv("XDG_STATE_HOME")
		if stateHome == "" {
			home := os.Getenv("HOME")
			if home == "" {
				return filepath.Join(".", "wherehouse.log")
			}
			stateHome = filepath.Join(home, ".local", "state")
		}
		return filepath.Join(stateHome, "wherehouse", "wherehouse.log")

	case goosDarwin:
		home := os.Getenv("HOME")
		if home == "" {
			return filepath.Join(".", "wherehouse.log")
		}
		return filepath.Join(home, "Library", "Logs", "wherehouse", "wherehouse.log")

	case goosWindows:
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData == "" {
			userProfile := os.Getenv("USERPROFILE")
			if userProfile != "" {
				localAppData = filepath.Join(userProfile, "AppData", "Local")
			} else {
				return filepath.Join(".", "wherehouse.log")
			}
		}
		return filepath.Join(localAppData, "wherehouse", "wherehouse.log")

	default:
		home := os.Getenv("HOME")
		if home == "" {
			return filepath.Join(".", "wherehouse.log")
		}
		return filepath.Join(home, ".wherehouse", "wherehouse.log")
	}
}

// GetLogPath returns the resolved log file path with the following precedence:
//  1. Explicit config file setting (cfg.Logging.FilePath)
//  2. WHEREHOUSE_LOG_PATH environment variable
//  3. Platform-specific default (via DefaultLogPath)
//
// Paths are expanded (~ and env vars) and converted to absolute paths.
// Returns error if path expansion fails.
func (c *Config) GetLogPath() (string, error) {
	if c.Logging.FilePath != "" {
		return ExpandPath(c.Logging.FilePath)
	}
	if envPath := os.Getenv("WHEREHOUSE_LOG_PATH"); envPath != "" {
		return ExpandPath(envPath)
	}
	return DefaultLogPath(), nil
}
