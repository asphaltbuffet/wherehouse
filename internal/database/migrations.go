package database

import (
	"context"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// RunMigrations runs all pending database migrations.
// This is called automatically during Open() if AutoMigrate is enabled.
func (d *Database) RunMigrations() error {
	// Create source driver from embedded filesystem
	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	// Create database driver
	dbDriver, err := sqlite.WithInstance(d.db, &sqlite.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration database driver: %w", err)
	}

	// Create migrator
	m, err := migrate.NewWithInstance("iofs", sourceDriver, "sqlite", dbDriver)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}

	// Run migrations
	if upErr := m.Up(); upErr != nil && !errors.Is(upErr, migrate.ErrNoChange) {
		return fmt.Errorf("migration failed: %w", upErr)
	}

	return nil
}

// GetMigrationVersion returns the current migration version and dirty state.
func (d *Database) GetMigrationVersion() (uint, bool, error) {
	// Create source driver from embedded filesystem
	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return 0, false, fmt.Errorf("failed to create migration source: %w", err)
	}

	// Create database driver
	dbDriver, err := sqlite.WithInstance(d.db, &sqlite.Config{})
	if err != nil {
		return 0, false, fmt.Errorf("failed to create migration database driver: %w", err)
	}

	// Create migrator
	m, err := migrate.NewWithInstance("iofs", sourceDriver, "sqlite", dbDriver)
	if err != nil {
		return 0, false, fmt.Errorf("failed to create migrator: %w", err)
	}

	version, dirty, versionErr := m.Version()
	if versionErr != nil {
		if errors.Is(versionErr, migrate.ErrNilVersion) {
			return 0, false, nil
		}
		return 0, false, versionErr
	}

	return version, dirty, nil
}

// RollbackMigration rolls back to the previous migration version.
// Use with caution - this may result in data loss.
func (d *Database) RollbackMigration() error {
	// Create source driver from embedded filesystem
	sourceDriver, err := iofs.New(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("failed to create migration source: %w", err)
	}

	// Create database driver
	dbDriver, err := sqlite.WithInstance(d.db, &sqlite.Config{})
	if err != nil {
		return fmt.Errorf("failed to create migration database driver: %w", err)
	}

	// Create migrator
	m, err := migrate.NewWithInstance("iofs", sourceDriver, "sqlite", dbDriver)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}

	// Rollback one step
	if stepErr := m.Steps(-1); stepErr != nil {
		return fmt.Errorf("rollback failed: %w", stepErr)
	}

	return nil
}

// SetMigrationVersion manually sets the migration version.
// This is primarily for testing purposes.
func (d *Database) SetMigrationVersion(ctx context.Context, version uint, dirty bool) error {
	dirtyInt := 0
	if dirty {
		dirtyInt = 1
	}

	_, err := d.db.ExecContext(ctx, `
		INSERT OR REPLACE INTO schema_migrations (version, dirty)
		VALUES (?, ?)
	`, version, dirtyInt)

	return err
}
