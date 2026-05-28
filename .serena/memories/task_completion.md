# Task Completion Checklist

Run these before every commit:
1. `mise run lint` — fix all warnings
2. `mise run test` — all tests pass with race detector
3. Run `/pre-commit` skill
4. Run `/commit` skill (conventional commits: `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`)
5. Run `/audit-docs` after features or fixes
