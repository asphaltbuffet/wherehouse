# Tags Flag CLI Design - Summary

**Status**: Design completed and approved
**Date**: 2026-02-20
**Author**: golang-ui-developer agent

---

## Design Overview

This project defines a complete CLI flag pattern for `--tags` in the wherehouse inventory tracker. The pattern follows cobra framework conventions and wherehouse CLI contract principles.

### Key Characteristics

- **Flag style**: Comma-separated values
- **Quoting support**: Quoted values can contain commas
- **Validation**: 7 constraints (non-empty, max 100 chars, no colons, etc.)
- **Canonicalization**: Lowercase, spaces→underscores, collapse runs
- **Error messages**: Clear, user-friendly, actionable
- **Output modes**: Human-readable, JSON, quiet
- **Integration**: Thin wrapper over domain layer (golang-developer)

---

## Core Pattern

### User Input

```bash
wherehouse move "10mm socket" Garage --tags urgent,tool,backup
wherehouse move key Safe --tags "tag,with,comma",regular
```

### Parsing Flow

```
Raw input: "urgent,tool,backup"
         ↓
CSV parser (respects quotes)
         ↓
Validate (non-empty, no colons, no duplicates, etc.)
         ↓
Parsed: ["urgent", "tool", "backup"]
         ↓
Canonicalize (lowercase, spaces→underscores)
         ↓
Canonical: ["urgent", "tool", "backup"]
         ↓
Pass to domain logic
```

### Code Pattern

```go
// Register flag
moveCmd.Flags().String("tags", "", "Comma-separated tags")

// In handler
parser := &cli.TagsParser{Raw: tagsFlag}
tags, err := parser.ParseAndValidate()  // Returns parsed form
if err != nil {
    return fmt.Errorf("invalid tags: %w", err)
}

// Canonicalize
canonical := make([]string, len(tags))
for i, tag := range tags {
    canonical[i] = cli.CanonicalizeTag(tag)
}

// Call domain
result, err := domain.MoveItem(item, location, MoveOptions{
    Tags: canonical,
})
```

---

## Detailed Documentation

### Primary Documents

1. **tags-flag-design.md** (comprehensive)
   - Full specification with rationale
   - Validation rules and constraints
   - Error handling patterns
   - Test coverage examples
   - Implementation checklist
   - **Use for**: Understanding design decisions, implementing feature

2. **tags-implementation-guide.md** (quick reference)
   - Copy-paste code patterns
   - API reference (TagsParser, CanonicalizeTag)
   - Input/output examples
   - Common mistakes
   - Integration checklist
   - **Use for**: Quick lookup while coding

3. **tags-example-code.go** (complete working example)
   - Full TagsParser implementation
   - Canonicalization logic
   - Cobra command pattern
   - Unit test examples
   - Output formatting examples
   - **Use for**: Reference implementation, copy patterns

---

## Design Decisions

### Decision 1: Comma-Separated vs. Multiple Flags

**Chosen**: Comma-separated (`--tags tag1,tag2,tag3`)

| Option | Pros | Cons |
|--------|------|------|
| Comma-separated | Single flag, scriptable, Unix convention, clear | Requires careful quoting |
| Multiple flags | Simple, cobra native | Verbose, harder to script |
| Space-separated | Natural | Ambiguous with positional args |

### Decision 2: CSV Parsing with Quote Support

**Chosen**: Use Go's `encoding/csv` for parsing

| Option | Reason |
|--------|--------|
| CSV reader | Handles quotes correctly, standard library |
| Manual split | Breaks on quoted commas, error-prone |
| Regex | Complex, fragile |

### Decision 3: Validation Strategy

**Chosen**: Two-phase (parse then validate)

```go
tags, _ := parser.Parse()           // Syntax check
parser.Validate(tags)               // Semantic checks
```

Allows clear error messages for each validation failure.

### Decision 4: Canonicalization

**Chosen**: Apply wherehouse rules (matching items/locations)

