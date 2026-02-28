# Tags Flag Implementation Guide

**Quick reference** for implementing `--tags` flag in wherehouse CLI commands.

---

## TL;DR: Copy-Paste Pattern

### 1. Register Flag

```go
var moveCmd = &cobra.Command{
    Use:   "move ITEM LOCATION [flags]",
    Short: "Move an item to a new location",
    Long: `Move an item to a different location.

Examples:
  wherehouse move socket Garage --tags urgent,tool
  wherehouse move key Safe --tags "tag,with,comma",regular`,
    Args: cobra.ExactArgs(2),
    RunE: runMove,
}

func init() {
    rootCmd.AddCommand(moveCmd)
    moveCmd.Flags().String(
        "tags",
        "",
        "Comma-separated tags (quote values with commas)",
    )
}
```

### 2. Parse in Command Handler

```go
func runMove(cmd *cobra.Command, args []string) error {
    tagsFlag, _ := cmd.Flags().GetString("tags")

    // Parse and validate
    parser := &cli.TagsParser{Raw: tagsFlag}
    tags, err := parser.ParseAndValidate()
    if err != nil {
        return fmt.Errorf("invalid tags: %w", err)
    }

    // Canonicalize
    canonicalTags := make([]string, len(tags))
    for i, tag := range tags {
        canonicalTags[i] = cli.CanonicalizeTag(tag)
    }

    // Call domain logic
    result, err := domain.MoveItem(itemSelector, locationSelector, domain.MoveOptions{
        Tags: canonicalTags,
    })
    if err != nil {
        return err
    }

    // Format output
    return formatResult(result)
}
```

---

## API Reference

### TagsParser Type

Location: `internal/cli/tags.go`

```go
type TagsParser struct {
    Raw string // Raw --tags flag value
}

// Parse returns []string of parsed tags (not yet validated)
// Respects quotes: "tag,with,comma" is one tag
// Unquoted commas are delimiters
func (tp *TagsParser) Parse() ([]string, error)

// Validate checks each tag meets wherehouse constraints
// - Not empty
// - Max 100 chars
// - No colons (reserved)
// - No duplicates
// - Valid UTF-8
// - Trimmed
func (tp *TagsParser) Validate(tags []string) error

// ParseAndValidate is the main entry point
// Returns validated, but not canonicalized, tags
func (tp *TagsParser) ParseAndValidate() ([]string, error)
```

### CanonicalizeTag Function

```go
// Canonicalize applies wherehouse naming rules
// - Lowercase
// - Spaces → underscores
// - Dashes → underscores
// - Collapse runs of underscores
func CanonicalizeTag(tag string) string
```

---

## Input Examples

| Input Flag | Parsed Result | Canonical Form |
|-----------|---------------|-----------------|
| `--tags urgent` | `["urgent"]` | `["urgent"]` |
| `--tags urgent,tool,backup` | `["urgent", "tool", "backup"]` | `["urgent", "tool", "backup"]` |
| `--tags "High Priority",tool` | `["High Priority", "tool"]` | `["high_priority", "tool"]` |
| `--tags "tag,with,comma",other` | `["tag,with,comma", "other"]` | `["tag_with_comma", "other"]` |
| `--tags ""` | `[]` | `[]` (empty list) |

---

## Error Handling

User-facing errors (auto-generated):

```
error: invalid tags: tag 0: colons not allowed (reserved for selector syntax): "invalid:tag"

error: invalid tags: duplicate tag: "urgent"

error: invalid tags: tag 1: too long (max 100 chars, got 101): "..."
```

---

## Output Formatting

### Human-Readable

```
Moved "10mm socket" to Garage
Tags: urgent, tool, backup
```

### JSON

```json
{
  "item": { ... },
  "location": { ... },
  "tags": [
    { "display": "High Priority", "canonical": "high_priority" },
    { "display": "tool", "canonical": "tool" }
  ]
}
```

---

## Validation Constraints

