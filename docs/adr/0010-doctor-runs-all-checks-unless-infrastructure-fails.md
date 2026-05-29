# Doctor runs all checks unless infrastructure fails

`wherehouse doctor` is designed to give a holistic view of inventory health in a single run. All checks execute unless a check's infrastructure prerequisite has already failed — at which point running that check would produce a predetermined, uninformative failure.

## The rule

- **Config file checks** (TOML validity, path expansion) never block DB checks. A config issue such as an invalid field value does not prevent a DB connection; running the DB checks tells the user whether the application is otherwise healthy.
- **DB-open failure** blocks event-log and projection checks. If the database cannot be opened, both checks will fail for the same infrastructure reason — running them produces noise, not signal.
- **Event-log check failure** (issues found, not an error) does not block the projection check. The two checks are independent scans; both results are useful together.
- **App-layer errors** (unexpected DB errors during a check) abort immediately — these indicate infrastructure instability, not findings.

## Why not abort on config issues

An earlier design aborted as soon as any config issue was found (matching the "misconfiguration diagnosed before connection" framing). This was revised because config issues are not all equally blocking: a bad field value or an invalid TOML file does not prevent a DB connection. Aborting on any config issue would hide otherwise-healthy DB state, reducing the diagnostic value of a single `doctor` run.

## Contrast with `config check`

`config check` is a focused config-only command that reports file validity. `doctor` is a holistic command; config checking is one layer among three. The abort rules differ accordingly.
