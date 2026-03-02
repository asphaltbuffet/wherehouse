Run this pre-commit checklist before committing. All steps **must** pass cleanly. Fix issues inline.

> **Project tooling**: See `.claude/project-config.md` → Build & Tooling and → Version Control for commands.

1. **Check for edited files**: `jj log -n1 --no-graph -T 'if(empty, "FAIL", "OK")'`
2. **Check if already committed**: `jj log -n1 --no-graph -T 'coalesce(description, "OK")'` — if not `OK`, **STOP**. The changes are already part of a commit.
3. **Run full pipeline**: `mise run dev` — test/lint/build with all necessary steps

Do not proceed to commit until every step passes cleanly. No exceptions.
