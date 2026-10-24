# Container Image Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Publish a multi-arch `wherehouse serve` container image to GHCR as part of the existing goreleaser release.

**Architecture:** Goreleaser's `dockers_v2` builds one image from the already-built (UPX'd) linux binaries, using a GoReleaser-only `goreleaser.Dockerfile` on distroless static. All runtime behaviour (DB path, logs, config) is set through existing `WHEREHOUSE_*` env vars and `WORKDIR` — no Go changes. CI gains buildx/QEMU and a GHCR login.

**Tech Stack:** goreleaser-pro v2 (`dockers_v2`), Docker buildx, `gcr.io/distroless/static-debian13:nonroot`, GitHub Actions, GHCR.

**Spec:** `docs/superpowers/specs/2026-10-24-container-image-design.md`

## Global Constraints

- VCS is jujutsu: `jj new` before each task's edits; `jj commit -m` to finish. Never run `git`.
- Run `/pre-commit` before every commit and `/commit` for the message (conventional commits).
- Only use goreleaser features available in **v2.16** (local `goreleaser-pro 2.16.0`): no `templated_dockerfile`, `templated_extra_files`, `hooks.*.if`, or scoped annotation keys.
- Image: `ghcr.io/asphaltbuffet/wherehouse`. Platforms: `linux/amd64`, `linux/arm64`.
- Tags: `{{ .Version }}`, `{{ if not .Prerelease }}{{ .Major }}.{{ .Minor }}{{ end }}`, `{{ if not .Prerelease }}latest{{ end }}`. No `v` prefix.
- Base: `gcr.io/distroless/static-debian13:nonroot`; runtime uid/gid `65532:65532`.
- Env: `WHEREHOUSE_DB_PATH=/data/wherehouse.db`, `WHEREHOUSE_LOGGING_FILE_PATH=/dev/stderr`. Do **not** set `WHEREHOUSE_USER_DEFAULT_IDENTITY`.
- `WORKDIR /data`, `VOLUME /data`, `EXPOSE 8080`, no `HEALTHCHECK`.
- Entrypoint: `["/usr/bin/wherehouse", "serve", "--bind", "0.0.0.0"]`, no `CMD`.
- No Go source changes in this plan.

## Review Focus

1. **Fresh named volume, first run** — `docker run -v wh:/data img` must create `/data/wherehouse.db` without a permission error (Task 1, Step 5).
2. **Bind-mounting a host dir owned by the host user** — fails with permission denied as nonroot; `--user "$(id -u):$(id -g)"` must make it work, and the README must say so (Task 1, Step 8; Task 3).
3. **Extra args after the image name** (`img --port 9090`) — must append to `serve --bind 0.0.0.0`, not replace it (Task 1, Step 6).
4. **Prerelease tag** — `vX.Y.Z-rc.N` must not produce `latest` or `X.Y` tags (Task 1, Step 9).
5. **arm64 image contains an arm64 binary** — `docker image inspect` architecture is `arm64` (Task 1, Step 7).

---

### Task 1: Dockerfile and goreleaser `dockers_v2`

**Files:**
- Create: `goreleaser.Dockerfile`
- Modify: `.goreleaser.yml` (append a `dockers_v2:` block after `nfpms:`)

**Interfaces:**
- Consumes: goreleaser build with default id `wherehouse` (no `id:` set in `builds`), binary name `wherehouse`.
- Produces: images `ghcr.io/asphaltbuffet/wherehouse:<tag>`; in `--snapshot` mode, per-platform local images `ghcr.io/asphaltbuffet/wherehouse:<tag>-amd64` / `-arm64`. Task 2 relies on `dockers_v2` running in the publish phase of `goreleaser release`.

- [ ] **Step 1: Start a fresh change**

Run: `jj new`

- [ ] **Step 2: Create `goreleaser.Dockerfile`**

