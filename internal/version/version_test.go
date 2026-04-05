package version

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestApplicationName(t *testing.T) {
	assert.Equal(t, "wherehouse", ApplicationName, "Application name should be 'wherehouse'")
}

func TestFullVersion_DefaultValues(t *testing.T) {
	// Save original values
	origVersion := Version
	origCommit := GitCommit
	defer func() {
		Version = origVersion
		GitCommit = origCommit
	}()

	// Reset to defaults
	Version = "v0.1.0-dev"
	GitCommit = "unknown"

	expected := "v0.1.0-dev+unknown"
	assert.Equal(t, expected, FullVersion(), "Default FullVersion should match expected format")
}

func TestFullVersion_WithGitCommit(t *testing.T) {
	// Save original values
	origVersion := Version
	origCommit := GitCommit
	defer func() {
		Version = origVersion
		GitCommit = origCommit
	}()

	// Simulate build injection
	Version = "v0.1.0-dev"
	GitCommit = "abc1234"

	expected := "v0.1.0-dev+abc1234"
	assert.Equal(t, expected, FullVersion(), "FullVersion with git commit should match expected format")
}

func TestFullVersion_ReleaseVersion(t *testing.T) {
	// Save original values
	origVersion := Version
	origCommit := GitCommit
	defer func() {
		Version = origVersion
		GitCommit = origCommit
	}()

	// Simulate release build injection
	Version = "v0.1.0"
	GitCommit = "abc1234"

	expected := "v0.1.0+abc1234"
	assert.Equal(t, expected, FullVersion(), "Release FullVersion should not have -dev suffix")
}

func TestShortVersion_Default(t *testing.T) {
	// Save original value
	origVersion := Version
	defer func() {
		Version = origVersion
	}()

	Version = "v0.1.0-dev"

	expected := "v0.1.0-dev"
	assert.Equal(t, expected, ShortVersion(), "ShortVersion should return version without commit hash")
}

func TestShortVersion_Release(t *testing.T) {
	// Save original value
	origVersion := Version
	defer func() {
		Version = origVersion
	}()

	Version = "v0.1.0"

	expected := "v0.1.0"
	assert.Equal(t, expected, ShortVersion(), "ShortVersion for release should not have -dev suffix")
}

func TestBuildInfo_NoInfo(t *testing.T) {
	// Save original values
	origCommit := GitCommit
	origDate := BuildDate
	defer func() {
		GitCommit = origCommit
		BuildDate = origDate
	}()

	// Both unknown
	GitCommit = "unknown"
	BuildDate = "unknown"

	assert.Empty(t, BuildInfo(), "BuildInfo should return empty string when no build info available")
}

func TestBuildInfo_CommitOnly(t *testing.T) {
	// Save original values
	origCommit := GitCommit
	origDate := BuildDate
	defer func() {
		GitCommit = origCommit
		BuildDate = origDate
	}()

	GitCommit = "abc1234"
	BuildDate = "unknown"

	expected := "abc1234"
	assert.Equal(t, expected, BuildInfo(), "BuildInfo should return commit when date is unknown")
}

func TestBuildInfo_CommitAndDate(t *testing.T) {
	// Save original values
	origCommit := GitCommit
	origDate := BuildDate
	defer func() {
		GitCommit = origCommit
		BuildDate = origDate
	}()

	GitCommit = "abc1234"
	BuildDate = "2026-02-20T12:34:56Z"

	expected := "abc1234 (2026-02-20T12:34:56Z)"
	assert.Equal(t, expected, BuildInfo(), "BuildInfo should include both commit and date")
}

func TestSemanticVersionFormat_Default(t *testing.T) {
	// Save original value
	origVersion := Version
	defer func() {
		Version = origVersion
	}()

	Version = "v0.1.0-dev"

	// Verify starts with 'v'
	assert.True(t, strings.HasPrefix(Version, "v"), "Version should start with 'v'")

	// Verify contains dev suffix
	assert.Contains(t, Version, "-dev", "Development version should contain '-dev' suffix")
}

func TestSemanticVersionFormat_Release(t *testing.T) {
	// Save original value
	origVersion := Version
	defer func() {
		Version = origVersion
	}()

	Version = "v0.1.0"

	// Verify starts with 'v'
	assert.True(t, strings.HasPrefix(Version, "v"), "Version should start with 'v'")

	// Verify does NOT contain dev suffix
	assert.NotContains(t, Version, "-dev", "Release version should not contain '-dev' suffix")
}

func TestFullVersion_Format(t *testing.T) {
	tests := []struct {
		name       string
		version    string
		gitCommit  string
		want       string
		wantPrefix string
	}{
		{
			name:       "development with commit",
			version:    "v0.1.0-dev",
			gitCommit:  "abc1234",
			want:       "v0.1.0-dev+abc1234",
			wantPrefix: "v",
		},
		{
			name:       "development unknown commit",
			version:    "v0.1.0-dev",
			gitCommit:  "unknown",
			want:       "v0.1.0-dev+unknown",
			wantPrefix: "v",
		},
		{
			name:       "release with commit",
			version:    "v0.1.0",
			gitCommit:  "def5678",
			want:       "v0.1.0+def5678",
			wantPrefix: "v",
		},
		{
			name:       "release unknown commit",
			version:    "v0.1.0",
			gitCommit:  "unknown",
			want:       "v0.1.0+unknown",
			wantPrefix: "v",
		},
		{
			name:       "patch version",
			version:    "v0.1.1-dev",
			gitCommit:  "789abcd",
			want:       "v0.1.1-dev+789abcd",
			wantPrefix: "v",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original values
			origVersion := Version
			origCommit := GitCommit
			defer func() {
				Version = origVersion
				GitCommit = origCommit
			}()

			// Set test values
			Version = tt.version
			GitCommit = tt.gitCommit

			got := FullVersion()
			assert.Equal(t, tt.want, got, "FullVersion should match expected format")
			assert.True(t, strings.HasPrefix(got, tt.wantPrefix), "FullVersion should start with 'v'")
			assert.Contains(t, got, "+", "FullVersion should contain '+' separator")
		})
	}
}

func TestBuildInfo_Format(t *testing.T) {
	tests := []struct {
		name      string
		gitCommit string
		buildDate string
		want      string
	}{
		{
			name:      "both unknown",
			gitCommit: "unknown",
			buildDate: "unknown",
			want:      "",
		},
		{
			name:      "commit only",
			gitCommit: "abc1234",
			buildDate: "unknown",
			want:      "abc1234",
		},
		{
			name:      "both provided",
			gitCommit: "abc1234",
			buildDate: "2026-02-20T12:34:56Z",
			want:      "abc1234 (2026-02-20T12:34:56Z)",
		},
		{
			name:      "different commit format",
			gitCommit: "def5678",
			buildDate: "2026-02-20T00:00:00Z",
			want:      "def5678 (2026-02-20T00:00:00Z)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Save original values
			origCommit := GitCommit
			origDate := BuildDate
			defer func() {
				GitCommit = origCommit
				BuildDate = origDate
			}()

			// Set test values
			GitCommit = tt.gitCommit
			BuildDate = tt.buildDate

			got := BuildInfo()
			assert.Equal(t, tt.want, got, "BuildInfo should match expected format")
		})
	}
}
