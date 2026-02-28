# Tags Flag CLI Design Pattern

**Status**: Design proposal for golang-ui-developer
**Date**: 2026-02-20
**Scope**: `--tags` flag implementation for wherehouse CLI commands

---

## Overview

This document specifies the CLI flag pattern for accepting and processing comma-separated tags in wherehouse commands. The design follows the wherehouse CLI contract conventions and cobra framework best practices.

### Requirements

- Accept comma-separated tag values: `--tags tag1,tag2,tag3`
- Support quoted values with commas: `--tags "tag with,comma",regular_tag`
- Validate tag format (characters allowed, length constraints)
- Integrate seamlessly with cobra commands
- Follow wherehouse name canonicalization rules
- Support JSON output mode
- Clear error messages for invalid tags
- Consistent with existing flag patterns

---

## Design Decisions

### 1. Flag Definition

**Pattern**: String flag with custom parser

```go
// Register flag on command
moveCmd.Flags().String(
    "tags",
    "",
    "Comma-separated tags to apply (can quote values with commas: \"tag,with,comma\",regular)",
)
```

**Rationale**:
- Single string flag is simpler than StringSlice (which doesn't preserve order in help text)
- Allows users to pass tags naturally: `--tags tag1,tag2,tag3`
- Quoted values support complex tags: `--tags "Project A","Budget 2026",urgent`
- Follows shell conventions (similar to `--include` in many Unix tools)

### 2. Parsing Strategy

**Two-phase parsing**:
1. **Shell parsing**: Let the shell handle quoting (user provides `--tags "tag,with,comma",regular`)
2. **Application parsing**: Custom CSV-aware parser respects quoted values

**Parser implementation**:

```go
// TagsParser handles comma-separated tags with quote awareness
type TagsParser struct {
    Raw string // Raw --tags flag value
}

// Parse returns []string of validated tags
// Respects quoted values: "tag,with,comma" is a single tag
// Unquoted commas are delimiters
func (tp *TagsParser) Parse() ([]string, error) {
    if tp.Raw == "" {
        return []string{}, nil
    }

    reader := csv.NewReader(strings.NewReader(tp.Raw))
    reader.LazyQuotes = false  // Strict quote validation
    reader.TrimLeadingSpace = true

    records, err := reader.ReadAll()
    if err != nil {
        return nil, fmt.Errorf(
            "invalid tags format: %w (use comma delimiter, quote values with commas: \"tag,with,comma\")",
            err,
        )
    }

    // Flatten records (csv.Reader returns [][]string, one record per row)
    if len(records) != 1 {
        return nil, fmt.Errorf(
            "invalid tags format: expected single line (got %d lines)",
            len(records),
        )
    }

    return records[0], nil
}

// Validate checks each tag meets wherehouse constraints
func (tp *TagsParser) Validate(tags []string) error {
    for i, tag := range tags {
        if tag == "" {
            return fmt.Errorf("tag %d: empty tag not allowed", i)
        }

        if len(tag) > 100 {
            return fmt.Errorf(
                "tag %d: too long (max 100 chars, got %d): %q",
                i, len(tag), tag,
            )
        }

        // Tags must not contain colons (reserved for selectors)
        if strings.Contains(tag, ":") {
            return fmt.Errorf(
                "tag %d: colons not allowed (reserved for item selector syntax): %q",
                i, tag,
            )
        }

        // Tags must be valid UTF-8 and contain printable characters
        if !utf8.ValidString(tag) {
            return fmt.Errorf("tag %d: invalid UTF-8: %q", i, tag)
        }

        // Warn about leading/trailing whitespace
        if tag != strings.TrimSpace(tag) {
            return fmt.Errorf(
                "tag %d: remove leading/trailing whitespace: %q",
                i, tag,
            )
        }
    }

    // Check for duplicates
    seen := make(map[string]bool)
    for _, tag := range tags {
        if seen[tag] {
            return fmt.Errorf("duplicate tag: %q", tag)
        }
        seen[tag] = true
    }

    return nil
}

// ParseAndValidate is the main entry point
func (tp *TagsParser) ParseAndValidate() ([]string, error) {
    tags, err := tp.Parse()
    if err != nil {
        return nil, err
    }

    if err := tp.Validate(tags); err != nil {
        return nil, err
    }

    return tags, nil
}

// Canonicalize applies wherehouse canonicalization rules to tags
// Tags follow item/location naming: lowercase, spaces → underscores, etc.
func Canonicalize(tag string) string {
    // Trim whitespace
    tag = strings.TrimSpace(tag)

    // Convert to lowercase
    tag = strings.ToLower(tag)

    // Replace spaces and dashes with underscores
    tag = strings.NewReplacer(
        " ", "_",
        "-", "_",
    ).Replace(tag)

    // Collapse runs of underscores
    for strings.Contains(tag, "__") {
        tag = strings.ReplaceAll(tag, "__", "_")
    }

    return tag
}
```

### 3. Integration with Cobra Commands

**Pattern for commands accepting tags**:

```go
var moveCmd = &cobra.Command{
    Use:   "move ITEM LOCATION [flags]",
    Short: "Move an item to a new location",
    Long: `Move an item to a different location.

The --tags flag accepts comma-separated values. Use quotes for tags containing commas:

  wherehouse move "10mm socket" Garage --tags urgent,tool-collection
  wherehouse move "spare key" Safe --tags "House A","Spare Keys",backup
  wherehouse move item location --tags "tag,with,comma",regular,"another,one"`,
    Args: cobra.ExactArgs(2),
    RunE: runMove,
}

func init() {
    rootCmd.AddCommand(moveCmd)

    // Add tags flag
    moveCmd.Flags().String(
        "tags",
        "",
        "Comma-separated tags (quote values with commas: \"tag,with,comma\",regular)",
    )

    // Add other standard flags
    moveCmd.Flags().String("project", "", "Associate with project")
    moveCmd.Flags().BoolP("quiet", "q", false, "Quiet output")
    moveCmd.Flags().BoolP("json", "j", false, "JSON output")
}

func runMove(cmd *cobra.Command, args []string) error {
    tagsFlag, _ := cmd.Flags().GetString("tags")
    projectID, _ := cmd.Flags().GetString("project")
    jsonOutput, _ := cmd.Flags().GetBool("json")
    quiet, _ := cmd.Flags().GetBool("quiet")

    // Parse and validate tags
    parser := &TagsParser{Raw: tagsFlag}
    tags, err := parser.ParseAndValidate()
    if err != nil {
        return fmt.Errorf("invalid tags: %w", err)
    }

    // Canonicalize tags for storage
    canonicalTags := make([]string, len(tags))
    for i, tag := range tags {
        canonicalTags[i] = Canonicalize(tag)
    }

    // Call domain logic (golang-developer's implementation)
    itemSelector := args[0]
    locationSelector := args[1]

    result, err := domain.MoveItem(itemSelector, locationSelector, domain.MoveOptions{
        ProjectID:      projectID,
        Tags:           canonicalTags,  // Pass canonicalized tags
        DisplayTags:    tags,            // Also store display form if needed
    })
    if err != nil {
        return formatError(err)
    }

    // Format output based on flags
    if jsonOutput {
        return outputJSON(result)
    } else if !quiet {
        return outputHuman(result)
    }
    return nil
}
```

### 4. Output Formatting

**Human-readable format**:

```
$ wherehouse move socket Garage --tags urgent,toolbox
Moved "10mm socket" to Garage
Tags: urgent, toolbox
```

**JSON output format**:

```json
{
  "item": {
    "id": "uuid-123",
    "display_name": "10mm socket",
    "canonical_name": "10mm_socket"
  },
  "location": {
    "id": "loc-456",
    "display_name": "Garage",
    "canonical_name": "garage"
  },
  "tags": [
    {
      "display": "urgent",
      "canonical": "urgent"
    },
    {
      "display": "toolbox",
      "canonical": "toolbox"
    }
  ],
  "event_id": 42
}
```

### 5. Error Handling

**User-facing error messages**:

```go
func formatTagError(err error) error {
    switch {
    case errors.Is(err, ErrEmptyTag):
        return fmt.Errorf("tags cannot be empty - provide at least one character")

    case errors.Is(err, ErrTagTooLong):
        return fmt.Errorf("tag too long (max 100 characters)")

    case errors.Is(err, ErrColonInTag):
        return fmt.Errorf("colons not allowed in tags (reserved for selector syntax)")

    case errors.Is(err, ErrDuplicateTag):
        return fmt.Errorf("duplicate tags not allowed - check spelling")

    case errors.Is(err, ErrInvalidFormat):
        return fmt.Errorf(
            "invalid tags format. Use comma delimiter and quote complex values:\n" +
            "  --tags tag1,tag2,tag3\n" +
            "  --tags \"tag with,comma\",regular",
        )

    default:
        return err
    }
}
```

### 6. Help Text Examples

**Minimal**: In command's `Short`/`Long` description

```
--tags value        Comma-separated tags (quote values with commas)
```

**Comprehensive**: In command's `Long` description

```
Tags:

  Tags allow organizing and categorizing items. Use comma-separated values:

    wherehouse move socket Garage --tags urgent,tool
    wherehouse move key Safe --tags "House A","Spare Keys"

  For tags containing commas, use quotes:

    --tags "tag,with,comma",regular

  Tags follow naming rules (lowercase, spaces→underscores, max 100 chars).
  Colons are reserved and not allowed in tags.
```

---

## Validation Rules

### Tag Format Constraints

1. **Required**: At least one character
2. **Maximum length**: 100 characters
3. **Character set**: UTF-8, printable characters
4. **Forbidden**: Colon (`:`) - reserved for selectors
5. **No duplicates**: Same tag cannot appear twice
6. **Whitespace**: Trimmed before storage

### Canonicalization Rules (matching wherehouse standards)

- Case-insensitive: `Urgent` → `urgent`
- Spaces → underscores: `high priority` → `high_priority`
- Dashes → underscores: `tool-collection` → `tool_collection`
- Collapse runs: `high__priority` → `high_priority`

### Example transformations

```
Input flag:    --tags "High Priority",tool-collection,urgent
Raw string:    High Priority,tool-collection,urgent
Parsed:        ["High Priority", "tool-collection", "urgent"]
Canonicalized: ["high_priority", "tool_collection", "urgent"]
Stored:        high_priority, tool_collection, urgent
```

---

## Test Coverage

### Unit Tests for TagsParser

```go
func TestTagsParser_Parse(t *testing.T) {
    tests := []struct {
        name    string
        input   string
        want    []string
        wantErr bool
    }{
        {
            name:  "single tag",
            input: "urgent",
            want:  []string{"urgent"},
        },
        {
            name:  "multiple tags",
            input: "urgent,tool,backup",
            want:  []string{"urgent", "tool", "backup"},
        },
        {
            name:  "quoted tag with comma",
            input: `"tag,with,comma",regular`,
            want:  []string{"tag,with,comma", "regular"},
        },
        {
            name:  "multiple quoted tags",
            input: `"House A","Spare Keys",backup`,
            want:  []string{"House A", "Spare Keys", "backup"},
        },
        {
            name:  "whitespace handling",
            input: "tag1 , tag2 , tag3",
            want:  []string{"tag1", "tag2", "tag3"},
        },
        {
            name:    "empty input",
            input:   "",
            want:    []string{},
            wantErr: false,
        },
        {
            name:    "unclosed quote",
            input:   `"unclosed`,
            wantErr: true,
        },
        {
            name:    "mismatched quotes",
            input:   `"tag1",tag2"`,
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            parser := &TagsParser{Raw: tt.input}
            got, err := parser.Parse()
            if tt.wantErr {
                assert.Error(t, err)
                return
            }
            assert.NoError(t, err)
            assert.Equal(t, tt.want, got)
        })
    }
}

