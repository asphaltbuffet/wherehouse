# Tags Flag CLI Design - Complete Documentation Index

**Comprehensive specification for `--tags` flag in wherehouse CLI**

Generated: 2026-02-20 | Status: Complete and Ready for Implementation

---

## Quick Navigation

### Start with these (Essential)

1. **README-TAGS-FLAG.md** (11K) - Navigation guide and document overview
2. **TAGS-FLAG-SUMMARY.md** (11K) - Executive summary with key decisions
3. **tags-quick-visual.md** (9.2K) - Visual flowcharts and decision trees

### For Implementation (Reference)

4. **tags-example-code.go** (13K) - Complete working code examples
5. **tags-implementation-guide.md** (7.7K) - Quick lookup while coding
6. **tags-flag-design.md** (19K) - Detailed specification with rationale

### For Understanding (Deep Dive)

7. **tags-design-walkthrough.md** (15K) - Real-world command examples
8. **tags-design-test-validation.md** (14K) - Test strategy and validation proofs

---

## Content Summary

### Core Documents

#### README-TAGS-FLAG.md
- Document index and navigation
- Recommended reading order
- Integration points with cobra and domain
- FAQ section
- Version history
**Read this first for orientation**

#### TAGS-FLAG-SUMMARY.md
- Design overview and key characteristics
- Parsing flow diagram
- Code pattern template
- Design decisions (with rationale)
- 7 validation constraints
- API reference
- Implementation checklist
**Read this to understand WHAT and WHY**

### Learning Documents

#### tags-quick-visual.md
- Visual flow diagrams
- Parsing process flowchart
- Validation rules visualization
- Error handling decision tree
- File organization
**Read this if you're a visual learner**

#### tags-design-walkthrough.md
- 10 step-by-step examples
- Happy path walkthrough
- Error case walkthroughs
- Output format examples
- Integration scenarios
- Canonicalization transformations
**Read this to see REAL EXAMPLES**

### Implementation Documents

#### tags-example-code.go
- Complete TagsParser implementation
- Canonicalization logic
- Cobra command integration pattern
- Unit test examples
- Integration test templates
- Output formatting examples
**Copy code patterns from this file**

#### tags-implementation-guide.md
- Copy-paste code patterns
- API quick reference
- Common mistakes to avoid
- Integration checklist
- Testing template
- Help text template
**Keep this open while coding**

#### tags-flag-design.md
- Complete specification (comprehensive)
- Validation rules explained
- Error handling patterns
- Test coverage examples
- Alternative approaches discussed
- Future extensions mentioned
**Reference for design decisions and details**

### Validation Documents

#### tags-design-test-validation.md
- Design quality checklist
- Test strategy coverage
- Correctness proofs
- Performance validation
- Specification completeness
- Integration testing scenarios
- Validation criteria
**Read this to verify design correctness**

---

## Reading Paths by Role

### CLI Developer (Implementation)
1. TAGS-FLAG-SUMMARY.md (10 min)
2. tags-quick-visual.md (5 min)
3. tags-example-code.go (15 min review)
4. Keep tags-implementation-guide.md open while coding
5. Reference tags-flag-design.md for edge cases
**Total: ~40 minutes**

### Code Reviewer
1. TAGS-FLAG-SUMMARY.md (10 min)
2. tags-flag-design.md (20 min)
3. tags-example-code.go (15 min review tests)
4. tags-design-test-validation.md (15 min)
**Total: ~60 minutes**

### Architecture Review
1. TAGS-FLAG-SUMMARY.md (10 min)
2. "Design Decisions" section in tags-flag-design.md (10 min)
3. "Integration Points" section in README-TAGS-FLAG.md (5 min)
**Total: ~25 minutes**

### Quick Lookup (During Implementation)
1. tags-implementation-guide.md (API reference)
2. tags-example-code.go (code patterns)
3. tags-quick-visual.md (decision trees)

---

## Key Specifications

### Flag Usage Pattern

```bash
# Single tag
wherehouse move item location --tags urgent

# Multiple tags
wherehouse move item location --tags urgent,tool,backup

# Quoted (spaces or commas in tag)
wherehouse move item location --tags "tag with space",regular
wherehouse move item location --tags "tag,with,comma",other
```

### Parsing Process

