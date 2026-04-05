# Migration: UUID to nanoid

## Overview

Wherehouse IDs have changed from UUID format (e.g. `550e8400-e29b-41d4-a716-446655440000`) to
10-character alphanumeric nanoid format (e.g. `aB3xK9mPqR`).

New IDs are generated using the `A-Za-z0-9` alphabet (62 characters) at length 10, giving
62^10 ≈ 839 trillion unique values — ample for a personal inventory tool.

## Who Needs to Act

If you have an **existing database** created before this change, you must run the database
migration command. New installations are unaffected.

## Before You Start

**Back up your database.** The migration is atomic (all-or-nothing), but a backup is still
strongly recommended.

```sh
cp ~/.config/wherehouse/wherehouse.db ~/.config/wherehouse/wherehouse.db.bak
```

## Running the Migration

### 1. Preview (dry run)

See what will change without modifying anything:

```sh
wherehouse migrate database --dry-run
```

This prints the old→new ID mapping for every location and item. No changes are written.

### 2. Apply

```sh
wherehouse migrate database
```

The command will:
1. Build a mapping of every entity ID (UUID → nanoid)
2. Apply all changes in a **single atomic transaction** — either everything succeeds or nothing changes
3. Rewrite IDs in:
   - `locations_current` (location_id, parent_id)
   - `items_current` (item_id, location_id, temp_origin_location_id)
   - `events` (item_id, location_id index columns, and the payload JSON blobs)

System locations receive fixed deterministic IDs:

| Location | New ID     |
|----------|------------|
| Missing  | sys0000001 |
| Borrowed | sys0000002 |
| Loaned   | sys0000003 |

### 3. Verify

After migration, normal commands should work as before:

```sh
wherehouse list
wherehouse find <item>
wherehouse history
```

## Idempotency

The migration is safe to run more than once. Any ID that is already in nanoid format
(10-character alphanumeric) is left unchanged.

## Rollback

The migration command does not provide automated rollback. To revert:

1. Stop using wherehouse
2. Restore your backup:
   ```sh
   cp ~/.config/wherehouse/wherehouse.db.bak ~/.config/wherehouse/wherehouse.db
   ```
3. Downgrade to the previous wherehouse version

## External References

If you have scripts or integrations that reference entity IDs by their UUID values
(e.g. hardcoded in shell scripts, notes, or other tools), those references will be
invalid after migration. Use `--dry-run` to obtain the old→new mapping before applying,
and update any external references accordingly.
