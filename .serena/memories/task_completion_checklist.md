# Task Completion Checklist

When a task is complete, always do the following in order:

## 1. Verify
- All warnings treated as errors: fix all warnings from `mise run lint`, `mise run test`, compiler
- Run `mise run test` — all tests pass
- Run `mise run lint` — no lint errors
- Run `mise run build` — binary builds cleanly

## 2. Pre-commit
- Run `/pre-commit` skill before every commit

## 3. Commit
- Run `/commit` skill for commit message conventions
- Use conventional commits: `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`
- CI trigger phrases go in commit body if needed

## 4. Docs audit
- Run `/audit-docs` after features or fixes

## Hard Rules (Non-Negotiable)
- No `git` commands — use `jj`
- No `&&` between shell commands — run as separate tool calls
- Use `rg` not `grep`, `fd` not `find`, `sd` not `sed`
- Pin GitHub Actions to version tags (e.g. `@v3.93.1`), not `@main`/`@latest`
- Two-strike rule for bug fixes: if second attempt fails, stop and re-read end-to-end
