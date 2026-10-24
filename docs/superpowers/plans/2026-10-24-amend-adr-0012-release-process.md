# Amend ADR 0012 (Release Process) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Rewrite `docs/adr/0012-ci-driven-release-process.md` in place so it describes the release process the repo actually uses (changie + local tag push), closing #266.

**Architecture:** Docs-only. One file is rewritten; no code, scripts, workflows, or other docs change. The new ADR text is given in full below. The work is to drop it in, then check every factual claim against the scripts and workflow it describes.

**Tech Stack:** Markdown; jujutsu (`jj`) for VCS; `rg` for verification.

**Spec:** Triage comment on #266, https://github.com/asphaltbuffet/wherehouse/issues/266#issuecomment-6387517457

## Global Constraints

- Amend 0012 **in place**: keep the filename `docs/adr/0012-ci-driven-release-process.md` and don't create a new ADR. (ADRs here have no status lines and nothing has been superseded before.)
- Rationale for the switch, verbatim from the maintainer: changie made `.changes/` the single source of truth for versions, so a separate `VERSION` file was redundant.
- Don't duplicate the README's step-by-step; link to it (`../../README.md#releasing`).
- Keep the original Context gaps list, and keep the "CI commits the README nix-pin update" and "Keep `before.hooks`" rejected alternatives as they are written today, apart from one parenthetical: CI no longer creates tags.
- VCS is `jj`. Never run `git` commands.
- Don't touch `.mise/tasks/*`, `.github/workflows/*`, `README.md`, or `CHANGELOG.md`. If the ADR and those files disagree, the ADR is wrong.
- No changie fragment: README says fragments are optional for docs-only changes.

## Review Focus

1. **Claims that don't match the scripts:** every step listed under Decision must match `.mise/tasks/pre-release`, `.mise/tasks/release`, and `.github/workflows/release.yml` as they are today (e.g. `release` checks for a clean working copy, `@-` == `trunk()`, the CHANGELOG entry, and that the tag doesn't exist). Pinned by Task 1, Step 3.
2. **Leftover contradiction:** the decision is tag-push, so the Alternatives section must not still reject tag-push. Pinned by Task 1, Step 4.
3. **Stale artifacts:** the ADR must not present `VERSION`, `release_notes/notes.md`, `workflow_dispatch`, or CI-created tags as current behavior. Pinned by Task 1, Step 4.
4. **Broken README link:** the relative link `../../README.md#releasing` must resolve; the heading is `### Releasing`. Pinned by Task 1, Step 4.
5. **Lost history:** a reader must still see that a `VERSION` trigger existed and why it was dropped, so the old decision isn't erased. Pinned by Task 1, Step 4.

---

### Task 1: Rewrite ADR 0012

**Files:**
- Modify: `docs/adr/0012-ci-driven-release-process.md` (full rewrite)
- Read only (sources of truth): `.mise/tasks/pre-release`, `.mise/tasks/release`, `.github/workflows/release.yml`, `README.md` (`### Releasing` section, around line 485)

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces: nothing other tasks rely on.

- [ ] **Step 1: Start a fresh jj change**

Run: `jj st`
If the working copy has changes or a description, run `jj new` first. Then:

```bash
jj describe -m "docs(adr): amend 0012 to describe changie + tag-push release

Closes #266"
```

- [ ] **Step 2: Replace the file contents**

Overwrite `docs/adr/0012-ci-driven-release-process.md` with exactly:

````markdown
# Changie-driven release with local tag push

Releases are cut by pushing a `v<version>` tag from trunk after a reviewed release PR merges. The version comes from changie (`.changes/`); there is no `VERSION` file.

## Context

An earlier process required a developer to run `mise run release <bump>` locally, which computed the next semver, created an annotated git tag, and pushed it. GitHub Actions triggered on `push: tags: v*` and ran goreleaser.

This had several gaps:
- Features and fixes accumulated between manual tag pushes
- The release tag was created before CI validated the build
- `before.hooks` in goreleaser ran silently; failures produced empty release notes without surfacing an error (confirmed: all releases v0.1.0–v0.4.0 have empty release bodies)
- No single PR showed the complete release intent (version bump + changelog + README pin) for review

The first version of this ADR answered those gaps with a `VERSION` file on `main`: changing it triggered CI, which created the tag. That was dropped shortly afterwards. Once changie was adopted, `.changes/` became the single source of truth for versions, and a separate `VERSION` file only duplicated it.

## Decision

