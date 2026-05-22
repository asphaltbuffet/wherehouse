package store

import (
	"context"
	"database/sql"
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	_ "modernc.org/sqlite" // register sqlite driver
)

const (
	// DefaultBusyTimeout is the SQLite busy timeout in milliseconds.
	DefaultBusyTimeout = 5000
	// DefaultBaseRetryDelay is the base delay for the first retry in WithRetry.
	DefaultBaseRetryDelay = 50 * time.Millisecond
)

// Tx is a handle to an active database transaction.
type Tx = *sql.Tx

// Config holds connection parameters for opening a Store.
type Config struct {
	Path        string
	BusyTimeout int
	AutoMigrate bool
}

// Store wraps a SQLite database connection.
type Store struct {
	db  *sql.DB
	cfg Config
}

// Open opens (or creates) the SQLite database at cfg.Path.
func Open(cfg Config) (*Store, error) {
	if cfg.Path == "" {
		return nil, ErrDatabasePathRequired
	}
	if cfg.BusyTimeout == 0 {
		cfg.BusyTimeout = DefaultBusyTimeout
	}

	dsn := fmt.Sprintf("file:%s?_busy_timeout=%d&_journal_mode=WAL&_foreign_keys=on", cfg.Path, cfg.BusyTimeout)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	s := &Store{db: db, cfg: cfg}
	if cfg.AutoMigrate {
		if migrateErr := s.RunMigrations(); migrateErr != nil {
			_ = db.Close()
			return nil, fmt.Errorf("auto-migrate: %w", migrateErr)
		}
	}
	return s, nil
}

// Close releases the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

// DB returns the underlying [sql.DB].
func (s *Store) DB() *sql.DB {
	return s.db
}

// ExecInTransaction runs fn inside a transaction, committing on success and rolling back on error.
func (s *Store) ExecInTransaction(ctx context.Context, fn func(Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if fnErr := fn(tx); fnErr != nil {
		return fnErr
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return fmt.Errorf("commit transaction: %w", commitErr)
	}
	return nil
}

// WithRetry retries fn on SQLite BUSY/LOCKED errors with jittered backoff.
func (s *Store) WithRetry(ctx context.Context, fn func() error) error {
	delay := DefaultBaseRetryDelay
	for {
		err := fn()
		if err == nil {
			return nil
		}
		if !isRetryableError(err) {
			return err
		}
		jitter := time.Duration(
			rand.Int64N(int64(delay)), //nolint:gosec // jitter does not need crypto-quality randomness
		)
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay + jitter):
		}
		delay *= 2
	}
}

func isRetryableError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "SQLITE_BUSY") ||
		strings.Contains(msg, "SQLITE_LOCKED") ||
		strings.Contains(msg, "database is locked")
}
