# Tags Flag Design Documentation

**Complete CLI flag pattern specification for `--tags` in wherehouse**

---

## Documents Overview

This folder contains comprehensive documentation for the `--tags` flag pattern, designed for CLI/TUI inventory tagging in the wherehouse project.

### Start Here

1. **TAGS-FLAG-SUMMARY.md** ← **START HERE**
   - Executive summary of the entire design
   - Key decisions and rationale
   - Integration points
   - Implementation checklist
   - ~10 min read

### For Understanding

2. **tags-quick-visual.md** ← **VISUAL LEARNERS**
   - Diagrams and flowcharts
   - Input/output examples
   - Decision trees
   - Visual walkthrough of flow
   - ~5 min read

3. **tags-flag-design.md** ← **DETAILED SPEC**
   - Complete specification with rationale
   - All validation rules explained
   - Test coverage examples
   - Error handling patterns
   - ~20 min read

### For Implementation

4. **tags-implementation-guide.md** ← **QUICK REFERENCE WHILE CODING**
   - Copy-paste code patterns
   - API reference
   - Common mistakes to avoid
   - Command integration checklist
   - ~10 min read

5. **tags-example-code.go** ← **WORKING EXAMPLE**
   - Complete TagsParser implementation
   - Canonicalization logic
   - Cobra command pattern
   - Unit test templates
   - Integration test templates
   - Output formatting examples
   - ~15 min review

---

## Recommended Reading Order

### For CLI/TUI Implementers (golang-ui-developer)

1. Read **TAGS-FLAG-SUMMARY.md** (understand the big picture)
2. Review **tags-quick-visual.md** (see visual flow)
3. Study **tags-example-code.go** (understand implementation)
4. Keep **tags-implementation-guide.md** open while coding
5. Reference **tags-flag-design.md** for edge cases

**Time estimate**: 40 minutes total

### For Code Reviewers

1. Read **TAGS-FLAG-SUMMARY.md** (understand design)
2. Review **tags-flag-design.md** (detailed spec)
3. Check test cases in **tags-example-code.go**
4. Verify error messages match spec

**Time estimate**: 30 minutes

### For Architecture Review

1. Read **TAGS-FLAG-SUMMARY.md** (decisions)
2. Check **Design Decisions** section in tags-flag-design.md
3. Review integration with domain layer
4. Verify adherence to CLI contract

**Time estimate**: 20 minutes

---

## What This Design Specifies

### User Interface

```bash
wherehouse move item location --tags urgent,tool,backup
wherehouse move key Safe --tags "tag,with,comma",regular
```

### Parsing Strategy

- CSV-aware parsing respects quoted values
- Comma is primary delimiter
- Validates 7 constraints before use

### Validation Rules

1. Non-empty tags
2. Maximum 100 characters per tag
3. No colons (reserved for selectors)
4. No duplicate tags
5. Valid UTF-8
6. Trimmed (no leading/trailing space)
7. Printable characters only

### Canonicalization

- Lowercase
- Spaces and dashes → underscores
- Collapse runs of underscores
- Consistent with item/location rules

### Output Modes

- **Human-readable**: "Tags: urgent, tool, backup"
- **JSON**: Structured with display and canonical forms
- **Quiet**: No output (exit code only)

### Error Handling

User-friendly error messages that:
- Cite specific constraint violated
- Provide example corrections
- Show exact problematic value

---

## Key Design Decisions (Summary)

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Flag style | Comma-separated | Unix convention, scriptable |
| Quote support | Yes, CSV-aware | Handle commas in tag values |
| Validation | Two-phase (parse/validate) | Clear error messages per phase |
| Canonicalization | Lowercase, spaces→underscores | Match item/location rules |
| Forbid colons | Yes | Prevent ambiguity with selector syntax |
| Duplicates allowed | No | Prevent user confusion |

---

## Integration Points

### With Cobra

```go
cmd.Flags().String("tags", "", "Comma-separated tags")
tagsFlag, _ := cmd.Flags().GetString("tags")
```

### With CLI Layer (internal/cli/)

```go
parser := &cli.TagsParser{Raw: tagsFlag}
tags, err := parser.ParseAndValidate()
canonical := cli.CanonicalizeTag(tag)
```

### With Domain Logic (golang-developer)

```go
domain.MoveItem(item, location, MoveOptions{
    Tags: canonicalTags,  // []string of canonical forms
})
```

### With Output Formatting

```go
// Human: strings.Join(tags, ", ")
// JSON: []TagOutput with display and canonical
// Quiet: (no output)
```

---

## File Locations (When Implemented)

```
internal/cli/
├── tags.go              # TagsParser, CanonicalizeTag
├── tags_test.go         # Unit tests (parsing, validation, canonicalization)
└── output.go            # formatTags functions (human, JSON)

cmd/
├── move.go              # Example: move --tags flag
└── [other_commands].go  # Other commands with tags (follow same pattern)
```

---

## Test Coverage

### Unit Tests (tags_test.go)

- **Parse tests**: 12 cases covering empty, single, multiple, quoted, errors
- **Validate tests**: 9 cases covering all 7 constraints
- **Canonicalize tests**: 6 cases covering transformation rules

Total: ~27 unit tests

### Integration Tests (cmd/*_test.go)

- **Command handler tests**: 8 cases per command with tags
- **Error message tests**: Verify user-friendly output
- **Output format tests**: JSON, human, quiet modes
- **Edge case tests**: Unicode, whitespace, special characters

Total: ~40+ integration tests per command

---

## Implementation Checklist

### Core Library (internal/cli/tags.go)

