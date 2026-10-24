# Release Notes Path Fix Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make GitHub releases get changie release notes by fixing the `--release-notes` path in the release workflow, failing fast when the notes file is missing, and backfilling the ten releases that shipped with empty bodies (#269).

**Architecture:** Task 1 changes `.github/workflows/release.yml`: goreleaser's notes path uses the full tag name (`v0.6.2`) instead of the bare semver, and a new step before goreleaser fails if that file is missing or empty. ADR 0012 is updated in the same change. Task 2 is a one-off `gh release edit` loop run after Task 1 reaches `main`. CI can't run locally, so each workflow change is checked by running its exact shell logic locally against the real `.changes/` files, plus `actionlint`.

**Tech Stack:** GitHub Actions, goreleaser-pro, changie, `gh` CLI, `actionlint`, jujutsu (`jj`).

**Spec:** Triage comment on #269, https://github.com/asphaltbuffet/wherehouse/issues/269#issuecomment-6387645862 (issue body: https://github.com/asphaltbuffet/wherehouse/issues/269)

## Global Constraints

- Notes path: `--release-notes=".changes/${{ github.ref_name }}.md"`.
- Keep `steps.version.outputs.version` (bare semver) for the CHANGELOG check; `CHANGELOG.md` headings are `## [0.6.2]`.
- New step, placed after "Assert CHANGELOG entry exists" and before goreleaser: `test -s ".changes/${GITHUB_REF_NAME}.md" || { echo "Error: .changes/${GITHUB_REF_NAME}.md missing or empty"; exit 1; }`
- Backfill exactly: `v0.1.0 v0.2.0 v0.3.0 v0.4.0 v0.5.0 v0.5.1 v0.5.2 v0.5.3 v0.6.0 v0.6.2`. **Not v0.6.1** (it has no GitHub release; out of scope).
- Don't change `.changie.yaml`. The workflow adapts to changie's `v`-prefixed filenames.
- VCS is `jj`. Never run `git` commands.
- This stack sits on unpushed change `uuwmlpss` (ADR 0012 amendment for #266). Task 1 edits text that exists only there, so build on top of it.

## Review Focus

1. **Tag with no changie file** (e.g. a hand-pushed `v9.9.9`): the job should fail before goreleaser with a clear message instead of publishing an empty release. Pinned by Task 1, Step 4.
2. **Notes file exists but is empty:** this must also fail, which is why the check is `test -s`, not `test -f`. Pinned by Task 1, Step 4.
3. **CHANGELOG check regressing to the `v` form:** someone "fixing" both paths would change the CHANGELOG grep to `## [v0.6.2]` and break every release. The CHANGELOG step must keep the bare version. Pinned by Task 1, Step 4.
4. **Backfill hitting a tag with no release** (v0.6.1): `gh release edit` would error mid-loop. The list excludes it, and the loop checks each release exists before editing. Pinned by Task 2, Steps 1 and 3.
5. **Backfill overwriting notes someone fixed by hand:** releases whose body is already longer than 1 character must be skipped, not overwritten. Pinned by Task 2, Step 3.

---

### Task 1: Fix the workflow notes path, add the check, update ADR 0012

**Files:**
- Modify: `.github/workflows/release.yml` (the "Assert CHANGELOG entry exists" step around lines 48–55; the "Run GoReleaser" step's `args:` around line 75)
- Modify: `docs/adr/0012-ci-driven-release-process.md` ("CI workflow steps" list, around lines 44–49)

**Interfaces:**
- Consumes: nothing from other tasks.
- Produces: `main`'s `release.yml` resolves release notes to `.changes/<tag>.md`. Task 2's precondition checks for exactly the string `--release-notes=".changes/${{ github.ref_name }}.md"`.

- [ ] **Step 1: Start a fresh jj change on top of the #266 stack**

Run: `jj log --no-graph -r '@ | @-' -T 'change_id.short() ++ " " ++ description.first_line() ++ "\n"'`
Expected: `@-` is `uuwmlpss docs(adr): amend 0012 …` (or a descendant of it). If `@` isn't empty, run `jj new`. Then:

```bash
jj describe -m "fix(ci): pass tag-named changie file as release notes

The workflow stripped the v prefix and passed .changes/0.6.2.md, but
changie writes .changes/v0.6.2.md, so every release shipped with an
empty body. Use github.ref_name for the path and fail before goreleaser
if the notes file is missing or empty.

Refs #269"
```

- [ ] **Step 2: Write the failing checks**

Save as `$SCRATCH/check-notes.sh`, where `$SCRATCH` is the session scratchpad. This file is not committed.

```bash
#!/usr/bin/env bash
# Resolves the workflow's --release-notes path for tag v0.6.2 the way CI would.
set -uo pipefail
wf=.github/workflows/release.yml
p=$(rg -o --replace '$1' -- '--release-notes="([^"]+)"' "$wf")
p=${p//'${{ github.ref_name }}'/v0.6.2}
p=${p//'${{ steps.version.outputs.version }}'/0.6.2}
echo "notes path: $p"
test -s "$p" && echo "NOTES: OK" || echo "NOTES: MISSING"
rg -q 'name: Assert release notes file exists' "$wf" && echo "GUARD: present" || echo "GUARD: absent"
```

- [ ] **Step 3: Run it to verify it fails**

Run: `bash $SCRATCH/check-notes.sh`
Expected:
```
notes path: .changes/0.6.2.md
NOTES: MISSING
GUARD: absent
```

- [ ] **Step 4: Write the check's own tests (Review Focus 1–3)**

Run each from the repo root. These test the exact shell logic the new step will run:

```bash
# RF1: tag with no changie file → must fail
GITHUB_REF_NAME=v9.9.9 bash -c 'test -s ".changes/${GITHUB_REF_NAME}.md" || { echo "Error: .changes/${GITHUB_REF_NAME}.md missing or empty"; exit 1; }'; echo "exit=$?"
```
Expected: `Error: .changes/v9.9.9.md missing or empty` then `exit=1`

```bash
# RF2: empty notes file → must fail
T=$(mktemp -d); mkdir "$T/.changes"; : > "$T/.changes/v0.0.1.md"
(cd "$T" && GITHUB_REF_NAME=v0.0.1 bash -c 'test -s ".changes/${GITHUB_REF_NAME}.md" || { echo "Error: .changes/${GITHUB_REF_NAME}.md missing or empty"; exit 1; }'); echo "exit=$?"; rm -r "$T"
```
Expected: `Error: .changes/v0.0.1.md missing or empty` then `exit=1`

```bash
# Happy path: real tag → passes
GITHUB_REF_NAME=v0.6.2 bash -c 'test -s ".changes/${GITHUB_REF_NAME}.md" || { echo "Error: .changes/${GITHUB_REF_NAME}.md missing or empty"; exit 1; }'; echo "exit=$?"
```
Expected: `exit=0`

```bash
# RF3: CHANGELOG headings are bare → the CHANGELOG step must keep the bare version
rg -c '^## \[0\.6\.2\]' CHANGELOG.md; rg -c '^## \[v0\.6\.2\]' CHANGELOG.md; echo "exit=$?"
```
Expected: `1`, then nothing for the second pattern, then `exit=1` (the bare heading exists; the `v` heading doesn't).

- [ ] **Step 5: Edit the workflow**

In `.github/workflows/release.yml`, add this step directly after the "Assert CHANGELOG entry exists" step (after its `fi` line, before `- name: Set up QEMU`), with the same indentation as the other steps:

```yaml
      - name: Assert release notes file exists
        run: |
          test -s ".changes/${GITHUB_REF_NAME}.md" || { echo "Error: .changes/${GITHUB_REF_NAME}.md missing or empty"; exit 1; }
```

(`GITHUB_REF_NAME` is the runner's environment variable, not a `${{ }}` expression, so the tag name never gets interpolated into the script text.)

In the "Run GoReleaser" step, replace:

```yaml
          args: release --clean --release-notes=".changes/${{ steps.version.outputs.version }}.md"
```

with:

```yaml
          args: release --clean --release-notes=".changes/${{ github.ref_name }}.md"
```

Leave the "Assert CHANGELOG entry exists" step exactly as it is.

- [ ] **Step 6: Run the checks to verify they pass**

Run: `bash $SCRATCH/check-notes.sh`
Expected:
```
notes path: .changes/v0.6.2.md
NOTES: OK
GUARD: present
```

Run: `rg -n 'VERSION: \$\{\{ steps.version.outputs.version \}\}' .github/workflows/release.yml`
Expected: one match (the CHANGELOG step still uses the bare version).

Run: `actionlint .github/workflows/release.yml; echo "exit=$?"`
Expected: no findings, `exit=0`.

- [ ] **Step 7: Update ADR 0012**

In `docs/adr/0012-ci-driven-release-process.md`, replace:

```markdown
3. Fail unless `CHANGELOG.md` has a `## [<version>]` entry
4. Run goreleaser with `--release-notes=.changes/<version>.md` (currently mismatched with changie's `v`-prefixed filename, so releases ship with empty notes; see #269)
```

with:

```markdown
3. Fail unless `CHANGELOG.md` has a `## [<version>]` entry
4. Fail unless `.changes/v<version>.md` exists and is non-empty
5. Run goreleaser with `--release-notes=.changes/v<version>.md`
```

Leave the next paragraph ("Here `<version>` is the bare semver …") and the tag-before-build paragraph unchanged.

Run: `rg -n '#269|currently mismatched|release-notes=\.changes/<version>' docs/adr/0012-ci-driven-release-process.md; echo "exit=$?"`
Expected: no matches, `exit=1`.

Run: `rg -n 'release-notes=\.changes/v<version>\.md|exists and is non-empty' docs/adr/0012-ci-driven-release-process.md`
Expected: two matches (steps 4 and 5).

- [ ] **Step 8: Run lint and tests**

Run: `mise run lint`
Expected: `0 issues.`

Run: `mise run test`
Expected: `DONE 893 tests, 2 skipped` (or the current count) with no failures.

- [ ] **Step 9: Commit**

```bash
jj st
jj new
```

Expected: `jj st` lists exactly `M .github/workflows/release.yml` and `M docs/adr/0012-ci-driven-release-process.md`.

---

### Task 2: Backfill notes on the ten empty releases

**Outward-facing:** this edits published GitHub releases. Run it only after Task 1's change has been pushed and merged to `main`, and only with the maintainer's go-ahead in the executing session.

**Files:** none in the repo. Reads `.changes/v*.md`; writes GitHub release bodies.

**Interfaces:**
- Consumes: Task 1 merged to `main` (`main`'s `release.yml` contains `--release-notes=".changes/${{ github.ref_name }}.md"`).
- Produces: nothing other tasks rely on.

- [ ] **Step 1: Check preconditions (RED)**

```bash
gh api repos/asphaltbuffet/wherehouse/contents/.github/workflows/release.yml --jq '.content' | base64 -d | rg -c 'release-notes="\.changes/\$\{\{ github\.ref_name \}\}\.md"'
```
Expected: `1`. If it prints nothing, Task 1 isn't on `main` yet. Stop and report.

```bash
for t in v0.1.0 v0.2.0 v0.3.0 v0.4.0 v0.5.0 v0.5.1 v0.5.2 v0.5.3 v0.6.0 v0.6.2; do
  printf '%s release_body=%s notes_file=%s\n' "$t" \
    "$(gh release view "$t" --repo asphaltbuffet/wherehouse --json body --jq '.body|length' 2>/dev/null || echo NONE)" \
    "$( [ -s ".changes/$t.md" ] && echo ok || echo MISSING )"
done
```
Expected: each line shows `release_body=1 notes_file=ok`. A `NONE` or `MISSING` means that tag is outside the spec's assumptions: drop it from the loop and report it. A body greater than 1 is handled by Step 3's skip.

- [ ] **Step 2: Confirm with the maintainer**

Show the Step 1 output and ask for a go-ahead before editing any release.

- [ ] **Step 3: Backfill**

```bash
for t in v0.1.0 v0.2.0 v0.3.0 v0.4.0 v0.5.0 v0.5.1 v0.5.2 v0.5.3 v0.6.0 v0.6.2; do
  len=$(gh release view "$t" --repo asphaltbuffet/wherehouse --json body --jq '.body|length' 2>/dev/null) || { echo "$t: no release, skipped"; continue; }
  if [ "$len" -gt 1 ]; then echo "$t: body already set ($len chars), skipped"; continue; fi
  [ -s ".changes/$t.md" ] || { echo "$t: notes file missing, skipped"; continue; }
  gh release edit "$t" --repo asphaltbuffet/wherehouse --notes-file ".changes/$t.md" >/dev/null && echo "$t: updated"
done
```
Expected: `<tag>: updated` for all ten.

- [ ] **Step 4: Verify (GREEN)**

```bash
for t in v0.1.0 v0.2.0 v0.3.0 v0.4.0 v0.5.0 v0.5.1 v0.5.2 v0.5.3 v0.6.0 v0.6.2; do
  printf '%s len=%s first=%s\n' "$t" \
    "$(gh release view "$t" --repo asphaltbuffet/wherehouse --json body --jq '.body|length')" \
    "$(gh release view "$t" --repo asphaltbuffet/wherehouse --json body --jq '.body' | head -1)"
done
```
Expected: every `len` is greater than 1, and every `first` is that version's changie heading, e.g. `v0.6.2 len=… first=## [0.6.2] - 2026-10-24`.

- [ ] **Step 5: Close out**

Comment on #269 with the Step 4 output and close it as completed. (If Task 1's PR body says `Closes #269`, the issue may already be closed; comment anyway.)
