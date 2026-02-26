// Package logging provides a structured, leveled, file-only logger for wherehouse.
//
// The package exposes a package-level singleton logger backed by [log/slog] with
// a text handler. Log rotation is optionally handled by lumberjack when MaxSizeMB
// is non-zero. The filesystem is abstracted via [afero.Fs] so tests can use an
// in-memory filesystem without touching disk.
//
// Typical usage:
//
//	// In main or cmd/root.go:
//	if err := logging.Init(afero.NewOsFs(), logPath, "warn", 0, 0); err != nil {
//	    fmt.Fprintf(os.Stderr, "warning: logging init failed: %v\n", err)
//	}
//	defer logging.Close()
//
//	// Anywhere in the application:
//	logging.Info("item moved", "item_id", id, "to", location)
//
//	// Component-scoped logger:
//	dbLog := logging.With("component", "database")
//	dbLog.Debug("query executed", "sql", query)
package logging