func TestTagsParser_Validate(t *testing.T) {
    tests := []struct {
        name    string
        tags    []string
        wantErr bool
        errMsg  string
    }{
        {
            name:    "valid tags",
            tags:    []string{"urgent", "tool", "backup"},
            wantErr: false,
        },
        {
            name:    "empty tag",
            tags:    []string{"urgent", "", "backup"},
            wantErr: true,
            errMsg:  "empty tag",
        },
        {
            name:    "tag too long",
            tags:    []string{strings.Repeat("a", 101)},
            wantErr: true,
            errMsg:  "too long",
        },
        {
            name:    "colon in tag",
            tags:    []string{"invalid:tag"},
            wantErr: true,
            errMsg:  "colons not allowed",
        },
        {
            name:    "duplicate tags",
            tags:    []string{"urgent", "tool", "urgent"},
            wantErr: true,
            errMsg:  "duplicate tag",
        },
        {
            name:    "whitespace only tag",
            tags:    []string{"   "},
            wantErr: true,
            errMsg:  "whitespace",
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            parser := &TagsParser{Raw: ""}
            err := parser.Validate(tt.tags)
            if tt.wantErr {
                assert.Error(t, err)
                if tt.errMsg != "" {
                    assert.Contains(t, err.Error(), tt.errMsg)
                }
                return
            }
            assert.NoError(t, err)
        })
    }
}

