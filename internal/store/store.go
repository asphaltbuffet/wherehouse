package store

import (
	"context"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite" // register sqlite driver
)

// DefaultBusyTimeout is the SQLite busy timeout in milliseconds.
const DefaultBusyTimeout = 5000

// dbtx is the query surface shared by [sql.DB] and [sql.Tx].
type dbtx interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// Reader runs read-only queries, either on the connection pool (Store.Reader)
// or inside a transaction (Tx embeds it). It has no write methods (ADR 0043).
type Reader struct{ q dbtx }

// reader lets Tx embed Reader under an unexported field name, so no package
// outside store can construct a Tx.
type reader = Reader

// Tx is a write transaction from ExecInTransaction. It can read (Reader's
// methods are promoted) and is the only way to write. Guards run on the Tx so
// they see earlier writes in the same transaction (ADR 0035).
type Tx struct{ reader }

// Reader returns t's read side, for helpers that only read and are also
// called with Store.Reader.
func (t Tx) Reader() Reader { return t.reader }

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

// Reader returns a Reader over the connection pool for callers that need no
// transaction, such as render decisions (ADR 0040). It cannot write, and a
// guard must never use it (ADR 0035).
func (s *Store) Reader() Reader { return Reader{q: s.db} }

// ExecInTransaction runs fn inside a transaction, committing on success and rolling back on error.
func (s *Store) ExecInTransaction(ctx context.Context, fn func(Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if fnErr := fn(Tx{Reader{q: tx}}); fnErr != nil {
		return fnErr
	}

	if commitErr := tx.Commit(); commitErr != nil {
		return fmt.Errorf("commit transaction: %w", commitErr)
	}
	return nil
}