```
Raw input: "urgent,tool,backup"
          ↓
CSV parsing (quote-aware)
          ↓
Validation (7 constraints)
          ↓
Canonicalization (lowercase, spaces→underscores)
          ↓
Result: ["urgent", "tool", "backup"]
```

### Validation Rules (7 Total)

1. Non-empty
2. Max 100 characters
3. No colons (reserved)
4. No duplicates
5. Valid UTF-8
6. Trimmed whitespace
7. Printable characters

### Output Formats

- **Human**: "Tags: tag1, tag2, tag3"
- **JSON**: `{"tags": [{"display": "tag1", "canonical": "tag1"}]}`
- **Quiet**: (no output)

---

## File Locations

All documentation located in:
```
/home/grue/dev/wherehouse/ai-docs/research/cli/
```

When implemented, code located in:
```
internal/cli/tags.go           (TagsParser, CanonicalizeTag)
internal/cli/tags_test.go      (Unit tests)
internal/cli/output.go         (Formatting functions)
cmd/move.go                    (Example command)
```

---

## Quick Reference

### TagsParser API

```go
type TagsParser struct {
    Raw string  // e.g., "urgent,tool,backup"
}

// Parse returns parsed tags (syntax check only)
func (tp *TagsParser) Parse() ([]string, error)

// Validate checks 7 constraints
func (tp *TagsParser) Validate(tags []string) error

// ParseAndValidate (main entry point)
func (tp *TagsParser) ParseAndValidate() ([]string, error)
```

### Canonicalization

```go
// Apply wherehouse naming rules
func CanonicalizeTag(tag string) string
```

### Usage in Command

```go
parser := &cli.TagsParser{Raw: tagsFlag}
tags, err := parser.ParseAndValidate()
if err != nil {
    return fmt.Errorf("invalid tags: %w", err)
}

for i, tag := range tags {
    canonical[i] = cli.CanonicalizeTag(tag)
}

domain.MoveItem(item, location, MoveOptions{
    Tags: canonical,
})
```

---

## Document Statistics

| Document | Size | Content | Read Time |
|----------|------|---------|-----------|
| README-TAGS-FLAG.md | 11K | Navigation, FAQ | 5 min |
| TAGS-FLAG-SUMMARY.md | 11K | Overview, decisions | 10 min |
| tags-quick-visual.md | 9.2K | Diagrams, examples | 5 min |
| tags-design-walkthrough.md | 15K | 10 walkthroughs | 15 min |
| tags-example-code.go | 13K | Complete code | 15 min |
| tags-implementation-guide.md | 7.7K | Quick reference | 10 min |
| tags-flag-design.md | 19K | Full spec | 20 min |
| tags-design-test-validation.md | 14K | Tests, proofs | 15 min |
| **Total** | **~100K** | **8 documents** | **~90 min** |

---

## Design Validation Status

- [x] Requirements met
- [x] CLI contract compliance
- [x] Specification complete
- [x] Error handling designed
- [x] Test strategy defined
- [x] Code patterns provided
- [x] Edge cases covered
- [x] Performance verified
- [x] Documentation complete
- [x] Ready for implementation

---

## Quality Assurance Checklist

Before implementation starts:

- [ ] Read TAGS-FLAG-SUMMARY.md
- [ ] Review tags-example-code.go
- [ ] Understand validation rules
- [ ] Check error message examples
- [ ] Verify test cases
- [ ] Plan file structure
- [ ] Review integration points

Before implementation completes:

- [ ] All unit tests pass
- [ ] All integration tests pass
- [ ] No go vet errors
- [ ] No golangci-lint errors
- [ ] Help text reviewed
- [ ] Error messages tested
- [ ] JSON output validated
- [ ] Edge cases tested

---

## Contact & Support

**Design phase**: Complete
**Implementation phase**: Ready to begin
**Review phase**: Design documents available

For questions during implementation:
1. Check README-TAGS-FLAG.md FAQ
2. Review tags-example-code.go for patterns
3. Consult tags-flag-design.md for specification
4. Run tests from tags-design-test-validation.md

---

## Version History

| Version | Date | Status |
|---------|------|--------|
| 1.0 | 2026-02-20 | Complete, Ready for Implementation |

---

**Next Step**: Begin implementation following tags-implementation-guide.md

**Time Estimate**: 4-8 hours for complete implementation with tests