func TestCanonicalizeTag(t *testing.T) {
    tests := []struct {
        input string
        want  string
    }{
        {"Urgent", "urgent"},
        {"High Priority", "high_priority"},
        {"tool-collection", "tool_collection"},
        {"HIGH__PRIORITY", "high_priority"},
        {"  spaces  ", "spaces"},
        {"Mix Case-Dash And Space", "mix_case_dash_and_space"},
    }

    for _, tt := range tests {
        t.Run(tt.input, func(t *testing.T) {
            got := Canonicalize(tt.input)
            assert.Equal(t, tt.want, got)
        })
    }
}
```

### Integration Tests

```go
func TestMoveCmdWithTags(t *testing.T) {
    tests := []struct {
        name      string
        args      []string
        wantTags  []string
        wantErr   bool
    }{
        {
            name:     "move with single tag",
            args:     []string{"move", "socket", "garage", "--tags", "urgent"},
            wantTags: []string{"urgent"},
        },
        {
            name:     "move with multiple tags",
            args:     []string{"move", "socket", "garage", "--tags", "urgent,tool,backup"},
            wantTags: []string{"urgent", "tool", "backup"},
        },
        {
            name:     "move with quoted tag containing comma",
            args:     []string{"move", "socket", "garage", "--tags", `"tag,with,comma",regular`},
            wantTags: []string{"tag,with,comma", "regular"},
        },
        {
            name:    "move with invalid tag (colon)",
            args:    []string{"move", "socket", "garage", "--tags", "invalid:tag"},
            wantErr: true,
        },
        {
            name:    "move with duplicate tags",
            args:    []string{"move", "socket", "garage", "--tags", "urgent,tool,urgent"},
            wantErr: true,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            cmd := NewRootCmd()
            cmd.SetArgs(tt.args)
            err := cmd.Execute()

            if tt.wantErr {
                assert.Error(t, err)
                return
            }

            assert.NoError(t, err)
            // Verify tags were passed to domain layer
            // (requires mock or test double)
        })
    }
}
```

---

## Implementation Checklist

- [ ] Create `internal/cli/tags.go` with `TagsParser` type
- [ ] Implement `Parse()` method with CSV-aware parsing
- [ ] Implement `Validate()` method for constraints
- [ ] Implement `ParseAndValidate()` entry point
- [ ] Implement `Canonicalize()` function
- [ ] Add `TagsParser` unit tests
- [ ] Define flag on commands (e.g., `moveCmd.Flags().String("tags", ...)`)
- [ ] Integrate with command handlers (call `ParseAndValidate()`)
- [ ] Pass canonicalized tags to domain layer
- [ ] Format tags in human output
- [ ] Format tags in JSON output
- [ ] Write integration tests for commands with tags
- [ ] Add help text examples to command descriptions
- [ ] Run `go vet ./...`
- [ ] Run `golangci-lint run`
- [ ] Verify tests pass

---

## File Structure

```
internal/cli/
├── flags.go          # Global flag handling
├── tags.go           # TagsParser implementation
├── output.go         # Output formatting
└── ...

