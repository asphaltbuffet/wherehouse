// Package store provides mechanical SQLite persistence for wherehouse.
// It owns connection management, migrations, and typed SQL read/write operations.
// It contains no business logic — callers decide what to write.
//
// Queries go through two types (ADR 0043). A Reader runs reads, either on the
// connection pool (Store.Reader) or inside a transaction. A Tx, handed out by
// Store.ExecInTransaction, embeds Reader and is the only way to write.
// Tx.AppendEvent is the only writer to the events table. Every entity read
// names its status filter: Live (excluding removed entities) or AnyStatus.
package store