```dockerfile
# Built by goreleaser (dockers_v2). The build context holds only the
# prebuilt binaries under $TARGETPLATFORM/, so `docker build .` from a
# checkout will not work.
FROM gcr.io/distroless/static-debian13:nonroot

ARG TARGETPLATFORM

# Distroless has no shell, so create a nonroot-owned /data by copying the
# base image's empty /tmp. Named volumes inherit this ownership on first use.
COPY --from=gcr.io/distroless/static-debian13:nonroot --chown=65532:65532 /tmp /data
COPY $TARGETPLATFORM/wherehouse /usr/bin/wherehouse

# /dev/stderr only works while logging.max_size_mb is 0 (rotation off).
# Replace once a real stderr sink lands: github.com/asphaltbuffet/wherehouse/issues/257
ENV WHEREHOUSE_DB_PATH=/data/wherehouse.db \
    WHEREHOUSE_LOGGING_FILE_PATH=/dev/stderr

USER 65532:65532
WORKDIR /data
VOLUME /data
EXPOSE 8080

ENTRYPOINT ["/usr/bin/wherehouse", "serve", "--bind", "0.0.0.0"]
```

- [ ] **Step 3: Append `dockers_v2` to `.goreleaser.yml`**

Add after the `nfpms:` block:

```yaml

dockers_v2:
  - id: wherehouse
    ids:
      - wherehouse
    dockerfile: goreleaser.Dockerfile
    images:
      - ghcr.io/asphaltbuffet/wherehouse
    tags:
      - "{{ .Version }}"
      - "{{ if not .Prerelease }}{{ .Major }}.{{ .Minor }}{{ end }}"
      - "{{ if not .Prerelease }}latest{{ end }}"
    platforms:
      - linux/amd64
      - linux/arm64
    labels:
      org.opencontainers.image.title: "{{ .ProjectName }}"
      org.opencontainers.image.description: Keep track of all your things
      org.opencontainers.image.version: "{{ .Version }}"
      org.opencontainers.image.revision: "{{ .FullCommit }}"
      org.opencontainers.image.created: "{{ .Date }}"
      org.opencontainers.image.source: https://github.com/asphaltbuffet/wherehouse
      org.opencontainers.image.licenses: MIT
```

`ids: [wherehouse]` keeps the nfpm packages (id `packages`) out of the build context. The `org.opencontainers.image.source` label is what links the GHCR package to the repo.

- [ ] **Step 4: Validate config and build snapshot images**

Run: `goreleaser check`
Expected: exits 0, no deprecation warning for `dockers_v2`.

Run (snapshot release needs completions/manpages for nfpm):
```bash
mise run completions
mise run manpages
goreleaser release --snapshot --clean
```
Expected: exits 0; log shows docker images built for both platforms.

Run: `docker images ghcr.io/asphaltbuffet/wherehouse --format '{{.Tag}}'`
Expected: tags ending `-snapshot-amd64` and `-snapshot-arm64` (plus `<major>.<minor>-amd64`/`-arm64` and `latest-amd64`/`-arm64`). Set `TAG` to the `<version>-snapshot-amd64` value for the next steps, e.g. `TAG=0.6.2-snapshot-amd64`.

If buildx rejects multi-platform builds with the default driver, create a builder first and retry:
```bash
docker buildx create --name=goreleaser --use
docker run --privileged --rm tonistiigi/binfmt --install all
```

- [ ] **Step 5: Smoke-test a fresh named volume (Review Focus 1)**

```bash
IMG=ghcr.io/asphaltbuffet/wherehouse:$TAG
docker run -d --name wh-smoke -p 18080:8080 -v wh-smoke:/data "$IMG"
curl -fsS http://127.0.0.1:18080/healthz
docker logs wh-smoke
docker run --rm -v wh-smoke:/data busybox:1.37 ls -ln /data
```
Expected: `curl` exits 0; `docker logs` contains `Listening on http://0.0.0.0:8080`; `ls` shows `wherehouse.db` owned by `65532 65532`.

Check CLI writes land on the volume and the actor default:
```bash
docker run --rm -v wh-smoke:/data --entrypoint /usr/bin/wherehouse "$IMG" add "Garage" --locked
docker run --rm -v wh-smoke:/data --entrypoint /usr/bin/wherehouse "$IMG" list
docker run --rm -v wh-smoke:/data --entrypoint /usr/bin/wherehouse "$IMG" history "Garage" --json
```
Expected: `list` shows `Garage`; history JSON shows the actor as `unknown`.

If `ls` shows `/data` or the DB owned by `0 0`, or the container logs a permission error, the `COPY --chown` trick did not chown the destination directory. Replace the first `COPY` in `goreleaser.Dockerfile` with a prep stage and rebuild:
```dockerfile
FROM busybox:1.37 AS prep
RUN mkdir /data && chown 65532:65532 /data

FROM gcr.io/distroless/static-debian13:nonroot
ARG TARGETPLATFORM
COPY --from=prep --chown=65532:65532 /data /data
```
(keep the rest of the file unchanged; also update the comment above the `COPY`).

