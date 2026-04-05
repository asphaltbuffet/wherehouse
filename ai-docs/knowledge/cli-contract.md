# Wherehouse CLI Contract (Commands, Output, JSON, Normalization, Completion)

Use as authoritative spec for implementing CLI + completion scripts.

## Command name

```
wherehouse
```

## Global flags (all commands)

- `--db PATH` : path to SQLite database file (required via config/default; must support network paths)
- `--as USER` : override acting user identity for this command (attribution only)
- `--json` : machine-readable output (stable schemas per command)
- `-q` : quiet (suppress normal informational output)
- `-qq` : extra quiet (suppress everything except exit status)

## Name handling

Store both display and canonical forms.

Canonicalization rules (for matching/selectors/dedupe):

- case-insensitive
- trim leading/trailing whitespace
- collapse internal whitespace runs to `_`
- normalize punctuation separators (`-`, `_`, spaces) to `_`
- strip or normalize other punctuation consistently (documented)
- Unicode-safe; display_name may include emoji; canonical_name must be ASCII-safe (or at least consistently comparable)

### Locations

- `locations.canonical_name` is globally unique (enforced)
- location matching uses `canonical_name`

### Items

- `items.display_name` preserved
- `items.canonical_name` used for matching
- duplicates allowed; warn when `canonical_name` already exists (warning only)
- matching is case-insensitive exact on canonical_name (no substring, no fuzzy)

### Selector syntax (non-fuzzy, deterministic)

Items may be referenced by:

- `--id <ITEM_ID>` (always valid)
- `LOCATION:ITEM` (both resolved by canonical names)

**Examples:**

```
wherehouse move tote_f:10mm_socket box_2
```

underscores in CLI args treated as whitespace for canonicalization, but quoting is allowed:

```
wherehouse add "10mm socket 🔧" tote_f
```

## Output contracts

Write commands default to short confirmation including item/location IDs where relevant.

**Example:**

```
Added item "10mm socket 🔧" (id: 8f3a2c) to TOTE F
```

`-q` suppresses this line; `-qq` suppresses everything but exit status.

## Read commands support verbosity:

```
wherehouse where 10mm_socket
```

- default: immediate location name only
- `-v`: full location path
- `-vv`: timestamp + actor + full path (UTC)

Example:

```
$ wherehouse where 10mm_socket
Tote F

$ wherehouse where 10mm_socket -v
Garage >> Shelf A >> Tote F

$ wherehouse where 10mm_socket -vv
2025-11-10T12:34Z (Alice) Garage >> Shelf A >> Tote F
```

## JSON output mode

`--json` returns structured data, stable keys.
Human output and JSON output must be mutually exclusive (except errors to stderr).

## Completion (v1 ergonomics)

Use built-in functionality from spf13/cobra to provide completion scripts for:

- bash
- zsh
- fish

Completion should be “dynamic”:

- query the local DB to list canonical item names and location names
- optionally integrate fzf if installed:
    - fzf is used only in completion layer, not in parser matching
- completion is allowed to be fuzzy; command execution is not
