# Tags Flag Design - Real-World Walkthrough

**Step-by-step examples of how the `--tags` flag works in practice**

---

## Walkthrough 1: Simple Single Tag

### User Types

```bash
wherehouse move socket Garage --tags urgent
```

### System Processing

```
Step 1: Shell parsing
  Raw input to program: ["move", "socket", "Garage", "--tags", "urgent"]

Step 2: Cobra flag parsing
  tagsFlag = "urgent"

Step 3: CLI layer parsing
  parser := &TagsParser{Raw: "urgent"}
  tags, err := parser.ParseAndValidate()

  Parse: CSV reader splits on commas → ["urgent"]
  Validate:
    ✓ Not empty: "urgent" has 6 chars
    ✓ Max 100: 6 < 100
    ✓ No colons: no ":" found
    ✓ No duplicates: first and only occurrence
    ✓ Valid UTF-8: ASCII is valid UTF-8
    ✓ Trimmed: no leading/trailing space
    ✓ Printable: all visible chars
  Result: ["urgent"]

Step 4: Canonicalization
  for i, tag := range tags {
    canonical[i] = CanonicalizeTag(tag)
  }

  Input: "urgent"
  - Trim: "urgent"
  - Lowercase: "urgent"
  - Replace spaces/dashes: "urgent" (none)
  - Collapse underscores: "urgent" (none)
  Output: "urgent"

  Result: ["urgent"]

Step 5: Domain layer
  result, err := domain.MoveItem("socket", "Garage", MoveOptions{
      Tags: ["urgent"],
  })

Step 6: Output formatting
  if jsonOutput {
      // JSON mode
      {
        "item": {"id": "uuid-1", "display_name": "10mm socket"},
        "location": {"id": "uuid-2", "display_name": "Garage"},
        "tags": [{"display": "urgent", "canonical": "urgent"}],
        "event_id": 42
      }
  } else {
      // Human mode
      printf("Moved \"10mm socket\" to Garage\n")
      printf("Tags: urgent\n")
  }
```

### User Sees

```
Moved "10mm socket" to Garage
Tags: urgent
```

---

## Walkthrough 2: Multiple Tags

### User Types

```bash
wherehouse move socket Garage --tags urgent,tool,wrench
```

### System Processing

```
Step 1-2: Shell and Cobra parse
  tagsFlag = "urgent,tool,wrench"

Step 3: CLI parsing
  parser := &TagsParser{Raw: "urgent,tool,wrench"}

  Parse: CSV splits on commas
    "urgent" | "tool" | "wrench"
    → ["urgent", "tool", "wrench"]

  Validate each tag:
    Tag 0: "urgent"
      ✓ All rules pass
    Tag 1: "tool"
      ✓ All rules pass
    Tag 2: "wrench"
      ✓ All rules pass
  Result: ["urgent", "tool", "wrench"]

Step 4: Canonicalize each
  "urgent" → "urgent"
  "tool" → "tool"
  "wrench" → "wrench"
  Result: ["urgent", "tool", "wrench"]

Step 5: Domain
  MoveItem(socket, Garage, MoveOptions{
    Tags: ["urgent", "tool", "wrench"],
  })

Step 6: Output
  "Moved \"10mm socket\" to Garage"
  "Tags: urgent, tool, wrench"
```

### User Sees

```
Moved "10mm socket" to Garage
Tags: urgent, tool, wrench
```

---

## Walkthrough 3: Tags with Spaces (Quoted)

### User Types

```bash
wherehouse move key Safe --tags "House A",backup
```

### System Processing

```
Step 1-2: Shell and Cobra parse
  Shell preserves quoted string: "House A"
  tagsFlag = "\"House A\",backup"

  Note: Actual string in Go is: House A,backup
        (shell removes quotes)

Step 3: CLI parsing
  parser := &TagsParser{Raw: "House A,backup"}

  Parse: CSV reader
    Sees: House A,backup
    Problem: Is this "House A" and "backup" (2 tags)?
             Or "House A,backup" (1 tag with comma)?

    Solution: If user wants space in tag, they don't quote!
             CSV doesn't add quote escaping.
             Quotes are only used for commas.

    Actual usage:
    wherehouse move key Safe --tags "House A",backup
                              ↑ shell strips quotes

    To CLI receives: House A,backup (unquoted A)
    CSV splits on comma: ["House A", "backup"]

  Validate:
    Tag 0: "House A"
      ✓ Not empty
      ✓ Max 100 chars
      ✓ No colons
      ✓ Valid UTF-8
      ✓ Trimmed (whitespace inside is OK; leading/trailing checked)
      ✓ All rules pass

    Tag 1: "backup"
      ✓ All rules pass

  Result: ["House A", "backup"]

Step 4: Canonicalize each
  "House A" → "house_a" (lowercase, space→underscore)
  "backup" → "backup"
  Result: ["house_a", "backup"]

Step 5: Domain
  MoveItem(key, Safe, MoveOptions{
    Tags: ["house_a", "backup"],
  })

Step 6: Output
  "Moved \"spare key\" to Safe"
  "Tags: house_a, backup"
```