- [ ] **Step 6: Smoke-test argument appending (Review Focus 3)**

```bash
docker run -d --name wh-port -p 19090:9090 -v wh-smoke-port:/data "$IMG" --port 9090
curl -fsS http://127.0.0.1:19090/healthz
```
Expected: `curl` exits 0 (proves `--bind 0.0.0.0` was kept and `--port` appended).

- [ ] **Step 7: Verify per-platform architectures (Review Focus 5)**

```bash
ARM_TAG=${TAG%-amd64}-arm64
docker image inspect --format '{{.Os}}/{{.Architecture}}' "ghcr.io/asphaltbuffet/wherehouse:$TAG"
docker image inspect --format '{{.Os}}/{{.Architecture}}' "ghcr.io/asphaltbuffet/wherehouse:$ARM_TAG"
```
Expected: `linux/amd64` then `linux/arm64`.

- [ ] **Step 8: Verify host bind-mount behaviour (Review Focus 2)**

```bash
BIND=/tmp/claude-2001/-home-grue-dev-wherehouse/fa2f132e-2aac-43f1-8888-b91df4e62447/scratchpad/wh-bind
mkdir -p "$BIND"
docker run --rm -v "$BIND":/data --entrypoint /usr/bin/wherehouse "$IMG" list
docker run --rm --user "$(id -u):$(id -g)" -v "$BIND":/data --entrypoint /usr/bin/wherehouse "$IMG" list
ls -ln "$BIND"
```
Expected: first run fails with a permission/`unable to open database` error; second succeeds; `wherehouse.db` exists owned by your uid. Record the exact first-run error text for the README in Task 3.

- [ ] **Step 9: Verify prerelease tagging (Review Focus 4)**

```bash
GORELEASER_CURRENT_TAG=v9.9.9-rc.1 goreleaser release --snapshot --clean
docker images ghcr.io/asphaltbuffet/wherehouse --format '{{.Tag}}' | rg '9\.9'
```
Expected: only `9.9.9-rc.1-snapshot-amd64` and `-arm64`. No `9.9-*` tags. Also confirm no newly created `latest-*` tag: `docker images ghcr.io/asphaltbuffet/wherehouse --format '{{.Tag}} {{.CreatedSince}}' | rg latest` should show only the older image from Step 4.

If goreleaser does not derive `.Prerelease` from `GORELEASER_CURRENT_TAG` in snapshot mode (all three tag kinds appear), say so in the task report rather than changing the templates. The template matches goreleaser's documented empty-tag behaviour, and the first real RC release will confirm it.

- [ ] **Step 10: Clean up smoke-test resources**

```bash
docker rm -f wh-smoke wh-port
docker volume rm wh-smoke wh-smoke-port
docker images ghcr.io/asphaltbuffet/wherehouse -q | xargs -r docker rmi -f
rm -rf dist
```

- [ ] **Step 11: Lint, test, commit**

Run: `mise run lint` then `mise run test` — expected: both pass (no Go changes).
Run `/pre-commit`, then `/commit`. Message:
`feat(release): build multi-arch container image with goreleaser`

---

### Task 2: CI — buildx, QEMU, and GHCR login

**Files:**
- Modify: `.github/workflows/release.yml` (permissions block, steps before "Run GoReleaser")
- Modify: `.github/workflows/snapshot.yml` (`paths` triggers, steps before "Release Test")

**Interfaces:**
- Consumes: the `dockers_v2` block from Task 1 (built during `goreleaser release`, pushed only when not `--snapshot`).
- Produces: nothing consumed by later tasks.

- [ ] **Step 1: Start a fresh change**

Run: `jj new`

- [ ] **Step 2: Grant package write in `release.yml`**

Replace:
```yaml
permissions:
  contents: write
```
with:
```yaml
permissions:
  contents: write
  packages: write
```

- [ ] **Step 3: Add docker setup steps to `release.yml`**

Insert immediately before `- name: Run GoReleaser`:
```yaml
      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to GHCR
        uses: docker/login-action@v3
        with:
          registry: ghcr.io
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

```

