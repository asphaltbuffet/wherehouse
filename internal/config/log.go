package config

import (
	"os"
	"path/filepath"
	"runtime"

	"github.com/asphaltbuffet/wherehouse/internal/logging"
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

// LegacyLogPathEnv is the removed log path environment variable. It is no
// longer read; LegacyLogPathSet lets callers warn users who still set it.
const LegacyLogPathEnv = "WHEREHOUSE_LOG_PATH"

// LegacyLogPathSet reports whether the removed WHEREHOUSE_LOG_PATH is set.
func LegacyLogPathSet() bool {
	return os.Getenv(LegacyLogPathEnv) != ""
}

// GetLogPath returns the resolved log file path:
//  1. logging.StderrPath ("-"), returned verbatim to select stderr
//  2. cfg.Logging.FilePath (also set by WHEREHOUSE_LOGGING_FILE_PATH), expanded
//  3. Platform-specific default (via DefaultLogPath)
func (c *Config) GetLogPath() (string, error) {
	switch c.Logging.FilePath {
	case logging.StderrPath:
		return logging.StderrPath, nil
	case "":
		return DefaultLogPath(), nil
	default:
		return ExpandPath(c.Logging.FilePath)
	}
}
