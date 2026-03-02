# Implementation Changes

## New Files
- internal/nanoid/nanoid.go — nanoid package (New, MustNew, IDLength, alphabet)
- internal/nanoid/nanoid_test.go — TDD tests for nanoid package
- internal/cli/migrate.go — MigrateDatabase business logic
- internal/cli/migrate_test.go — TDD tests for migrate logic
- cmd/migrate/migrate.go — thin cobra migrate parent command
- cmd/migrate/database.go — thin cobra migrate database subcommand
- cmd/migrate/doc.go — package doc
- cmd/migrate/database_test.go — command registration tests
- internal/database/migrations/000003_nanoid_migration.up.sql — schema version bump
- internal/database/migrations/000003_nanoid_migration.down.sql — down migration
- docs/migration-nanoid.md — user-facing migration documentation

## Modified Files
- internal/cli/selectors.go — LooksLikeUUID → LooksLikeID (10-char alphanumeric)
- internal/cli/selectors_test.go — tests updated, uuid → nanoid
- internal/database/schema_metadata.go — system location IDs → sys0000001/2/3
- internal/database/queries.go (or similar) — added GetAllLocations, GetAllItems, ExecInTransaction
- internal/database/helper_test.go — 14 UUID constants → 10-char alphanumeric
- cmd/add/item.go — uuid.NewV7() → nanoid.New()
- cmd/add/location.go — uuid.NewV7() → nanoid.New()
- cmd/history/output.go — removed uuidPrefixLength, show full ID
- cmd/move/item_test.go — uuid → nanoid
- cmd/lost/item_test.go — uuid → nanoid
- cmd/list/list_test.go — uuid → nanoid
- cmd/root.go — registered migrate command
- 16 cmd/**/*.go files — UUID→ID doc string updates
- go.mod / go.sum — added go-nanoid/v2, uuid demoted to indirect
