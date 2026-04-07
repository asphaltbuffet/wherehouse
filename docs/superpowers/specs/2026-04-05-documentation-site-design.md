# Documentation Site Design

**Date**: 2026-04-05  
**Status**: Approved  
**Audience**: Small public audience (GitHub project)

---

## Summary

Replace the single hand-maintained `README.md` with a three-layer documentation system:

1. **Auto-generated command reference** — cobra generates markdown from the live command tree
2. **Terminal recordings** — VHS renders `.tape` scripts into GIFs stored in Git LFS
3. **MkDocs Material site** — hosted on GitHub Pages, deployed on every merge to `main`

---

## Architecture

Three layers, each with a different update mechanism:

| Layer | Source | Update trigger |
|---|---|---|
| Command reference | cobra command tree | Any `cmd/**/*.go` change |
| Terminal recordings | `.tape` scripts + fixture DB | Manual tape update |
| Narrative pages | Hand-authored markdown | Manual edit |

---

## Components

### 1. Cobra Markdown Generation

A hidden `docs` subcommand (matching the existing `man` subcommand pattern) calls `cobra/doc.GenMarkdownTree(rootCmd, "./docs/reference/")`. Invoked as `go run . docs` in CI.

- Output: `docs/reference/*.md` (one file per command/subcommand)
- Gitignored — regenerated on every CI run
- New mise task: `mise run docs-gen`

### 2. VHS Terminal Recordings

Tape scripts in `docs/tapes/*.tape` cover key workflows:

- `add-item.tape` — adding items and locations
- `find.tape` — searching with various flags
- `history.tape` — event timeline display
- `move.tape` — moving items between locations

Each tape references a fixed SQLite fixture at `docs/tapes/fixtures/wherehouse.db` via `--db`, ensuring deterministic output.

Output GIFs land in `docs/assets/recordings/`. These are tracked in Git LFS (`.gitattributes` entries for `*.gif` and `*.webp`). CI commits updated GIFs back to `main` when they change.

### 3. MkDocs Material Site

`mkdocs.yml` at the repo root. Python dependencies pinned in `docs/requirements.txt`.

**Directory structure:**

```
docs/
  index.md              # landing page (narrative, hand-authored)
  quickstart.md         # hand-authored
  configuration.md      # hand-authored
  reference/            # cobra-generated, gitignored
    wherehouse.md
    wherehouse_add.md
    wherehouse_find.md
    ...
  assets/
    recordings/         # VHS output, Git LFS
      add-item.gif
      find.gif
      ...
  tapes/
    fixtures/
      wherehouse.db     # committed fixture database
    add-item.tape
    find.tape
    history.tape
    move.tape
  requirements.txt      # pinned mkdocs-material==X.Y.Z
```

---

## CI Workflow

New `docs.yml` GitHub Actions workflow, triggers on push to `main`:

```
permissions:
  contents: write   # for LFS asset commit-back
  pages: write      # for gh-pages deploy

steps:
1. checkout (LFS enabled, full history)
2. setup-go → go run . docs              # cobra → docs/reference/*.md
3. setup-python → pip install -r docs/requirements.txt
4. vhs (ghcr.io/charmbracelet/vhs)       # render *.tape → docs/assets/recordings/*.gif
5. if GIFs changed: git add + commit + push (LFS)
6. mkdocs gh-deploy --force              # built HTML → gh-pages branch
```

Steps 5 and 6 are independent targets: LFS assets go back to `main`, built HTML goes to `gh-pages`.

---

## Local Development

Two new mise tasks:

| Task | What it does | Requires |
|---|---|---|
| `mise run docs-gen` | cobra markdown generation only | Go |
| `mise run docs` | docs-gen + `mkdocs serve` (live preview) | Go + Python |

---

## Error Handling & Maintenance

**Command added/changed**: Reference docs update automatically next CI run. No manual steps unless a recording is wanted — add a `.tape` file in that case.

**Tape breaks**: VHS failures fail CI loudly. Root causes are limited: output format changed, command renamed, or VHS version bump. Fix: update the tape script.

**Fixture database**: Committed to `docs/tapes/fixtures/`. Small, known dataset. Recreate with `wherehouse initialize database` + a few `add` commands if schema migrations break it.

**MkDocs upgrades**: Handled by pinning in `docs/requirements.txt`. Upgrade by bumping the pin and verifying the site builds.

**Narrative pages**: Still manual — same as today, but now scoped to focused files. The `/audit-docs` hook in the development workflow catches stale narrative content after features or fixes.

---

## What Is Not Automated

- Narrative pages (`index.md`, `quickstart.md`, `configuration.md`)
- Adding new `.tape` scripts when new commands are implemented
- Fixture database recreation after breaking schema migrations

---

## Dependencies Added

| Dependency | Where | Purpose |
|---|---|---|
| `github.com/spf13/cobra/doc` | Go (build-time only) | Markdown generation |
| `mkdocs-material` | Python (CI + local) | Site generation |
| `ghcr.io/charmbracelet/vhs` | CI Docker action | Terminal recordings |
| Git LFS | Repo + CI | Binary asset versioning |
