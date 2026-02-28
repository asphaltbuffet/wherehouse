package config

import (
	"os"

	"github.com/spf13/afero"
)

// cmdFS is the filesystem abstraction used by all config commands.
// By default it uses the OS filesystem, but can be injected with
// a different implementation (e.g., in-memory) for testing.
var cmdFS afero.Fs = afero.NewOsFs()

// SetFilesystem allows injecting a filesystem implementation for testing.
// This enables unit tests to use in-memory filesystems without touching
// the real filesystem.
func SetFilesystem(fs afero.Fs) {
	cmdFS = fs
}

// fileExists checks if a file exists and is accessible.
// Returns (true, nil) if the file exists.
// Returns (false, nil) if the file does not exist.
// Returns (false, err) if there was an error checking (e.g., permission denied).
func fileExists(fs afero.Fs, path string) (bool, error) {
	_, err := fs.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