internal/cli/test/
├── tags_test.go      # TagsParser unit tests
└── ...

cmd/
├── move.go           # Example: move command with tags flag
└── ...
```

---

## Example Commands (Once Implemented)

```bash
# Single tag
wherehouse move "10mm socket" Garage --tags urgent

# Multiple tags
wherehouse move "10mm socket" Garage --tags urgent,tool,wrench

# Tags with spaces (quoted)
wherehouse move "spare key" Safe --tags "House A",backup

# Tags with commas (quoted)
wherehouse move "spare key" Safe --tags "tag,with,comma",regular

# Tags with JSON output
wherehouse move "spare key" Safe --tags urgent,backup --json

# Tags with quiet output (just exit code)
wherehouse move "spare key" Safe --tags urgent -q
```

---

## Comparison with Alternatives

### Alternative 1: Multiple `--tag` flags

```bash
wherehouse move socket Garage --tag urgent --tag tool --tag backup
```

**Pros**: Simple, clear, cobra native
**Cons**: Verbose, harder to script, less common in Unix CLI

### Alternative 2: Space-separated (unquoted)

```bash
wherehouse move socket Garage --tags urgent tool backup
```

**Pros**: Natural, no escaping
**Cons**: Ambiguous with positional args, harder to parse, breaks if tag contains space

### Chosen: Comma-separated with quote support

```bash
wherehouse move socket Garage --tags urgent,tool,backup
```

**Chosen because**:
- Single flag value (clean)
- Follows Unix conventions (`--include`, `--exclude`, etc.)
- Quoted values for complex strings
- CSV-aware parsing handles edge cases
- Easy to script: `--tags tag1,tag2,tag3`
- Consistent with selector syntax philosophy

---

## Future Enhancements

1. **Tag completion**: Shell completion for known tags
2. **Tag aliases**: Allow `--tag-alias` for frequently used tag sets
3. **Tag filtering**: `wherehouse list --with-tags urgent` to query by tags
4. **Tag history**: Track tag changes in events
5. **Tag suggestions**: Suggest tags based on item context

---

## References

- Wherehouse CLI Contract: `.claude/knowledge/cli-contract.md`
- Domain Model: `.claude/knowledge/domain-model.md`
- Cobra Documentation: https://cobra.dev/
- CSV Parsing: Go `encoding/csv` package

---

**Implementation owner**: golang-ui-developer
**Review owner**: code-reviewer
**Next step**: Implement `internal/cli/tags.go` with unit tests