- [ ] Define TagsParser type
- [ ] Implement Parse() - CSV-aware parsing
- [ ] Implement Validate() - 7 constraint checks
- [ ] Implement ParseAndValidate() - main entry point
- [ ] Implement CanonicalizeTag() - naming rules
- [ ] Write 27+ unit tests
- [ ] Pass go vet
- [ ] Pass golangci-lint

### Per Command Integration

- [ ] Add flag definition
- [ ] Get flag value and parse
- [ ] Canonicalize tags
- [ ] Pass to domain layer
- [ ] Format output (human and JSON)
- [ ] Write integration tests (8+ cases)
- [ ] Verify help text
- [ ] Pass linting

---

## FAQ

### Q: Why CSV parsing instead of simple split?

A: CSV parsing respects quotes, so `"tag,with,comma"` is one tag. Manual split would incorrectly treat it as two tags.

### Q: Why forbid colons?

A: Colons are reserved for `LOCATION:ITEM` selector syntax. Allowing colons in tags creates ambiguity.

### Q: Why canonicalize tags?

A: Consistent with item/location naming rules. Prevents duplicates like "Urgent" and "urgent". Makes storage and querying deterministic.

### Q: Can users delete or rename tags?

A: This design handles flag input only. Tag lifecycle (edit, delete, bulk operations) is out of scope and can be designed separately.

### Q: What about tag suggestions/autocomplete?

A: Not included in initial design. Can be added later in completion layer (shell completion).

### Q: Can I use --tag (singular)?

A: Design uses --tags (plural). Could support --tag for single tags, but requires separate discussion.

---

## Design Philosophy

This design follows wherehouse principles:

- **Explicit over implicit**: Clear validation, not auto-repair
- **Deterministic**: CSV parsing always gives same result
- **User-friendly**: Clear error messages with examples
- **Consistent**: Follows existing domain model rules
- **Testable**: All functions are pure and mockable
- **Thin layer**: CLI only does parsing/validation, not business logic

---

## Validation Guarantee

If `TagsParser.ParseAndValidate()` returns successfully:

✓ Tags are syntactically valid (no CSV errors)
✓ No tag is empty
✓ No tag exceeds 100 characters
✓ No tag contains colons
✓ No duplicate tags (exact match on parsed form)
✓ All tags are valid UTF-8
✓ All tags are trimmed
✓ All tags are printable

**After canonicalization**: Can safely pass to domain layer.

---

## Error Path (What Goes Wrong)

```
User Input: --tags "bad:tag"
              ↓
Parse: OK → ["bad:tag"]
              ↓
Validate: FAIL (Rule 3: No colons)
              ↓
Error: "tag 0: colons not allowed (reserved for selector syntax): \"bad:tag\""
              ↓
Exit: 1 (failure)
              ↓
User sees clear message and knows how to fix
```

---

## Performance Characteristics

- **Parse**: O(n) where n = string length (CSV reader)
- **Validate**: O(m) where m = number of tags (linear checks)
- **Canonicalize**: O(m) where m = number of tags
- **Overall**: O(n + m), negligible for typical usage

No regex, no recursion, no algorithmic complexity.

---

## Future Extensions (Out of Scope)

These could be added later without breaking existing design:

1. **Tag categories**: `--tags @location:garage,@priority:urgent`
2. **Tag operators**: `--tags +add,-remove`
3. **Tag filtering**: `wherehouse list --with-tags urgent`
4. **Tag history**: Track tag changes in events
5. **Bulk tagging**: `wherehouse tag item1 item2 item3 --tags urgent`
6. **Tag aliases**: `--tags @toolroom` → expands to multiple tags
7. **Tag suggestions**: Based on item context or history
8. **Shell completion**: Auto-complete known tags with fzf

Current design supports these extensions cleanly.

---

## Document Index

| Document | Purpose | Audience | Read Time |
|----------|---------|----------|-----------|
| **TAGS-FLAG-SUMMARY.md** | Overview + decisions | Everyone | 10 min |
| **tags-quick-visual.md** | Visual guide | Visual learners | 5 min |
| **tags-flag-design.md** | Complete spec | Detailed review | 20 min |
| **tags-implementation-guide.md** | Quick reference | Implementers | 10 min |
| **tags-example-code.go** | Working code | Implementers | 15 min |
| **README-TAGS-FLAG.md** | This index | Navigation | 5 min |

**Total reading**: ~60 minutes for complete understanding
**Quick start**: 20 minutes (SUMMARY + VISUAL + GUIDE)

---

## Quick Links

- **Project docs**: `/home/grue/dev/wherehouse/docs/DESIGN.md`
- **CLI contract**: `/home/grue/dev/wherehouse/.claude/knowledge/cli-contract.md`
- **Domain model**: `/home/grue/dev/wherehouse/.claude/knowledge/domain-model.md`
- **Business rules**: `/home/grue/dev/wherehouse/.claude/knowledge/business-rules.md`

---

## Version & Status

- **Version**: 1.0
- **Status**: Design Complete, Ready for Implementation
- **Date**: 2026-02-20
- **Author**: golang-ui-developer agent (design phase)

---

## Next Steps

1. **Implementer**: Review TAGS-FLAG-SUMMARY.md + tags-example-code.go
2. **Implement**: Create internal/cli/tags.go with tests
3. **Integrate**: Add --tags flag to commands (move, create, etc.)
4. **Test**: Verify unit and integration tests pass
5. **Review**: Code review against tags-flag-design.md spec
6. **Release**: Deploy with help text and examples

---

**Status**: READY FOR IMPLEMENTATION
