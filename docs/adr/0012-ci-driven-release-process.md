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
2. Runs `changie batch <bump>` (writes `.changes/v<version>.md`) and `changie merge` (updates `CHANGELOG.md`)
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
4. Run goreleaser with `--release-notes=.changes/<version>.md` (currently mismatched with changie's `v`-prefixed filename, so releases ship with empty notes; see #269)

Here `<version>` is the bare semver (e.g. `0.6.2`); tags and changie version files carry the `v` prefix.

This closes the reviewability and silent-hook gaps above. Tag-before-build remains: the tag is pushed before CI builds anything. That is accepted, mitigated by `release` checking the CHANGELOG entry locally and CI checking it again before goreleaser runs.

## Alternatives rejected

**`VERSION` file on `main` as the trigger (this ADR's original decision).** Rejected because it duplicates the version changie already tracks. Two sources of truth for the version can drift, and the file added a step to every release without adding information.

**Tag push with no reviewed release PR.** The original objection to tag-push triggers was that a tag could be pushed with no changelog entry or README update visible in a PR. The current process keeps the tag trigger but closes that gap: `pre-release` puts the changelog and README changes in a release PR, and `release` refuses to tag unless `@-` is `trunk()` and the CHANGELOG entry exists (i.e. the release PR has merged).

**CI commits the README nix-pin update.** Rejected because it produces a bot commit on main after every release and requires the workflow to have write access to branch content. Moving it into the local task keeps CI read-only (it publishes releases but never writes to branches) and makes the update reviewable in the PR.

**Keep `before.hooks` in goreleaser.** Rejected because hook failures are silent — goreleaser does not fail when a hook produces empty output. Explicit CI steps surface failures with visible logs and allow fail-fast before goreleaser starts.

## Colocated jj/git note

The repository uses jujutsu in colocated mode (`.jj` and `.git` coexist). `mise run release` creates and pushes the tag with `git` directly, because jj tag support is not yet first-class; git tag operations work normally against the shared object store. CI checks out a plain git clone with no jj involvement. jj does not display tags in `jj log`.
