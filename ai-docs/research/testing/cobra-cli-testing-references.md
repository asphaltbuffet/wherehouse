# Cobra CLI Testing — References

Companion to `cobra-cli-testing.md`. Maps document sections to source URLs. For human readers only — agents should use the primary document.

| Section | Sources |
|---|---|
| §2 Thin entrypoint / core principle | [Cobra Enterprise Guide](https://cobra.dev/docs/explanations/enterprise-guide/) · [gianarb.it](https://gianarb.it/blog/golang-mockmania-cli-command-with-cobra) · [bradcypert.com](https://www.bradcypert.com/testing-a-cobra-cli-in-go/) · [qua.name](https://qua.name/antolius/making-a-testable-cobra-cli-app) |
| §3 What Cobra guarantees | [cobra/args.go](https://github.com/spf13/cobra/blob/main/args.go) · [cobra/args_test.go](https://github.com/spf13/cobra/blob/main/args_test.go) · [Working with Flags](https://cobra.dev/docs/how-to-guides/working-with-flags/) · [issue #1413](https://github.com/spf13/cobra/issues/1413) · [issue #745](https://github.com/spf13/cobra/issues/745) |
| §4 What must be tested | [Gopher Advent 2022](https://gopheradvent.com/calendar/2022/taming-cobras-making-most-of-cobra-clis/) · [eli.thegreenplace.net](https://eli.thegreenplace.net/2020/testing-flag-parsing-in-go-programs/) · [copyprogramming.com subcommands](https://copyprogramming.com/howto/cobra-viper-golang-how-to-test-subcommands) |
| §5.3 Context / PreRunE testing | [cobra/command_test.go](https://github.com/spf13/cobra/blob/main/command_test.go) · [PR #893](https://github.com/spf13/cobra/pull/893) · [PR #1551](https://github.com/spf13/cobra/pull/1551) · [issue #1109](https://github.com/spf13/cobra/issues/1109) · [blog.ksub.org](https://blog.ksub.org/bytes/2019/10/07/using-context.context-with-cobra/) |
| §5.4 SetOut/SetErr output testing | [issue #1708](https://github.com/spf13/cobra/issues/1708) · [issue #1100](https://github.com/spf13/cobra/issues/1100) · [Gopher Advent 2022](https://gopheradvent.com/calendar/2022/taming-cobras-making-most-of-cobra-clis/) |
| §5.6 Singleton / global state | [issue #770](https://github.com/spf13/cobra/issues/770) · [issue #1180](https://github.com/spf13/cobra/issues/1180) · [issue #1488](https://github.com/spf13/cobra/issues/1488) · [issue #1599](https://github.com/spf13/cobra/issues/1599) · [gh CLI issue #759](https://github.com/cli/cli/issues/759) |
| §6 Coverage thresholds | [go-test-coverage](https://github.com/vladopajic/go-test-coverage) · [DEV community coverage](https://dev.to/gkampitakis/golang-coverage-29cb) |
| §7 testscript integration tests | [rogpeppe/go-internal](https://pkg.go.dev/github.com/rogpeppe/go-internal/testscript) · [Encore blog](https://encore.dev/blog/testscript-hidden-testing-gem) · [Bitfield Consulting](https://bitfieldconsulting.com/posts/cli-testing) · [go.dev build-cover](https://go.dev/doc/build-cover) · [FOSDEM 2024](https://archive.fosdem.org/2024/events/attachments/fosdem-2024-1802-testing-go-command-line-programs-with-go-internal-testscript-/slides/22802/Testing_Go_programs_with_go-internal-testscript_wT7GqqA.pdf) |

**Research method:** Web search against Cobra official docs, GitHub issues/PRs, and practitioner blogs. Compiled 2026-02-27.
