Create a commit per project guidelines.

> **Project VCS**: See `.claude/project-config.md` → Version Control for VCS tool and branch info.

# Before Committing

You must run `/pre-commit` first. Every step **must** pass cleanly.

# Commit Message Format

Use conventional commits with scopes:

```
<type>(<scope>): <description>

[optional body]
```

## Type rules

- `feat:` — new/updated user-facing functionality
- `fix:` — user-facing defect fix only. Never use for commits that only change a test
- `test:` or `chore(test):` — test-only changes
- `ci:` — CI workflow changes. Never use `fix:`
- `docs:` — documentation changes. Never for changes only in `ai-docs/`
- `chore:` — maintenance, deps, tooling
- `ai:` — changes that only affect `ai-docs/` or `.claude/`

## Avoid CI trigger phrases

The tokens `[skip ci]`, `[ci skip]`, `[no ci]`, `[skip actions]`, and `[actions skip]` suppress CI runs. Never include them in commit messages.
