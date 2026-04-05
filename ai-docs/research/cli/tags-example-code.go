// File: ai-docs/research/cli/tags-example-code.go
// Complete example implementation of --tags flag pattern
// This file is EXAMPLE CODE - copy patterns to actual implementation
// Build tag prevents this from being compiled with main package

//go:build exclude

package examples

import (
	"encoding/csv"
	"fmt"
	"strings"
	"unicode/utf8"
)

// ============================================================================
// TagsParser - Main parsing logic
// ============================================================================

// TagsParser handles comma-separated tags with quote awareness.
// Place this in: internal/cli/tags.go.
type TagsParser struct {
	Raw string // Raw --tags flag value from user input
}

// Parse returns []string of parsed tags (validated but not canonicalized).
//
// Parsing rules:
// - Comma is the primary delimiter
// - Double quotes allow embedding commas: "tag,with,comma"
// - Whitespace after commas is trimmed
//
// Examples:
//
//	""                           → []
//	"urgent"                     → ["urgent"]
//	"urgent,tool,backup"         → ["urgent", "tool", "backup"]
//	"\"tag,with,comma\",other"   → ["tag,with,comma", "other"]
//	"tag1 , tag2 , tag3"         → ["tag1", "tag2", "tag3"]
func (tp *TagsParser) Parse() ([]string, error) {
	if tp.Raw == "" {
		return []string{}, nil
	}

	// Use CSV reader to handle quotes properly
	reader := csv.NewReader(strings.NewReader(tp.Raw))
	reader.LazyQuotes = false // Strict quote validation
	reader.TrimLeadingSpace = true

	// Read all records (should be exactly 1 record = 1 line)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf(
			"invalid tags format: %w\n"+
				"  use comma delimiter: --tags tag1,tag2,tag3\n"+
				"  quote values with commas: --tags \"tag,with,comma\",regular",
			err,
		)
	}

	if len(records) != 1 {
		return nil, fmt.Errorf(
			"invalid tags format: expected single line (got %d lines)",
			len(records),
		)
	}

	return records[0], nil
}

// Validate checks each tag meets wherehouse constraints.
//
// Validation rules:
// 1. Not empty
// 2. Max 100 characters
// 3. No colons (reserved for LOCATION:ITEM selector syntax)
// 4. No duplicates (case-sensitive on parsed form)
// 5. Valid UTF-8
// 6. No leading/trailing whitespace (will be trimmed).
func (tp *TagsParser) Validate(tags []string) error {
	seen := make(map[string]bool)

	for i, tag := range tags {
		// Check for empty tag
		if tag == "" {
			return fmt.Errorf("tag %d: empty tag not allowed", i)
		}

		// Check length
		if len(tag) > 100 {
			return fmt.Errorf(
				"tag %d: too long (max 100 chars, got %d): %q",
				i, len(tag), tag,
			)
		}

		// Check for colons (reserved for selectors)
		if strings.Contains(tag, ":") {
			return fmt.Errorf(
				"tag %d: colons not allowed (reserved for item selector syntax): %q",
				i, tag,
			)
		}

		// Check UTF-8 validity
		if !utf8.ValidString(tag) {
			return fmt.Errorf("tag %d: invalid UTF-8: %q", i, tag)
		}

		// Check for leading/trailing whitespace
		if tag != strings.TrimSpace(tag) {
			return fmt.Errorf(
				"tag %d: remove leading/trailing whitespace: %q",
				i, tag,
			)
		}

		// Check for duplicates
		if seen[tag] {
			return fmt.Errorf("duplicate tag: %q", tag)
		}
		seen[tag] = true
	}

	return nil
}

// ParseAndValidate is the main entry point.
// Returns tags in parsed form (ready for canonicalization).
// Call CanonicalizeTag() on each result before storing.
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

// ============================================================================
// Canonicalization - Apply wherehouse naming rules
// ============================================================================

