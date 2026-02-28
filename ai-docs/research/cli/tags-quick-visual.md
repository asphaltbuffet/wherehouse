# Tags Flag - Quick Visual Guide

**At-a-glance reference** for the `--tags` flag design.

---

## How It Works (Visual)

```
User Input
│
├─ wherehouse move socket Garage --tags urgent,tool
│
└─→ Shell passes to CLI:
    tagsFlag = "urgent,tool"
       │
       ├─→ TagsParser{Raw: tagsFlag}.ParseAndValidate()
       │
       ├─ CSV Parse: "urgent,tool" → ["urgent", "tool"]
       │
       ├─ Validate:
       │  ✓ Not empty
       │  ✓ No colons
       │  ✓ No duplicates
       │  ✓ Valid UTF-8
       │  ✓ Trimmed
       │  ✓ Max 100 chars
       │  ✓ Printable
       │
       ├─ Return: ["urgent", "tool"]
       │
       ├─→ Canonicalize each:
       │  "urgent" → "urgent"
       │  "tool" → "tool"
       │
       ├─→ Pass to domain:
       │  domain.MoveItem(item, location, MoveOptions{
       │      Tags: ["urgent", "tool"]
       │  })
       │
       └─→ Format output
          "Moved socket to Garage"
          "Tags: urgent, tool"
```

---

## Input → Processing → Output

### Simple Case

```
Input:    --tags urgent
          ↓
Parse:    ["urgent"]
          ↓
Validate: ✓ OK
          ↓
Canonical: ["urgent"]
          ↓
Output:   "Tags: urgent"
```

### Complex Case (Quoted Comma)

```
Input:    --tags "tag,with,comma",regular
          ↓
Parse:    ["tag,with,comma", "regular"]
          ↓
Validate: ✓ OK
          ↓
Canonical: ["tag_with_comma", "regular"]
          ↓
Output:   "Tags: tag_with_comma, regular"
```

### Error Case (Colon)

```
Input:    --tags invalid:tag
          ↓
Parse:    ["invalid:tag"]
          ↓
Validate: ✗ COLON NOT ALLOWED
          ↓
Error:    "tag 0: colons not allowed (reserved for selector syntax): \"invalid:tag\""
          ↓
Exit:     1 (failure)
```

---

## Parsing Flow (Detailed)

```
Raw Input String
"urgent,tool,backup"
      │
      ├─→ csv.Reader.ReadAll()
      │   (respects quotes, handles escapes)
      │
      ├─ Split on commas (outside quotes)
      ├─ Trim whitespace
      │
      ├─→ Result: []string
      │   ["urgent", "tool", "backup"]
      │
      └─→ [PARSE COMPLETE]
```

### Parsing Scenarios

| Input | Parsed | Notes |
|-------|--------|-------|
| `urgent` | `["urgent"]` | Single tag |
| `urgent,tool` | `["urgent", "tool"]` | Multiple tags |
| `urgent , tool` | `["urgent", "tool"]` | Whitespace trimmed |
| `"tag,comma",other` | `["tag,comma", "other"]` | Quote-aware parsing |
| `""` | `[]` | Empty is OK (0 tags) |
| `"unclosed` | ERROR | Validation fails |

---

## Validation Flow (7 Rules)

```
Tag: "Urgent"
 │
 ├─ Rule 1: Non-empty?          ✓ "Urgent" has length
 ├─ Rule 2: Max 100 chars?      ✓ 6 < 100
 ├─ Rule 3: No colon?           ✓ No ":"
 ├─ Rule 4: No duplicate?       ✓ First occurrence
 ├─ Rule 5: Valid UTF-8?        ✓ ASCII subset of UTF-8
 ├─ Rule 6: Trimmed?            ✓ No leading/trailing space
 └─ Rule 7: Printable?          ✓ All visible chars
      │
      └─→ [VALIDATION PASSES]

Tag: "bad:tag"
 │
 ├─ Rule 1: Non-empty?          ✓
 ├─ Rule 2: Max 100 chars?      ✓
 ├─ Rule 3: No colon?           ✗ CONTAINS ":"
      │
      └─→ [VALIDATION FAILS]
          Error: "colons not allowed..."
```

---

## Canonicalization Examples

```
Input            Canonical       Explanation
─────────────────────────────────────────────────────
Urgent        → urgent           lowercase
High Priority → high_priority    space → underscore
tool-set      → tool_set         dash → underscore
MULTI__DASH   → multi_dash       collapse __ → _
  spaces      → spaces           trim whitespace
High-Priority → high_priority    dash → underscore
high__priority→ high_priority    collapse runs
Mixed Case_123→ mixed_case_123   lowercase, preserve numbers
```

---

## Code Integration (4 Steps)

### Step 1: Register Flag

```go
cmd.Flags().String("tags", "", "Comma-separated tags")
```

### Step 2: Get Flag Value

```go
tagsFlag, _ := cmd.Flags().GetString("tags")
```

### Step 3: Parse & Validate

```go
parser := &cli.TagsParser{Raw: tagsFlag}
tags, err := parser.ParseAndValidate()
if err != nil {
    return fmt.Errorf("invalid tags: %w", err)
}
```

### Step 4: Canonicalize & Use