- [ ] **Step 4: Add docker setup and trigger path to `snapshot.yml`**

In both the `push.paths` and `pull_request.paths` lists, add after `- '.goreleaser.yml'`:
```yaml
      - 'goreleaser.Dockerfile'
```

Insert immediately before `- name: Release Test`:
```yaml
      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

```
No login and no `packages` permission: snapshot builds never push.

- [ ] **Step 5: Check the latest major versions of the docker actions**

Run:
```bash
gh release view --repo docker/setup-qemu-action --json tagName -q .tagName
gh release view --repo docker/setup-buildx-action --json tagName -q .tagName
gh release view --repo docker/login-action --json tagName -q .tagName
```
If any latest major is newer than `v3`, update that `uses:` line in both workflows to the newer major (e.g. `@v4`). `.github/dependabot.yml` will keep them current afterwards.

- [ ] **Step 6: Lint workflows**

Run: `actionlint .github/workflows/release.yml .github/workflows/snapshot.yml`
Expected: no output, exit 0. If `actionlint` is not installed, run `nix run nixpkgs#actionlint -- .github/workflows/release.yml .github/workflows/snapshot.yml`.

- [ ] **Step 7: Commit**

Run `/pre-commit`, then `/commit`. Message:
`ci: set up buildx and GHCR login for container image release`

---

### Task 3: README and changelog

**Files:**
- Modify: `README.md` (new `### Container` subsection after `### Nix`, before the `---` that precedes `## Quick Start`)
- Create: `.changes/unreleased/Added-<timestamp>.yaml`

**Interfaces:**
- Consumes: image name, tags, env vars, and the bind-mount error text from Task 1 Step 8.
- Produces: nothing.

- [ ] **Step 1: Start a fresh change**

Run: `jj new`

- [ ] **Step 2: Add the Container section to `README.md`**

Insert after the Nix Home Manager code block (after the closing fence of the `flake.nix` example), before the `---`:

````markdown

### Container

A multi-arch (`linux/amd64`, `linux/arm64`) image runs the web UI:

```bash
docker run -d --name wherehouse \
  -p 8080:8080 \
  -v wherehouse:/data \
  ghcr.io/asphaltbuffet/wherehouse:latest
```

- **Tags**: `X.Y.Z` (exact), `X.Y` (latest patch), `latest` (latest stable). Prereleases get only `X.Y.Z-rc.N`.
- **Data**: the database is `/data/wherehouse.db`. Keep `/data` on a volume, or you lose the event log when the container is removed.
- **Bind mounts**: the image runs as uid `65532`. To mount a host directory, either `chown 65532:65532` it or run with `--user "$(id -u):$(id -g)"`.
- **Arguments**: anything after the image name is passed to `wherehouse serve`, e.g. `... wherehouse:latest --port 9090`.
- **Configuration**: set `WHEREHOUSE_*` environment variables (see [Environment Variables](#environment-variables)), or put a `wherehouse.toml` in `/data`.
- **Identity**: web edits are recorded as user `unknown` unless you set `-e WHEREHOUSE_USER_DEFAULT_IDENTITY=<name>`.
- **Logs**: written to stderr; view with `docker logs wherehouse`. Leave `logging.max_size_mb` unset.
- **Health**: `GET /healthz` returns 200 when the server is up. Use it for orchestrator probes; the image has no `HEALTHCHECK`.
- **CLI commands**: run them against the same volume with `docker run --rm -v wherehouse:/data --entrypoint /usr/bin/wherehouse ghcr.io/asphaltbuffet/wherehouse:latest doctor`.
````

If Task 1 Step 8 showed that the default (no `--user`) bind mount works, delete the **Bind mounts** bullet instead.

- [ ] **Step 3: Verify the anchor exists**

Run: `rg -n '^### Environment Variables' README.md`
Expected: one match (the `#environment-variables` link target).

- [ ] **Step 4: Add a changie entry**

Create `.changes/unreleased/Added-20261024-120000.yaml` (use the actual current local time for the filename and `time`):
```yaml
kind: Added
body: multi-arch container image published to ghcr.io/asphaltbuffet/wherehouse
time: 2026-10-24T12:00:00.000000000-04:00
custom:
    Issue: ""
```

- [ ] **Step 5: Commit**

Run `/pre-commit`, then `/commit`. Message:
`docs: document container image usage`

Then run `/audit-docs`.