// CanonicalizeTag applies wherehouse naming rules to a single tag.
//
// Rules (matching item/location canonicalization):
// - Lowercase
// - Spaces → underscores
// - Dashes → underscores
// - Collapse runs of underscores
//
// Examples:
//
//	"Urgent"              → "urgent"
//	"High Priority"       → "high_priority"
//	"tool-collection"     → "tool_collection"
//	"HIGH__PRIORITY"      → "high_priority"
//	"  spaces  "          → "spaces" (after trim)
func CanonicalizeTag(tag string) string {
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

// ============================================================================
// Example: Integration with Cobra Command
// ============================================================================

// Example command handler showing how to use TagsParser
//
// Place this pattern in: internal/cli/move.go (or similar)
//
// Usage example:
//
//	wherehouse move "10mm socket" Garage --tags urgent,tool
//	wherehouse move key Safe --tags "tag,with,comma",regular
func exampleMoveCommandHandler(tagsFlag string) error {
	// Step 1: Parse and validate tags
	parser := &TagsParser{Raw: tagsFlag}
	tags, err := parser.ParseAndValidate()
	if err != nil {
		// Return user-friendly error
		return fmt.Errorf("invalid tags: %w", err)
	}

	// Step 2: Canonicalize tags for storage
	canonicalTags := make([]string, len(tags))
	for i, tag := range tags {
		canonicalTags[i] = CanonicalizeTag(tag)
	}

	// Step 3: Pass to domain logic
	// (golang-developer implements this)
	// result, err := domain.MoveItem(itemID, locationID, MoveOptions{
	//     Tags: canonicalTags,
	// })

	// Step 4: Format output (see below)
	_ = canonicalTags // Use in formatting

	return nil
}

// ============================================================================
// Output Formatting Examples
// ============================================================================

// FormatTagsHuman formats tags for human-readable output
// Example: "urgent, tool, backup".
func FormatTagsHuman(tags []string) string {
	if len(tags) == 0 {
		return ""
	}
	return strings.Join(tags, ", ")
}

// FormatTagsJSON formats tags for JSON output
// Includes both display and canonical forms (if stored).
type TagOutput struct {
	Display   string `json:"display"`
	Canonical string `json:"canonical"`
}

func FormatTagsJSON(displayTags, canonicalTags []string) []TagOutput {
	result := make([]TagOutput, len(canonicalTags))
	for i, canonical := range canonicalTags {
		result[i] = TagOutput{
			Display:   displayTags[i],
			Canonical: canonical,
		}
	}
	return result
}

// ============================================================================
// Example: Cobra Command Definition
// ============================================================================

// exampleCommandStructure shows how to define a command with --tags flag
//
// Pattern to follow in cmd/*.go:
//
//   var moveCmd = &cobra.Command{
//       Use:   "move ITEM LOCATION [flags]",
//       Short: "Move an item to a new location",
//       Long: `Move an item to a different location.
//
//   Tags can be applied with comma-separated values:
//
//     wherehouse move socket Garage --tags urgent,tool
//     wherehouse move key Safe --tags "tag,with,comma",regular
//
//   Quoted values allow embedding commas in tag names.`,
//
//       Args: cobra.ExactArgs(2),
//       RunE: runMove,
//   }
//
//   func init() {
//       rootCmd.AddCommand(moveCmd)
//
//       moveCmd.Flags().String(
//           "tags",
//           "",
//           "Comma-separated tags to apply (quote values with commas)",
//       )
//   }
//
//   func runMove(cmd *cobra.Command, args []string) error {
//       tagsFlag, _ := cmd.Flags().GetString("tags")
//
//       // Parse and validate
//       parser := &TagsParser{Raw: tagsFlag}
//       tags, err := parser.ParseAndValidate()
//       if err != nil {
//           return fmt.Errorf("invalid tags: %w", err)
//       }
//
//       // Canonicalize
//       canonicalTags := make([]string, len(tags))
//       for i, tag := range tags {
//           canonicalTags[i] = CanonicalizeTag(tag)
//       }
//
//       // Call domain logic (golang-developer implements this)
//       itemSelector := args[0]
//       locationSelector := args[1]
//       result, err := domain.MoveItem(itemSelector, locationSelector, MoveOptions{
//           Tags: canonicalTags,
//       })
//       if err != nil {
//           return err
//       }
//
//       // Format output
//       if jsonOutput {
//           return outputJSON(result)
//       }
//       return outputHuman(result)
//   }

// ============================================================================
// Example Test Cases
// ============================================================================

// Example unit tests for TagsParser
// Place in: internal/cli/tags_test.go.
func exampleTestTagsParser() {
	tests := []struct {
		name    string
		input   string
		want    []string
		wantErr bool
	}{
		{
			name:    "empty input",
			input:   "",
			want:    []string{},
			wantErr: false,
		},
		{
			name:    "single tag",
			input:   "urgent",
			want:    []string{"urgent"},
			wantErr: false,
		},
		{
			name:    "multiple tags",
			input:   "urgent,tool,backup",
			want:    []string{"urgent", "tool", "backup"},
			wantErr: false,
		},
		{
			name:    "quoted tag with comma",
			input:   `"tag,with,comma",regular`,
			want:    []string{"tag,with,comma", "regular"},
			wantErr: false,
		},
		{
			name:    "whitespace handling",
			input:   "tag1 , tag2 , tag3",
			want:    []string{"tag1", "tag2", "tag3"},
			wantErr: false,
		},
		{
			name:    "unclosed quote - error",
			input:   `"unclosed`,
			want:    nil,
			wantErr: true,
		},
	}

	_ = tests // Use in actual test implementation
}

func exampleTestValidation() {
	tests := []struct {
		name    string
		tags    []string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid single tag",
			tags:    []string{"urgent"},
			wantErr: false,
		},
		{
			name:    "valid multiple tags",
			tags:    []string{"urgent", "tool", "backup"},
			wantErr: false,
		},
		{
			name:    "empty tag - error",
			tags:    []string{"urgent", "", "backup"},
			wantErr: true,
			errMsg:  "empty tag",
		},
		{
			name:    "tag too long - error",
			tags:    []string{strings.Repeat("a", 101)},
			wantErr: true,
			errMsg:  "too long",
		},
		{
			name:    "colon in tag - error",
			tags:    []string{"invalid:tag"},
			wantErr: true,
			errMsg:  "colons not allowed",
		},
		{
			name:    "duplicate tags - error",
			tags:    []string{"urgent", "tool", "urgent"},
			wantErr: true,
			errMsg:  "duplicate tag",
		},
	}

	_ = tests // Use in actual test implementation
}

