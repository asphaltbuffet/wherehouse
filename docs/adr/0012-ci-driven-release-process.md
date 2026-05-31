# CI-driven release process

Releases are triggered by a change to the `VERSION` file on `main`, not by pushing a git tag manually.

## Context

The previous process required a developer to run `mise run release <bump>` locally, which computed the next semver, created an annotated git tag, and pushed it. GitHub Actions triggered on `push: tags: v*` and ran goreleaser.

This had several gaps:
- Features and fixes accumulated between manual tag pushes
- The release tag was created before CI validated the build
- `before.hooks` in goreleaser ran silently; failures produced empty release notes without surfacing an error (confirmed: all releases v0.1.0–v0.4.0 have empty release bodies)
- No single PR showed the complete release intent (version bump + changelog + README pin) for review

## Decision

### Trigger

The release workflow triggers on `push: branches: [main], paths: [VERSION]` plus `workflow_dispatch` as a manual fallback. Tags are created by CI as an *output* of the release process, not used as an *input* to trigger it.

### Local task (`mise run release <major|minor|patch>`)

Runs locally before opening a PR. Responsibilities:
1. Read current version from `VERSION`
2. Compute next version via `semver bump`
3. Assert new version > current version
4. Assert `## [<new-version>]` entry exists in `CHANGELOG.md`
5. Write new version to `VERSION`
6. Update nix flake pin in `README.md` (`github:asphaltbuffet/wherehouse/v<old>` → `v<new>`)

The PR diff then shows all three release artifacts (VERSION, CHANGELOG entry, README pin) together, making release intent reviewable before anything fires.

### CI workflow steps

1. `mise run generate`
2. `mise run completions`
3. `mise run manpages`
4. `mise run pre-release` (reads `VERSION`, extracts changelog section → `release_notes/notes.md`)
5. Assert `release_notes/notes.md` is non-empty (`test -s`)
6. Assert `## [<version>]` entry exists in `CHANGELOG.md`
7. Assert tag `v<version>` does not already exist (guard against double-runs)
8. Create and push annotated tag `v$(cat VERSION)`
9. Run goreleaser (`--release-notes=release_notes/notes.md`)

### `mise run pre-release`

Reads `VERSION` directly — no argument. Callable identically from CI and locally. The README nix-pin update moves out of this task into `mise run release`.

## Alternatives rejected

**Keep tag-push as the trigger.** Rejected because it decouples the version bump from the PR review cycle — a tag can be pushed without a corresponding changelog entry or README update being visible in a PR diff.

**CI commits the README nix-pin update.** Rejected because it produces a bot commit on main after every release and requires the workflow to have write access to branch content. Moving it into the local task keeps CI read-only (except for tag creation) and makes the update reviewable in the PR.

**Keep `before.hooks` in goreleaser.** Rejected because hook failures are silent — goreleaser does not fail when a hook produces empty output. Explicit CI steps surface failures with visible logs and allow fail-fast before goreleaser starts.

## Colocated jj/git note

The repository uses jujutsu in colocated mode (`.jj` and `.git` coexist). Git tag operations work normally against the shared object store. CI checks out a plain git clone with no jj involvement. jj does not display tags in `jj log`, but that is an existing limitation unaffected by this change.
