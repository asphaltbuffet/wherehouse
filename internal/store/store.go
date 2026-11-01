package store

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // register sqlite driver
)

// DefaultBusyTimeout is the SQLite busy timeout in milliseconds.
const DefaultBusyTimeout = 5000

// Tx is the interface passed to functions running inside ExecInTransaction.
// It exposes only the query methods needed by store operations.
type Tx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

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

	// _txlock=immediate takes the write lock at BEGIN, so concurrent writers
	// queue on busy_timeout and each sees the previous commit before its
	// guards run. Deferred BEGIN lets two transactions read the same state,
	// then one fails its lock upgrade with SQLITE_BUSY (or both write).
	dsn := fmt.Sprintf(
		"file:%s?_busy_timeout=%d&_journal_mode=WAL&_foreign_keys=on&_txlock=immediate",
		cfg.Path, cfg.BusyTimeout,
	)
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

// Path returns the absolute filesystem path of the database file.
func (s *Store) Path() string {
	return s.cfg.Path
}

// DB returns the underlying [sql.DB].
func (s *Store) DB() *sql.DB {
	return s.db
}

// Reader returns the pooled connection as a Tx for read-only callers that
// need no transaction, such as render decisions (ADR 0040). Never write
// through it, and never use it for a guard (ADR 0035).
func (s *Store) Reader() Tx { return s.db }

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
