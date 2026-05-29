# Task Completion

Run in order before every commit:

1. `mise run lint`   — golangci-lint --fix (must be clean)
2. `mise run test`   — gotestsum with race detector (must pass)
3. `/pre-commit` skill
4. `/commit` skill for message (conventional commits: `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`)
5. `/audit-docs` after features or fixes