### Important Note About Quoting

The shell handles quotes, not the CLI program. Example:

```bash
# With quotes (shell removes them)
wherehouse move item loc --tags "House A",backup
CLI receives: House A,backup (no quotes in string)

# CSV sees: House A,backup
# Splits: ["House A", "backup"]

# Alternative: User can NOT quote if no commas
wherehouse move item loc --tags "House A",backup
# Result same as above
```

---

## Walkthrough 4: Tags with Commas (Complex)

### User Types

```bash
wherehouse move item loc --tags "tag,with,comma",other
```

### System Processing

```
Step 1-2: Shell and Cobra parse
  User types: --tags "tag,with,comma",other
  Shell removes quotes from tagged value

  tagsFlag = "tag,with,comma,other"

  Wait! The shell removed the inner quotes!
  Now it looks like 4 separate tags!

PROBLEM: Without shell-preserved quoting, CSV can't parse correctly.

SOLUTION: User must escape for shell, or use different quoting:

Option A: Single quotes (shell preserves inner double quotes)
  wherehouse move item loc --tags '"tag,with,comma",other'
  Shell receives and passes: "tag,with,comma",other

  CSV parses:
    "tag,with,comma" → tag,with,comma (quoted value)
    other            → other

  Result: ["tag,with,comma", "other"]

Option B: Bash -c (full control)
  $ bash -c 'wherehouse move item loc --tags "tag,with,comma",other'

Option C: In scripts, use variable
  tags='"tag,with,comma",other'
  wherehouse move item loc --tags "$tags"
```

### Correct Command

```bash
# With single quotes (preserves double quotes inside)
wherehouse move item loc --tags '"tag,with,comma",other'
```

### System Processing (Correct)

```
Step 1-2: Shell and Cobra parse
  tagsFlag = '"tag,with,comma",other'

Step 3: CLI parsing
  parser := &TagsParser{Raw: '"tag,with,comma",other'}

  Parse: CSV reader
    Input: "tag,with,comma",other
    CSV field 1: tag,with,comma (inside quotes, so comma is literal)
    CSV field 2: other
    Result: ["tag,with,comma", "other"]

  Validate:
    Tag 0: "tag,with,comma"
      ✓ Not empty (14 chars)
      ✓ Max 100 chars
      ✓ No colons
      ✓ Commas are fine (they're part of tag value, not delimiters)
      ✓ All rules pass

    Tag 1: "other"
      ✓ All rules pass

  Result: ["tag,with,comma", "other"]

Step 4: Canonicalize
  "tag,with,comma" → "tag_with_comma" (comma stays in value!)
                     Wait... that's wrong.

  Actually: CanonicalizeTag() only handles spaces/dashes
            No rule for commas!
            So commas are preserved in canonical form.

  "tag,with,comma" → "tag_with_comma" (only spaces→underscores)
  "other" → "other"
  Result: ["tag_with_comma", "other"]

Step 5-6: Domain and output
  Tags stored: ["tag_with_comma", "other"]
  Output: "Tags: tag_with_comma, other"
```

### User Sees

```
Moved item to location
Tags: tag_with_comma, other
```

---

## Walkthrough 5: Error Case - Colon in Tag

### User Types

```bash
wherehouse move socket Garage --tags invalid:tag
```

### System Processing (FAILURE)

