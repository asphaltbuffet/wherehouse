# `import --replace` requires an explicit `--yes` flag, not an interactive prompt

The `--replace` flag truncates all event and projection data before replaying the import — a destructive, irreversible operation. Confirmation is required to prevent accidental data loss.

We chose a `--yes` flag over an interactive TTY prompt for two reasons: (1) interactive prompts require TTY detection, which adds complexity and behaves inconsistently when import is used in scripts or pipelines; (2) `--yes` is an established convention in CLI tooling (`kubectl delete`, `gh repo delete`) that is universally understood as "I know what I'm doing."

`--yes` without `--replace` is silently accepted and has no effect — rejecting it would be unnecessarily strict for users who habitually pass it.
