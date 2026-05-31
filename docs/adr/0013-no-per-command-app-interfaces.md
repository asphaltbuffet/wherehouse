# Command packages take *app.App directly; no per-command interfaces

Each `cmd/xxx/` package accepts `*app.App` directly in its injectable constructor (`NewXxxCmd(a *app.App)`). There are no per-command `xxxApp` interfaces and no `cmd/*/db.go` files.

We previously had a one-method interface per command (e.g. `addApp`, `scryApp`) backed by a hand-rolled record-fake in tests. These failed the two-adapter test: the fake only captured the request and returned a preconfigured response — it never exercised behaviour divergent from the real `App`. With only one real adapter, the seam was hypothetical, and the fake created a drift risk where tests could pass while the real wiring was broken.

Command tests now use `apptesting.OpenApp(t)`, which wires a real `*app.App` over an in-memory SQLite store. Assertions target observable DB state rather than captured arguments. This trades slightly heavier test setup for elimination of fake/prod drift.

The `internal/web` package retains its `App` interface (`internal/web/app.go`). That interface has eight methods and separates HTTP handler concerns from business logic — a handler test can verify error rendering and HTTP response codes by injecting specific return values without spinning up SQLite. That is a real architectural seam and is not covered by this decision.