func exampleTestCanonical() {
	tests := []struct {
		input string
		want  string
	}{
		{"Urgent", "urgent"},
		{"High Priority", "high_priority"},
		{"tool-collection", "tool_collection"},
		{"HIGH__PRIORITY", "high_priority"},
		{"  spaces  ", "spaces"},
	}

	_ = tests // Use in actual test implementation
}

// ============================================================================
// Integration Point: Domain Layer
// ============================================================================

// The CLI layer (this code) passes canonicalized tags to the domain layer:
//
//   domain.MoveOptions{
//       Tags: []string{"urgent", "tool_collection"},  // Canonicalized
//   }
//
// The domain layer (golang-developer) is responsible for:
// 1. Creating events with tag data
// 2. Storing tags in projections
// 3. Querying items by tags (if needed)
//
// The CLI is ONLY responsible for:
// 1. Parsing comma-separated input
// 2. Validating format constraints
// 3. Canonicalizing tags
// 4. Formatting output

// ============================================================================
// Help Text Template
// ============================================================================

const exampleHelpText = `
Examples:

  # Single tag
  wherehouse move socket Garage --tags urgent

  # Multiple tags
  wherehouse move socket Garage --tags urgent,tool,backup

  # Tags with spaces (use quotes)
  wherehouse move key Safe --tags "House A",backup

  # Tags with commas (use quotes)
  wherehouse move key Safe --tags "tag,with,comma",regular

  # Multiple complex tags
  wherehouse move item location --tags "Project A","Budget 2026",urgent

Tags follow naming rules:
  - Lowercase (High → high)
  - Spaces to underscores (High Priority → high_priority)
  - Max 100 characters
  - No colons (reserved for item selector syntax)
  - No duplicates
`