```
Step 1-2: Shell and Cobra parse
  tagsFlag = "invalid:tag"

Step 3: CLI parsing
  parser := &TagsParser{Raw: "invalid:tag"}

  Parse: CSV reader
    Input: "invalid:tag"
    Result: ["invalid:tag"]

  Validate: Check constraint 3 (no colons)
    Tag 0: "invalid:tag"
      ✓ Not empty
      ✓ Max 100 chars
      ✗ CONTAINS COLON: Has ":"

    ERROR: Tag violates constraint 3
    Message: "tag 0: colons not allowed (reserved for selector syntax): \"invalid:tag\""

Step 4: Error handling
  return fmt.Errorf("invalid tags: %w", err)

  Output to stderr:
    error: invalid tags: tag 0: colons not allowed \
      (reserved for selector syntax): "invalid:tag"

Step 5: Exit
  Exit code: 1 (failure)
```

### User Sees

```
error: invalid tags: tag 0: colons not allowed \
  (reserved for selector syntax): "invalid:tag"
```

**Action**: User understands colons are not allowed and corrects input.

---

## Walkthrough 6: Error Case - Duplicate Tags

### User Types

```bash
wherehouse move socket Garage --tags urgent,tool,urgent
```

### System Processing (FAILURE)

```
Step 1-2: Shell and Cobra parse
  tagsFlag = "urgent,tool,urgent"

Step 3: CLI parsing
  parser := &TagsParser{Raw: "urgent,tool,urgent"}

  Parse: CSV reader
    Result: ["urgent", "tool", "urgent"]

  Validate: Check constraint 4 (no duplicates)
    Tag 0: "urgent" → add to seen map
    Tag 1: "tool" → add to seen map
    Tag 2: "urgent" → ALREADY IN MAP!

    ERROR: Duplicate tag detected
    Message: "duplicate tag: \"urgent\""

Step 4: Error handling
  return fmt.Errorf("invalid tags: %w", err)

  Output to stderr:
    error: invalid tags: duplicate tag: "urgent"

Step 5: Exit
  Exit code: 1 (failure)
```

### User Sees

```
error: invalid tags: duplicate tag: "urgent"
```

**Action**: User removes duplicate "urgent" and tries again.

---

## Walkthrough 7: JSON Output Mode

### User Types

```bash
wherehouse move socket Garage --tags urgent,tool --json
```

### System Processing

```
Steps 1-5: Same as Walkthrough 2
  Tags parsed: ["urgent", "tool"]
  Tags canonical: ["urgent", "tool"]
  Domain returns result

Step 6: Output formatting (JSON mode)
  if jsonOutput {
      type Result struct {
          Item Item `json:"item"`
          Location Location `json:"location"`
          Tags []TagOutput `json:"tags"`
          EventID int `json:"event_id"`
      }

      type TagOutput struct {
          Display string `json:"display"`
          Canonical string `json:"canonical"`
      }

      result := Result{
          Item: {ID: "uuid-1", DisplayName: "10mm socket", ...},
          Location: {ID: "uuid-2", DisplayName: "Garage", ...},
          Tags: []TagOutput{
              {Display: "urgent", Canonical: "urgent"},
              {Display: "tool", Canonical: "tool"},
          },
          EventID: 42,
      }

      encoder := json.NewEncoder(os.Stdout)
      encoder.SetIndent("", "  ")
      encoder.Encode(result)
  }
```

### User Sees

```json
{
  "item": {
    "id": "2850e47b-d20c-7f06-811b-5a37f70f6f06",
    "display_name": "10mm socket",
    "canonical_name": "10mm_socket"
  },
  "location": {
    "id": "f848e7b6-d8e2-7c25-bd1a-ea3f4d8f2a9b",
    "display_name": "Garage",
    "canonical_name": "garage"
  },
  "tags": [
    {
      "display": "urgent",
      "canonical": "urgent"
    },
    {
      "display": "tool",
      "canonical": "tool"
    }
  ],
  "event_id": 42
}
```

---

## Walkthrough 8: Canonicalization Examples

### Input → Canonical Transformations

