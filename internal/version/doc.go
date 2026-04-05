// Package version provides version information for the wherehouse application.
// Version information follows semantic versioning 2.0 (semver.org).
//
// Build-time injection:
//
//	go build -ldflags "\
//	  -X github.com/asphaltbuffet/wherehouse/internal/version.Version=v0.1.0 \
//	  -X github.com/asphaltbuffet/wherehouse/internal/version.GitCommit=abc1234 \
//	  -X github.com/asphaltbuffet/wherehouse/internal/version.BuildDate=2026-02-20T12:34:56Z"
//
// Version format:
//
//	Development: v0.1.0-dev+abc1234
//	Release:     v0.1.0+abc1234
package version
