# Container Image — Design

Agreed in a grilling session on 2026-10-24. Packaging-only change; no Go code changes.

## Decisions

1. **Purpose — server only.** `ENTRYPOINT ["/usr/bin/wherehouse", "serve", "--bind", "0.0.0.0"]`, `EXPOSE 8080`. Extra `docker run` args append to `serve` (e.g. `--port 9090`).
2. **Actor identity — not set in the image.** Web-originated events record actor `"unknown"` unless the operator sets `WHEREHOUSE_USER_DEFAULT_IDENTITY`. Documented as a caveat.
3. **Base — `gcr.io/distroless/static-debian13:nonroot`** (debian12 is no longer published). Runs as uid/gid `65532`. `/data` exists in the image, owned by `65532:65532`. The UPX-compressed release binary is used as-is (no separate container build).
4. **Registry — `ghcr.io/asphaltbuffet/wherehouse` only**, platforms `linux/amd64` + `linux/arm64`, one multi-arch manifest via goreleaser `dockers_v2`. Auth via `GITHUB_TOKEN`.
5. **Tags — `{{ .Version }}`, `{{ .Major }}.{{ .Minor }}`, `latest`;** the last two omitted on prereleases. No `v`-prefixed tags.
6. **Logs — `ENV WHEREHOUSE_LOGGING_FILE_PATH=/dev/stderr`.** Requires `logging.max_size_mb` to stay `0`. Proper stderr sink tracked in #257.
7. **Config — `WORKDIR /data`.** Env vars are primary; an optional `/data/wherehouse.toml` is picked up as the local config. `ENV WHEREHOUSE_DB_PATH=/data/wherehouse.db`, `VOLUME /data`.
8. **Health — no `HEALTHCHECK`.** Document `GET /healthz` for orchestrator probes. No healthcheck subcommand.
9. **Files — `goreleaser.Dockerfile` at repo root;** "Container" subsection under README "Installation".

## Out of scope

- Docker Hub, other registries, other architectures
- Stderr log sink (#257)
- Per-request web identity
- Non-UPX container binary
