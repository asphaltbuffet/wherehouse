# Task Completion Checklist

Run in order before committing:

1. `mise run lint` — fix all lint warnings
2. `mise run test` — all tests pass (race detector on)
3. `/pre-commit` skill — final pre-commit check
4. `/commit` skill — conventional commit message (`feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`)
5. `/audit-docs` — after features or fixes
6. Deferred work → GitHub Issues at `github.com/asphaltbuffet/wherehouse` (not TODOs in code)
