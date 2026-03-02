# User Clarifications

## Migration Delivery
- Use a new CLI subcommand following the `wherehouse <verb> [<noun>]` pattern
- Suggested name: `wherehouse migrate database`
- Must fit existing cobra command structure

## Nanoid Alphabet
- Alphanumeric only: `A-Za-z0-9` (62 characters)
- No underscores or hyphens

## System Location IDs
- Use `sys0000001` format
- sys0000001 = Missing
- sys0000002 = Borrowed
- sys0000003 = Loaned

## Other Implicit Decisions (from gaps.json not asked)
- Migration should be opt-in (user runs `wherehouse migrate database` explicitly)
- Migration should be atomic (single transaction, all-or-nothing)
- History fallback display: show all 10 characters (since nanoids are only 10 chars, truncation makes no sense)
- Documentation: committed markdown file AND terminal output when migration runs