| Constraint | Details |
|-----------|---------|
| **Required** | At least one character |
| **Max length** | 100 characters |
| **Forbidden** | Colon (`:`) |
| **Character set** | UTF-8, printable |
| **Duplicates** | Not allowed (same canonical form) |
| **Whitespace** | Trimmed before storage |

---

## Command Integration Checklist

When adding `--tags` to a command:

- [ ] Add flag definition in `init()` function
- [ ] Add examples to command's `Long` description
- [ ] Parse with `TagsParser{Raw: tagsFlag}.ParseAndValidate()`
- [ ] Canonicalize tags before passing to domain layer
- [ ] Format tags in human output
- [ ] Format tags in JSON output (if applicable)
- [ ] Test with unit tests (see `tags_test.go`)
- [ ] Test with integration tests
- [ ] Run `go vet` and `golangci-lint`

---

## Testing Template

```go
func TestMoveCmdWithTags(t *testing.T) {
    tests := []struct {
        name      string
        args      []string
        wantTags  []string
        wantErr   bool
        errMatch  string
    }{
        {
            name:     "valid single tag",
            args:     []string{"move", "item", "loc", "--tags", "urgent"},
            wantTags: []string{"urgent"},
        },
        {
            name:     "valid multiple tags",
            args:     []string{"move", "item", "loc", "--tags", "urgent,tool"},
            wantTags: []string{"urgent", "tool"},
        },
        {
            name:    "invalid: colon in tag",
            args:    []string{"move", "item", "loc", "--tags", "bad:tag"},
            wantErr: true,
            errMatch: "colons not allowed",
        },
        {
            name:    "invalid: duplicate tag",
            args:    []string{"move", "item", "loc", "--tags", "urgent,tool,urgent"},
            wantErr: true,
            errMatch: "duplicate",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Implementation
        })
    }
}
```

---

## Common Mistakes (Don't Do This)

```go
// WRONG: Splitting on comma manually (breaks quoted values)
tags := strings.Split(tagsFlag, ",")

// WRONG: Not canonicalizing before storage
domain.MoveItem(..., MoveOptions{Tags: tags})

// WRONG: Not validating before calling domain
// (always validate first, then canonicalize, then call domain)

// WRONG: Storing both display and canonical forms separately
// (just store canonical; display_name is for items, not tags)

// WRONG: Allowing colons in tags
// (colons are reserved for LOCATION:ITEM selector syntax)
```

---

## Integration Points

### Where TagsParser is Used

1. **Command handlers** (e.g., `internal/cli/move.go`)
   - Parse and validate `--tags` flag
   - Canonicalize tags
   - Pass to domain layer

2. **Unit tests** (e.g., `internal/cli/tags_test.go`)
   - Test parsing with various inputs
   - Test validation rules
   - Test canonicalization

3. **Integration tests** (e.g., `internal/cli/move_test.go`)
   - Test full command flow with tags
   - Test error output
   - Test JSON output

### Domain Layer Integration

Tags are passed to domain logic as canonicalized strings:

```go
// From CLI
canonicalTags := []string{"urgent", "tool_collection"}

// To domain
result, err := domain.MoveItem(itemID, locationID, MoveOptions{
    Tags: canonicalTags,  // Pass as []string
})

// Domain stores in events/projections
// CLI is responsible for parsing/validating only
```

---

## File Locations

```
internal/cli/
├── tags.go              # TagsParser implementation
└── tags_test.go         # Unit tests

cmd/
└── move.go             # Example: move command using tags

// Output formatting in:
internal/cli/
├── output.go           # formatTags, formatTagsJSON, etc.
```

---

## Help Text Template

For commands supporting tags:

```
--tags value    Comma-separated tags to apply (quote values with commas)

Examples:
  wherehouse move socket Garage --tags urgent,tool
  wherehouse move key Safe --tags "House A","Spare Keys"
```

---

## Version History

| Version | Date | Change |
|---------|------|--------|
| 1.0 | 2026-02-20 | Initial design |

---

**See also**:
- Full design: `ai-docs/research/cli/tags-flag-design.md`
- Business rules: `.claude/knowledge/business-rules.md`
- CLI contract: `.claude/knowledge/cli-contract.md`