- Lowercase: `Urgent` → `urgent`
- Spaces→underscores: `High Priority` → `high_priority`
- Collapse runs: `high__priority` → `high_priority`

Consistent with existing domain model.

### Decision 5: Constraint: No Colons

**Chosen**: Forbid colons in tags

**Reason**: Colons reserved for `LOCATION:ITEM` selector syntax. Prevents ambiguity.

---

## Validation Constraints (7 Rules)

| # | Rule | Example |
|---|------|---------|
| 1 | Non-empty | `""` → ERROR |
| 2 | Max 100 chars | `"abc...xyz"` (101) → ERROR |
| 3 | No colons | `"bad:tag"` → ERROR |
| 4 | No duplicates | `"urgent,tool,urgent"` → ERROR |
| 5 | Valid UTF-8 | (binary) → ERROR |
| 6 | Trimmed | `"  spaces  "` → ERROR |
| 7 | Printable | (control chars) → ERROR |

**Error messages**: Specific, actionable, cite rule violated

---

## Canonicalization Examples

| Input | Parsed | Canonical |
|-------|--------|-----------|
| `--tags Urgent` | `["Urgent"]` | `["urgent"]` |
| `--tags High Priority,urgent` | `["High Priority", "urgent"]` | `["high_priority", "urgent"]` |
| `--tags "tag,comma",other` | `["tag,comma", "other"]` | `["tag_comma", "other"]` |
| `--tags TOOL-COLLECTION` | `["TOOL-COLLECTION"]` | `["tool_collection"]` |

---

## Integration Points

### With Cobra

```go
// Define flag
cmd.Flags().String("tags", "", "Comma-separated tags")

// Get value
tagsFlag, _ := cmd.Flags().GetString("tags")

// Parse
parser := &cli.TagsParser{Raw: tagsFlag}
tags, err := parser.ParseAndValidate()
```

### With Domain Logic (golang-developer)

```go
// CLI canonicalizes and passes to domain
domain.MoveItem(item, location, MoveOptions{
    Tags: canonicalTags,  // []string of canonical names
})
```

### With Output Formatting

```go
// Human
fmt.Printf("Tags: %s\n", strings.Join(tags, ", "))

// JSON
type Result struct {
    Tags []TagOutput `json:"tags"`
}
type TagOutput struct {
    Display   string `json:"display"`
    Canonical string `json:"canonical"`
}
```

---

## Test Coverage

### Unit Tests (tags_test.go)

```
Parse tests (12 cases)
  - empty, single, multiple, quoted, whitespace, errors

Validate tests (9 cases)
  - valid, empty, too long, colon, duplicate, whitespace, UTF-8

Canonicalize tests (6 cases)
  - lowercase, spaces, dashes, collapse, trim
```

### Integration Tests (move_test.go, etc.)

```
Command tests (8 cases)
  - valid single/multiple, quoted, invalid (colon/duplicate)
  - JSON output, quiet output
  - error message clarity
```

---

## File Structure

### Implementation Files

```
internal/cli/
├── tags.go              # TagsParser, CanonicalizeTag
├── tags_test.go         # Unit tests for parsing/validation
└── output.go            # formatTags(), formatTagsJSON()

cmd/
├── move.go              # Example: move --tags
├── create.go            # Other commands with tags
└── ...
```

### Documentation Files

```
ai-docs/research/cli/
├── tags-flag-design.md              # Full spec (comprehensive)
├── tags-implementation-guide.md     # Quick reference
├── tags-example-code.go             # Working example code
└── TAGS-FLAG-SUMMARY.md            # This file
```

---

## Usage Examples

### Basic Usage

```bash
# Single tag
wherehouse move socket Garage --tags urgent

# Multiple tags
wherehouse move socket Garage --tags urgent,tool,wrench

# Tags with spaces
wherehouse move key Safe --tags "House A",backup

# Tags with commas
wherehouse move item location --tags "tag,with,comma",other

# Multiple complex tags
wherehouse move item location \
  --tags "Project A","Budget 2026",urgent,pending
```

