# Attribute changes use one combined `attr` command with explicit-value flags

Changing an entity's `locked` / `discrete` attributes after creation is done with a single command, `wherehouse attr <path>... [--locked[=bool]] [--discrete[=bool]]`, rather than per-intent verbs like the status commands (`lost`, `found`, `loan`, `return`). The flags reuse the exact spelling of `add --locked --discrete`, and clearing is spelled `--locked=false`, so there is one vocabulary for attributes whether at creation or later, and contradictory input (`--lock --unlock`) cannot be expressed. The whole invocation — every path and every attribute — is one atomic batch, and each change still enforces ADR 0024's rule that a command which cannot make a meaningful change is an error, not a no-op event.

## Considered Options

**Four top-level verbs (`lock`, `unlock`, `discrete`, `undiscrete`).** This mirrors the status commands and gives each intent a specific error, but it adds four top-level commands for two booleans, needs an awkward inverse verb for `discrete` (or a new metaphor such as `seal` that the glossary would have to define), and cannot change both attributes in one atomic call. Rejected.

**Four boolean flags on one command (`--lock --unlock --discrete --undiscrete`).** Reads more like verbs but requires conflict rules between paired flags and diverges from `add`'s spelling. Rejected.

## Consequences

- `--locked false` (space-separated) is parsed by cobra as `--locked` plus a path argument `false`; help text and examples must show the `=` form.
- With no attribute flags, `attr` is read-only and lists each entity's attributes (as `tag <path>` lists tags); `--note` without an attribute flag is an error.
- Attributes are `attr`; status is the intent verbs. A future reader should not "unify" them by adding `lock`/`unlock` aliases or by folding status changes into `attr`.
