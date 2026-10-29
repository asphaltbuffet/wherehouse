# Browser tests use playwright-go behind a build tag

The web UI's viewport-dependent behaviour (the `48rem` breakpoint, `pointer: coarse` sizing, the `tree.js` drawer, htmx swaps) is invisible to handler tests, so it is exercised by browser tests driven by `playwright-go` inside `go test`. They live in `internal/web/browser_*_test.go` under `//go:build browser` and run via `mise run test-browser` and a dedicated CI job; `mise run test` and the Nix build never compile them. Each device subtest gets its own `BrowserContext`, `httptest` server, and `apptesting.OpenApp(t)` database seeded through the `App` API, so tests cannot reach a running `wherehouse serve` or the user's real database. The device matrix is Desktop Chrome, Desktop Firefox, iPad (gen 7), iPad Mini, and Pixel 7; tests branch on a `Layout` (Narrow/Wide) derived from viewport width, never on device name.

## Considered Options

- **Node `@playwright/test`** — rejected: best-in-class runner (device projects, trace viewer, HTML report, `toHaveScreenshot`), but adds Node, `package.json`, and a lockfile to a pure-Go, Nix-built repo, and seeding would go through CLI shell-outs against a temp DB instead of typed `App` calls.
- **chromedp / go-rod** — rejected: no device presets and Chromium-only, so emulation and cross-engine coverage would be bespoke.
- **Always compiled, runtime skip when no browser** — rejected: the Nix sandbox has no browser, so tests would skip silently; a build tag makes "did browser tests run?" an explicit choice.

## Consequences

- No visual regression baselines: `playwright-go` has no screenshot comparison. Layout is asserted behaviourally (no horizontal overflow, touch-target size, drawer visibility, computed dark-mode colours); traces and screenshots are saved on failure only.
- Browser builds are pinned by the `playwright-go` version in `go.mod`; bumping the module upgrades the browsers.
- On NixOS the downloaded browsers rely on nix-ld; WebKit needs its shared libraries listed in `programs.nix-ld.libraries`. CI installs with `--with-deps`.
- Changes to `internal/web/assets` or templates should be checked with `mise run test-browser`, which `mise run test` does not cover.