### JSON Output

```bash
wherehouse move socket Garage --tags urgent --json
```

Returns:

```json
{
  "item": {...},
  "location": {...},
  "tags": [
    {
      "display": "urgent",
      "canonical": "urgent"
    }
  ],
  "event_id": 42
}
```

### With Other Flags

```bash
# Move with project and tags
wherehouse move socket Garage --project toolroom --tags urgent

# Move with tags and quiet output
wherehouse move socket Garage --tags tool -q

# Move with tags, verbose, and JSON
wherehouse move socket Garage --tags urgent -vv --json
```

---

## Error Messages (User-Facing)

Clear, specific, actionable:

```
error: invalid tags: tag 0: colons not allowed \
  (reserved for selector syntax): "invalid:tag"

error: invalid tags: duplicate tag: "urgent"

error: invalid tags: tag 1: too long (max 100 chars, got 101): "..."

error: invalid tags: invalid tags format: ...
  use comma delimiter: --tags tag1,tag2,tag3
  quote values with commas: --tags "tag,with,comma",regular
```

---

## Implementation Checklist

### Phase 1: Core Parser (Must Complete)

- [ ] Create `internal/cli/tags.go`
- [ ] Implement `TagsParser` type
- [ ] Implement `Parse()` method (CSV-aware)
- [ ] Implement `Validate()` method (7 constraints)
- [ ] Implement `ParseAndValidate()` entry point
- [ ] Implement `CanonicalizeTag()` function
- [ ] Write unit tests
- [ ] Run `go test ./...`
- [ ] Run `go vet ./...`

### Phase 2: Command Integration (One Per Command)

For each command using `--tags`:

- [ ] Add flag definition in `init()`
- [ ] Add examples to Long description
- [ ] Parse with `TagsParser{Raw: tagsFlag}.ParseAndValidate()`
- [ ] Canonicalize before calling domain
- [ ] Format output (human and JSON)
- [ ] Write integration tests
- [ ] Verify help text
- [ ] Run linting

### Phase 3: Testing (All Commands)

- [ ] Unit tests for TagsParser
- [ ] Integration tests for each command
- [ ] Error message tests
- [ ] JSON output tests
- [ ] Help text verification
- [ ] Edge case tests (empty, whitespace, unicode)

---

## API Reference (Summary)

### TagsParser

```go
type TagsParser struct {
    Raw string  // e.g., "urgent,tool,backup"
}

// Parse returns parsed tags (not yet validated)
func (tp *TagsParser) Parse() ([]string, error)

// Validate checks constraints (runs 7 validation rules)
func (tp *TagsParser) Validate(tags []string) error

// ParseAndValidate (main entry point)
func (tp *TagsParser) ParseAndValidate() ([]string, error)
```

### CanonicalizeTag

```go
// Apply wherehouse naming rules to single tag
func CanonicalizeTag(tag string) string
// Input:  "High Priority"
// Output: "high_priority"
```

---

## Related Documentation

- **CLI Contract**: `.claude/knowledge/cli-contract.md`
- **Domain Model**: `.claude/knowledge/domain-model.md`
- **Business Rules**: `.claude/knowledge/business-rules.md`
- **Cobra Guide**: https://cobra.dev/

---

## Implementation Ready?

This design is **complete and ready for implementation**. All:

- Design decisions documented with rationale
- Validation rules specified with examples
- Code patterns provided with test templates
- Error messages designed for users
- Integration points defined
- Testing strategy outlined

**Next step**: golang-ui-developer implements based on:
1. `tags-flag-design.md` (understand why)
2. `tags-example-code.go` (copy patterns)
3. `tags-implementation-guide.md` (quick reference)

---

**Version**: 1.0
**Status**: Ready for Implementation
**Reviewed by**: golang-ui-developer agent (design phase)
