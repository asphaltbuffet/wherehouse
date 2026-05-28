# Export format is NDJSON (newline-delimited JSON), not a JSON array

The `export` command writes one JSON object per line to stdout, with no wrapping array.

We chose this over a single JSON array because NDJSON streams naturally — consumers can process events one at a time without buffering the entire output. It composes directly with standard Unix tools: `wc -l` gives the event count, `jq` processes records individually, `head`/`tail`/`grep` work line-by-line. A JSON array requires reading the entire output before parsing begins, and is marginally larger (outer brackets, inter-element commas). The export use case (backup, migration, piping) is inherently streaming-friendly.