```go
canonical := make([]string, len(tags))
for i, tag := range tags {
    canonical[i] = cli.CanonicalizeTag(tag)
}

result, err := domain.MoveItem(item, location, MoveOptions{
    Tags: canonical,
})
```

---

## Error Messages (User's Perspective)

### Error: Empty Tag

```
$ wherehouse move item loc --tags "tag1,,tag3"
error: invalid tags: tag 1: empty tag not allowed
```

### Error: Colon (Reserved)

```
$ wherehouse move item loc --tags "bad:tag"
error: invalid tags: tag 0: colons not allowed \
  (reserved for selector syntax): "bad:tag"
```

### Error: Too Long

```
$ wherehouse move item loc --tags "$(python -c 'print("a"*101)')"
error: invalid tags: tag 0: too long (max 100 chars, got 101): "..."
```

### Error: Duplicate

```
$ wherehouse move item loc --tags "urgent,tool,urgent"
error: invalid tags: duplicate tag: "urgent"
```

### Error: Format (CSV Parse)

```
$ wherehouse move item loc --tags '"unclosed'
error: invalid tags: invalid tags format: ...
  use comma delimiter: --tags tag1,tag2,tag3
  quote values with commas: --tags "tag,with,comma",regular
```

---

## Output Formatting (3 Modes)

### Human-Readable (Default)

```
$ wherehouse move socket Garage --tags urgent,tool
Moved "10mm socket" to Garage
Tags: urgent, tool
```

### JSON Output

```
$ wherehouse move socket Garage --tags urgent --json
{
  "item": {
    "id": "abc123",
    "display_name": "10mm socket",
    "canonical_name": "10mm_socket"
  },
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

### Quiet Mode

```
$ wherehouse move socket Garage --tags urgent -q
# (no output, just exit code 0)
```

---

## Flag Examples (Copy-Paste Ready)

```bash
# Single tag
wherehouse move socket Garage --tags urgent

# Multiple tags (comma-separated)
wherehouse move socket Garage --tags urgent,tool,wrench

# Tags with spaces (use quotes)
wherehouse move key Safe --tags "House A",backup

# Tags with commas (use quotes)
wherehouse move item location --tags "tag,with,comma",other

# Multiple complex tags
wherehouse move item location \
  --tags "Project A","Budget 2026",urgent,pending

# With JSON output
wherehouse move socket Garage --tags urgent --json

# With quiet output
wherehouse move socket Garage --tags urgent -q

# With other flags
wherehouse move socket Garage --tags urgent --project toolroom

# Verbose with tags
wherehouse move socket Garage --tags urgent -v
```

---

## Decision Tree (Is My Input Valid?)

```
START
  │
  ├─ Empty string?
  │  └─ YES → ✓ VALID (0 tags, empty list)
  │
  ├─ Has colons?
  │  └─ YES → ✗ INVALID (reserved for selector syntax)
  │
  ├─ Duplicate tags after parsing?
  │  └─ YES → ✗ INVALID (duplicates forbidden)
  │
  ├─ Any tag > 100 chars?
  │  └─ YES → ✗ INVALID (too long)
  │
  ├─ Unclosed quotes?
  │  └─ YES → ✗ INVALID (CSV parse error)
  │
  ├─ Any empty tag?
  │  └─ YES → ✗ INVALID (empty tags forbidden)
  │
  ├─ Invalid UTF-8?
  │  └─ YES → ✗ INVALID (not UTF-8)
  │
  └─ All checks pass?
     └─ YES → ✓ VALID (proceed to canonicalize)
```

---

## File Organization

```
Project Structure
├── internal/cli/
│   ├── tags.go              ← TagsParser, CanonicalizeTag()
│   ├── tags_test.go         ← Unit tests
│   └── output.go            ← Formatting functions
│
├── cmd/
│   ├── move.go              ← Example with --tags
│   └── ...
│
└── ai-docs/research/cli/
    ├── tags-flag-design.md              ← Full spec (WHY)
    ├── tags-implementation-guide.md     ← Quick ref (WHAT)
    ├── tags-example-code.go             ← Code examples (HOW)
    ├── tags-quick-visual.md             ← This file
    └── TAGS-FLAG-SUMMARY.md             ← Overview
```

---

## Checklist: Before Implementation

- [ ] Understand parsing flow (CSV-aware)
- [ ] Know 7 validation rules
- [ ] Understand canonicalization rules
- [ ] Know error messages to show users
- [ ] Know 3 output formats (human, JSON, quiet)
- [ ] Understand domain layer integration
- [ ] Review example code in tags-example-code.go
- [ ] Plan test cases (parsing, validation, canonicalization)

---

## Quick Reference: Common Commands

| Task | Command |
|------|---------|
| Parse tags | `parser := &TagsParser{Raw: tagsFlag}` |
| | `tags, err := parser.ParseAndValidate()` |
| Canonicalize | `canonical := cli.CanonicalizeTag(tag)` |
| Format human | `fmt.Sprintf("Tags: %s", strings.Join(tags, ", "))` |
| Format JSON | `json.Marshal(TagOutput{Display, Canonical})` |
| Handle error | `return fmt.Errorf("invalid tags: %w", err)` |

---

**For details**: See `tags-implementation-guide.md`
**For examples**: See `tags-example-code.go`
**For spec**: See `tags-flag-design.md`
