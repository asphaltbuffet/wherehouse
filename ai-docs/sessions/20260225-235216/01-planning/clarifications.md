# User Clarifications

## Comment preservation on set/unset
**Answer**: Strip comments (acceptable)
- Current behavior is fine - go-toml/v2 will lose comments on write, which is acceptable.
- No need for a different TOML library or complex AST editing.

## Golden file for config init
**Answer**: Golden file is for TESTING ONLY, not for the actual init template
- User note: "I meant that the golden file is used for testing, not for setting up a user's config"
- The `config init` command should generate a config from defaults/code
- A golden file should be used in tests to validate the exact output of `config init`
- The golden file is NOT rendered as a template at runtime - it's a test fixture

## config set scope
**Answer**: All known keys (including logging.*)
- Expose all config fields including logging.* via `config set`
- More complete and consistent API
