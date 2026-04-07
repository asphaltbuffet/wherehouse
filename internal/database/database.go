// Package database provides SQLite database access for the wherehouse inventory system.
package database

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	_ "modernc.org/sqlite" // SQLite driver
)

// Database wraps the SQLite database connection and provides methods for database operations.
type Database struct {
	db *sql.DB
	// System location caching for performance
	missingLocationID   string
	borrowedLocationID  string
	loanedLocationID    string
	removedLocationID   string
	systemLocationsOnce sync.Once
}

// Config holds database configuration options.
type Config struct {
	// Path to the SQLite database file
	Path string
	// BusyTimeout in milliseconds for locked database retries
	BusyTimeout int
	// AutoMigrate runs migrations on Open if true
	AutoMigrate bool
}

const (
	// DefaultBusyTimeout is the default timeout in milliseconds for database locks.
	DefaultBusyTimeout = 30000 // 30 seconds

	// DefaultBaseRetryDelay is the base delay for exponential backoff retries.
	DefaultBaseRetryDelay = 100 * time.Millisecond
)

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		BusyTimeout: DefaultBusyTimeout,
		AutoMigrate: true,
	}
}

// Open opens a connection to the SQLite database and configures it for use.
// If AutoMigrate is enabled in the config, it runs pending migrations.
func Open(cfg Config) (*Database, error) {
	if cfg.Path == "" {
		return nil, ErrDatabasePathRequired
	}

	// Open database connection
	db, err := sql.Open("sqlite", cfg.Path)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Configure connection pool
	db.SetMaxOpenConns(1) // SQLite can only handle one writer at a time
	db.SetMaxIdleConns(1)
	db.SetConnMaxLifetime(0)

	// Configure SQLite PRAGMAs
	ctx := context.Background()
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA synchronous=NORMAL",
		fmt.Sprintf("PRAGMA busy_timeout=%d", cfg.BusyTimeout),
		"PRAGMA wal_autocheckpoint=1000",
	}

	for _, pragma := range pragmas {
		if _, pragmaErr := db.ExecContext(ctx, pragma); pragmaErr != nil {
			_ = db.Close()
			return nil, fmt.Errorf("failed to set pragma %q: %w", pragma, pragmaErr)
		}
	}

	// Verify connection
	if pingErr := db.PingContext(ctx); pingErr != nil {
		_ = db.Close()
		return nil, fmt.Errorf("failed to ping database: %w", pingErr)
	}

	database := &Database{db: db}

	// Run migrations if enabled
	if cfg.AutoMigrate {
		if migrateErr := database.RunMigrations(); migrateErr != nil {
			_ = db.Close()

			return nil, fmt.Errorf("failed to run migrations: %w", migrateErr)
		}

		// Seed system locations after initial migration
		if seedErr := database.seedSystemLocations(context.Background()); seedErr != nil {
			_ = db.Close()

			return nil, fmt.Errorf("failed to seed system locations: %w", seedErr)
		}
	}

	return database, nil
}

// Close closes the database connection.
func (d *Database) Close() error {
	if d.db != nil {
		return d.db.Close()
	}

	return nil
}

// ExecInTransaction executes a function within a transaction.
// If the function returns an error, the transaction is rolled back.
// Otherwise, the transaction is committed.
func (d *Database) ExecInTransaction(ctx context.Context, fn func(*sql.Tx) error) error {
	tx, err := d.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer func() {
		_ = tx.Rollback() // Rollback if not committed (errors ignored as commit may have succeeded)
	}()

	if fnErr := fn(tx); fnErr != nil {
		return fnErr
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return fmt.Errorf("failed to commit transaction: %w", commitErr)
	}

	return nil
}

// WithRetry executes a function with exponential backoff retry logic.
// This is useful for handling SQLite BUSY errors on write operations.
func (d *Database) WithRetry(ctx context.Context, fn func() error) error {
	const maxRetries = 5
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if fnErr := fn(); fnErr != nil {
			lastErr = fnErr
			// Check if error is retryable (SQLITE_BUSY or SQLITE_LOCKED)
			if !isRetryableError(fnErr) {
				return fnErr
			}

			if attempt < maxRetries {
				delay := DefaultBaseRetryDelay * time.Duration(1<<uint(attempt))
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(delay):
					continue
				}
			}
		} else {
			return nil
		}
	}

	return fmt.Errorf("operation failed after %d retries: %w", maxRetries, lastErr)
}

// isRetryableError checks if an error is retryable (SQLITE_BUSY, SQLITE_LOCKED).
func isRetryableError(err error) bool {
	// Check for SQLite busy/locked errors. These are typically safe to retry
	if err == nil {
		return false
	}

	errStr := err.Error()

	return errStr == "database is locked" || errStr == "database table is locked"
}

// DB returns the underlying [sql.DB] for direct access if needed.
// Use with caution - prefer using the Database methods when possible.
func (d *Database) DB() *sql.DB {
	return d.db
}

// initSystemLocations initializes the system location ID cache.
// This is called once on first access via [sync.Once] pattern.
func (d *Database) initSystemLocations(ctx context.Context) error {
	const query = `
		SELECT location_id, canonical_name
		FROM locations_current
		WHERE is_system = 1 AND canonical_name IN ('missing', 'borrowed', 'loaned', 'removed')
	`

	rows, err := d.db.QueryContext(ctx, query)
	if err != nil {
		return fmt.Errorf("failed to query system locations: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var locID, canonName string
		if scanErr := rows.Scan(&locID, &canonName); scanErr != nil {
			return fmt.Errorf("failed to scan system location: %w", scanErr)
		}

		switch canonName {
		case "missing":
			d.missingLocationID = locID
		case "borrowed":
			d.borrowedLocationID = locID
		case "loaned":
			d.loanedLocationID = locID
		case "removed":
			d.removedLocationID = locID
		}
	}

	if rowsErr := rows.Err(); rowsErr != nil {
		return fmt.Errorf("error iterating system locations: %w", rowsErr)
	}

	return nil
}
