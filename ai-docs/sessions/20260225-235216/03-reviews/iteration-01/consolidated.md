# Consolidated Review: Configuration Refactoring

**Session**: 20260225-235216
**Date**: 2026-02-26
**Overall Assessment**: CHANGES_NEEDED
**Risk**: Low

---

## IMPORTANT (3 issues)

| ID | File | Line | Summary |
|----|------|------|---------|
| I1 | `cmd/config/get.go` | 58 | Context key mismatch -- uses string `"config"` instead of typed `config.ConfigKey`. `config get` always returns "configuration not loaded". |
| I2 | `cmd/config/check.go` | 49-50 | `ExpandPath` errors silently ignored with `_`. Empty/incorrect paths treated as "not found". |
| I3 | `cmd/config/check.go` | 55,67 | `fileExists` errors silently ignored. Permission-denied errors treated as "file does not exist". |

## MEDIUM (4 issues)

| ID | File | Line | Summary |
|----|------|------|---------|
| M1 | `cmd/root.go` | bindFlagsToConfig | `--json=false` still sets format to "json" because only `Changed` is checked, not the actual value. Same for `--quiet`. |
| M2 | `internal/config/writer_test.go` | unmarshalTOML | Helper uses viper instead of `toml.Unmarshal`, so round-trip test does not match `Check`'s code path. |
| M3 | `cmd/config/helpers.go` | ensureDir | Function has no callers outside test files after refactoring. Dead code candidate. |
| M4 | `cmd/config/set.go` | 36 | Pre-existing lint violation: magic number `2` in `cobra.ExactArgs(2)`. |

## LOW (2 issues)

| ID | File | Summary |
|----|------|---------|
| L1 | `internal/config/writer.go` | `parseConfigValue` allocates a map per call for level validation. Negligible for CLI. |
| L2 | `internal/config/writer.go` | `WriteDefault` manually lists 9 defaults; coupled to struct but no linking comment. |

## Open Questions

| ID | Question |
|----|----------|
| Q1 | Was removal of `config unset` communicated to users? No deprecation notice found. |
| Q2 | Should `config check` apply defaults before validation? Currently an empty `path = ""` passes because defaults fill it in. |

---

## Strengths

- Clean separation: all config write/validation centralized in `internal/config/writer.go`
- 18 test functions in writer_test.go with good table-driven coverage
- Proper validation pipeline in `Set` (parse -> read -> override -> unmarshal -> defaults -> validate -> write)
- `Check` correctly uses direct TOML unmarshal to catch syntax errors viper would ignore
- `bindFlagsToConfig` uses `Changed` guard to avoid clobbering config values
- `toml` struct tags added alongside `mapstructure` for dual compatibility

**Source**: internal-review.md (code-reviewer agent)