```bash
# Example 1: Uppercase tag
wherehouse move item loc --tags URGENT
Parse: ["URGENT"]
Canonical: ["urgent"]
Output: "Tags: urgent"

# Example 2: Tag with spaces
wherehouse move item loc --tags "High Priority"
Parse: ["High Priority"]
Canonicalize:
  Input: "High Priority"
  Lowercase: "high priority"
  Spaces→underscores: "high_priority"
  Collapse: "high_priority" (no runs)
  Output: "high_priority"
Output: "Tags: high_priority"

# Example 3: Tag with dashes
wherehouse move item loc --tags tool-collection
Parse: ["tool-collection"]
Canonicalize:
  Input: "tool-collection"
  Lowercase: "tool-collection"
  Dashes→underscores: "tool_collection"
  Collapse: "tool_collection" (no runs)
  Output: "tool_collection"
Output: "Tags: tool_collection"

# Example 4: Mixed case with spaces and dashes
wherehouse move item loc --tags "High-Priority TASK"
Parse: ["High-Priority TASK"]
Canonicalize:
  Input: "High-Priority TASK"
  Lowercase: "high-priority task"
  Spaces/dashes→underscores: "high_priority_task"
  Collapse: "high_priority_task" (no runs)
  Output: "high_priority_task"
Output: "Tags: high_priority_task"

# Example 5: Collapse underscores
wherehouse move item loc --tags "high__priority"
Parse: ["high__priority"]
Canonicalize:
  Input: "high__priority"
  Lowercase: "high__priority"
  (no spaces/dashes)
  Collapse underscores: "high_priority"
  Output: "high_priority"
Output: "Tags: high_priority"
```

---

## Walkthrough 9: Integration with Other Flags

### User Types

```bash
wherehouse move socket Garage --tags urgent --project toolroom -v
```

### System Processing

```
Flag parsing:
  itemSelector: "socket"
  locationSelector: "Garage"
  tagsFlag: "urgent"
  projectFlag: "toolroom"
  verboseFlag: true

Tags processing:
  parser := &TagsParser{Raw: "urgent"}
  tags, err := parser.ParseAndValidate() → ["urgent"]
  canonical := "urgent"

Domain call:
  result, err := domain.MoveItem("socket", "Garage", MoveOptions{
      ProjectID: "toolroom",
      Tags: ["urgent"],
  })

Output formatting (verbose mode):
  // Default: simple confirmation
  Moved "10mm socket" to Garage
  Tags: urgent

  // -v: verbose output
  Item: 10mm socket (id: uuid-1)
  From: Garage Shelf B (id: uuid-2)
  To: Garage (id: uuid-3)
  Tags: urgent
  Project: toolroom
  Event ID: 42
  Timestamp: 2026-02-20T15:30:42Z
```

### User Sees

```
Item: 10mm socket (id: 2850e47b-d20c-7f06-811b-5a37f70f6f06)
From: Garage >> Shelf B (id: abc123)
To: Garage (id: def456)
Tags: urgent
Project: toolroom
Event ID: 42
Timestamp: 2026-02-20T15:30:42Z
```

---

## Walkthrough 10: Quiet Mode

### User Types

```bash
wherehouse move socket Garage --tags urgent -q
```

### System Processing

```
Flag parsing:
  quietFlag: true
  jsonFlag: false
  verboseFlag: 0

Tags processing:
  Parse, validate, canonicalize (same as usual)

Domain call:
  (same as usual)

Output formatting:
  if jsonOutput {
      // JSON mode
  } else if quiet {
      // Quiet mode: no output
      return nil
  } else {
      // Human-readable mode
      fmt.Printf("Moved \"10mm socket\" to Garage\n")
      fmt.Printf("Tags: urgent\n")
  }

Exit:
  return nil  // Exit code 0 (success)
```

### User Sees

```
(no output)
```

**Note**: Exit code is still 0 (success). User can check with `echo $?`

---

## Summary: Decision Flow

```
User Input
  ↓
Is flag provided?
  ├─ NO → empty list (0 tags) ✓ VALID
  └─ YES → Parse with CSV reader
      ↓
   Parse successful?
     ├─ NO → show CSV error, exit 1
     └─ YES → ["tag1", "tag2", ...]
         ↓
      Validate each tag
        ├─ Non-empty? ✓
        ├─ Max 100 chars? ✓
        ├─ No colons? ✓
        ├─ No duplicates? ✓
        ├─ Valid UTF-8? ✓
        ├─ Trimmed? ✓
        └─ Printable? ✓
           ↓
       All pass?
         ├─ NO → show specific error, exit 1
         └─ YES → Canonicalize each
             ↓
            Pass to domain
             ↓
            Format output
             ↓
            Exit 0 (success)
```

---

**These walkthroughs show**:
1. Happy path (single tag, multiple tags, complex tags)
2. Error paths (colon, duplicate)
3. Output modes (human, JSON, quiet)
4. Integration with other flags
5. Canonicalization transformations

**Next step**: Review code in `tags-example-code.go` with these walkthroughs in mind.
