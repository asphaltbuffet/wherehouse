# add command batch creates all entities in one transaction; --json always returns an array

The `add` command accepts one or more path arguments and creates all entities in a single SQLite transaction (all succeed or all fail). `--json` always returns a JSON array even for a single argument — a shape that varies with argument count would make consumers fragile.
