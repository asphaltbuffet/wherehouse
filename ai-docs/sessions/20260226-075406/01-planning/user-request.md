# User Request

## Problem
If there isn't a database file, the `add` command shows an error that says "there isn't enough memory", which is misleading.

## Requirements

### 1. Update error when trying to access database but file isn't present
- When any command tries to access the database and the file doesn't exist, show a clear, helpful error message (not the misleading "not enough memory" error)
- The error should guide the user to run `init` to create the database

### 2. Add basic validation in prerun for any command that accesses database
- Add a PreRun (or PersistentPreRun) hook to any command that needs database access
- Check if the database file is present before attempting to open it
- If not present, show a user-friendly error message pointing them to `wherehouse init`

### 3. Add `init` command
- `--database` flag: creates the database (fails if already present)
- `--force` flag: overwrites existing database (renames current by adding `.backup` to end of existing file name)
- If unable to create backup, shows warning but overwrites anyway (does not fail)

## Notes
- The `init` command should be safe to run as a first-time setup step
- The backup behavior should be: rename existing file to `<filename>.backup`, then create fresh database
- If backup rename fails (e.g., permissions issue), warn but proceed with overwrite
- The `--database` flag on init likely specifies WHERE to create the database (path), consistent with other commands that use `--database` to specify DB path
