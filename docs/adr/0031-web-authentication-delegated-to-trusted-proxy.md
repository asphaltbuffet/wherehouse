# Web UI authentication is delegated to a trusted reverse proxy

`wherehouse serve` has no authentication. Every endpoint is reachable by anyone who can connect to the listener, and web mutations are attributed to the literal actor `webui`, which makes the audit trail meaningless for browser activity (#145).

The motivating deployment is the container image behind Tailscale Serve, where the proxy already authenticates tailnet users and injects their identity as a request header (`Tailscale-User-Login`).

## Decision

Wherehouse does not authenticate web requests itself. Access control is the responsibility of a reverse proxy that is the sole network path to the listener. Wherehouse's only identity concern is **attribution**: an opt-in setting names a request header whose value, when present and non-empty, becomes the `Actor` on events produced by that request (#267).

## Consequences

- The free-text "User" field on the web add-item form is removed. Every web mutation derives its actor the same way (identity header, else `webui`); the web UI offers no equivalent of the CLI's `--as`.
- The page header always displays the effective actor as "acting as …" (including `webui`), so a header that fails to arrive is visible. The wording deliberately avoids "signed in", since no login exists.
- No credential storage, password hashing, session handling, or 401 middleware in `internal/web`.
- Running `serve` on a reachable interface without a proxy exposes the inventory to anyone on that network; this is the operator's responsibility and is documented in `serve --help`.
- When an actor header is configured and the bind address is not loopback, `serve` warns at startup but still runs. Proxies in a separate container legitimately need a non-loopback bind, so refusing would break valid deployments; the shipped container image binds `0.0.0.0`, which in a Tailscale-sidecar namespace exposes the listener on the tailnet IP and lets peers bypass Serve and forge the header.
- The identity header is only trustworthy when the proxy overwrites or strips any client-supplied copy and nothing can reach the listener directly (e.g. `--bind 127.0.0.1` inside the proxy's network namespace).

## Considered Options

**HTTP Basic Auth with a hashed credential in `config.toml`** (the original #145 proposal). Rejected for now: it duplicates authentication that the target deployment already performs, yields a single shared identity (no better attribution than `webui` for multi-user households), and puts credential handling into a small inventory tool. Per-user accounts or OIDC remain possible later if a no-proxy deployment becomes a real need; doing so would supersede this ADR.