The step-by-step procedure lives in the [README "Releasing" section](../../README.md#releasing). This ADR records the shape of the process and why.

### Trigger

The release workflow (`.github/workflows/release.yml`) triggers on `push: tags: v*`. The tag is pushed locally by `mise run release`, never by CI.

### `mise run pre-release <major|minor|patch>`

Prepares the release PR:
1. Fails if `.changes/unreleased/` has no fragments
2. Runs `changie batch <bump>` (writes `.changes/<version>.md`) and `changie merge` (updates `CHANGELOG.md`)
3. Fails unless `CHANGELOG.md` now has a `## [<version>]` entry
4. Updates the nix flake pins in `README.md` from the previous version to the new one

The resulting PR diff shows every release artifact (`.changes/`, `CHANGELOG.md`, README pins) together, so release intent is reviewed before anything is published.

### `mise run release`

Run locally after the release PR merges. Takes no argument:
1. Fails if the jj working copy has changes, or if `@-` is not `trunk()`
2. Reads the version from `changie latest`
3. Fails unless `CHANGELOG.md` has a `## [<version>]` entry
4. Fails if tag `v<version>` already exists
5. Creates an annotated tag on trunk and pushes it

### CI workflow steps

1. Extract the version from the tag name
2. `mise run generate`, `mise run completions`, `mise run manpages`
3. Fail unless `CHANGELOG.md` has a `## [<version>]` entry
4. Run goreleaser with `--release-notes=.changes/<version>.md`

## Alternatives rejected

**`VERSION` file on `main` as the trigger (this ADR's original decision).** Rejected because it duplicates the version changie already tracks. Two sources of truth for the version can drift, and the file added a step to every release without adding information.

**Tag push with no reviewed release PR.** The original objection to tag-push triggers was that a tag could be pushed with no changelog entry or README update visible in a PR. The current process keeps the tag trigger but closes that gap: `pre-release` puts the changelog and README changes in a release PR, and `release` refuses to tag unless that PR has merged to trunk and the CHANGELOG entry exists.

**CI commits the README nix-pin update.** Rejected because it produces a bot commit on main after every release and requires the workflow to have write access to branch content. Moving it into the local task keeps CI read-only (it publishes releases but never writes to branches) and makes the update reviewable in the PR.

**Keep `before.hooks` in goreleaser.** Rejected because hook failures are silent — goreleaser does not fail when a hook produces empty output. Explicit CI steps surface failures with visible logs and allow fail-fast before goreleaser starts.

## Colocated jj/git note

The repository uses jujutsu in colocated mode (`.jj` and `.git` coexist). `mise run release` creates and pushes the tag with `git` directly, because jj tag support is not yet first-class; git tag operations work normally against the shared object store. CI checks out a plain git clone with no jj involvement. jj does not display tags in `jj log`.
````

(The "CI commits the README nix-pin update" paragraph has one deliberate wording change: "except for tag creation" became "it publishes releases but never writes to branches", because CI no longer creates tags.)

- [ ] **Step 3: Check each Decision claim against its source**

Each command must print a match; if one doesn't, fix the ADR (not the script):

```bash
rg -n "tags:" -A1 .github/workflows/release.yml                      # v* tag trigger
rg -n "unreleased|changie batch|changie merge|github:asphaltbuffet" .mise/tasks/pre-release
rg -n "jj diff -s|trunk\(\)|changie latest|git tag|git push" .mise/tasks/release
rg -n "GITHUB_REF_NAME|mise run (generate|completions|manpages)|CHANGELOG|release-notes" .github/workflows/release.yml
```

Expected: matches for the tag trigger; fragment check, batch, merge, and README pin `sd` in `pre-release`; clean-copy check, trunk check, `changie latest`, tag-exists check, `git tag`/`git push` in `release`; version extraction, the three generate steps, the CHANGELOG check, and `--release-notes=".changes/...` in the workflow.

- [ ] **Step 4: Check the Review Focus conditions**

```bash
rg -n "workflow_dispatch|release_notes/|cat VERSION|paths: \[VERSION\]" docs/adr/0012-ci-driven-release-process.md
```
Expected: no output (no old artifacts presented as current).

```bash
rg -n "VERSION" docs/adr/0012-ci-driven-release-process.md
```
Expected: matches only in the lead sentence ("there is no `VERSION` file"), the last Context paragraph, and the first Alternatives entry.

```bash
rg -n "Keep tag-push as the trigger" docs/adr/0012-ci-driven-release-process.md
```
Expected: no output (the old rejection of tag-push is gone).

```bash
rg -n "except for tag creation" docs/adr/0012-ci-driven-release-process.md
```
Expected: no output.

```bash
rg -n "^### Releasing$" README.md
```
Expected: exactly one match, so `#releasing` resolves.

- [ ] **Step 5: Run lint and tests**

Run: `mise run lint`
Expected: `0 issues.`

Run: `mise run test`
Expected: `DONE ... tests` with no failures. (Docs-only change; this confirms nothing else broke.)

- [ ] **Step 6: Commit**

The change was described in Step 1. Close it off:

```bash
jj st
jj new
```

Expected: `jj st` shows only `M docs/adr/0012-ci-driven-release-process.md`. If anything else appears, move it out of this change before running `jj new`.
