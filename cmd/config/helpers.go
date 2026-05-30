package config

import (
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
