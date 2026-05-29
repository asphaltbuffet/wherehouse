# Task Completion

Run before every commit:
1. `mise run lint` — fix all warnings
2. `mise run test` — all tests pass
3. `/pre-commit` skill
4. `/commit` skill — conventional commits: `feat:`, `fix:`, `chore:`, `docs:`, `refactor:`, `test:`

Post-feature/fix:
5. `/audit-docs`

Deferred work → GitHub Issues at `github.com/asphaltbuffet/wherehouse`, not TODOs in code/CLAUDE.md.
