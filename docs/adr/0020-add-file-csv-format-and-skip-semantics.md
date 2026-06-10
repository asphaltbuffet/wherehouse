# add --file uses CSV with trailing-optional columns and skip-on-warning semantics

The bulk-add file format is CSV (`encoding/csv`, stdlib, no new dependency). Columns are `path, locked, discrete, tags` — trailing columns are optional and default to `false`/empty; an empty middle field (e.g. `path,,true`) is valid and defaults that field. Tags are space-separated within the fourth field. Comment lines starting with `#` are skipped. Boolean fields accept only `"true"` or `"false"`; any other value is a hard error with a line number.

All rows load in a single transaction. Rows that trigger warnings (DB duplicate without `--allow-duplicates`, existing parent with `--create-parents`) are skipped within the transaction; hard errors (malformed path, discrete parent, invalid boolean, missing parent without `--create-parents`) abort the entire batch. This extends the all-or-nothing semantics from ADR 0017 while allowing non-fatal skips for expected duplicate conditions.
