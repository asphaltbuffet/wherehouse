package cli

import (
	"context"
	"encoding/csv"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// LoadResult holds the outcome of loading a single CSV file.
type LoadResult struct {
	Path           string         `json:"path"`
	ItemCount      int            `json:"item_count"`
	LocationCount  int            `json:"location_count"`
	InvalidEntries []InvalidEntry `json:"invalid_entries"`
}

// InvalidEntry records a row that could not be loaded.
type InvalidEntry struct {
	Line  int    `json:"line"`  // 1-based, counting the header row as line 1
	Entry string `json:"entry"` // value of the name field, or empty string if name was missing
	Error string `json:"error"` // human-readable reason
}

// loadDB is the combined interface needed to load CSV rows.
// It satisfies both addLocationsDB and addItemsDB.
type loadDB interface {
	Close() error
	LocationItemQuerier
	ValidateLocationExists(ctx context.Context, locationID string) error
	ValidateUniqueLocationName(ctx context.Context, canonicalName string, excludeLocationID *string) error
	AppendEvent(
		ctx context.Context,
		eventType database.EventType,
		actorUserID string,
		payload any,
		note string,
	) (int64, error)
}

// LoadCSV parses a CSV file and inserts valid locations and items into the database.
// It returns a hard error only when the file cannot be opened or has the wrong extension.
// Individual row problems are recorded as InvalidEntry values in the result.
//
// CSV format (header required, column order flexible):
//
//	type - "L" (location) or "I" (item)
//	name - display name (required)
//	home - parent location name (optional for locations, required for items)
func LoadCSV(ctx context.Context, fp string) (*LoadResult, error) {
	abs, err := filepath.Abs(fp)
	if err != nil {
		return nil, err
	}

	if filepath.Ext(abs) != ".csv" {
		return nil, fmt.Errorf("invalid extension: %q (expected .csv)", filepath.Ext(abs))
	}

	db, err := OpenDatabase(ctx)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	return loadCSV(ctx, db, abs, fp)
}

// loadCSV is the injectable inner function used by LoadCSV and tests.
func loadCSV(ctx context.Context, db loadDB, abs, displayPath string) (*LoadResult, error) {
	f, err := os.Open(abs)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	result := &LoadResult{Path: displayPath}
	r := csv.NewReader(f)
	r.FieldsPerRecord = -1

	header, err := r.Read()
	if err != nil {
		if errors.Is(err, io.EOF) {
			return result, nil
		}
		return nil, fmt.Errorf("reading header: %w", err)
	}

	colIndex := buildColIndex(header)

	lineNum := 1 // header was line 1
	for {
		lineNum++
		row, readErr := r.Read()
		if errors.Is(readErr, io.EOF) {
			break
		}
		if readErr != nil {
			result.InvalidEntries = append(result.InvalidEntries, InvalidEntry{
				Line:  lineNum,
				Error: fmt.Sprintf("malformed CSV row: %v", readErr),
			})
			continue
		}

		if isSkippableRow(row) {
			lineNum--
			continue
		}

		rowType := strings.ToUpper(colValue(row, colIndex, "type"))
		name := colValue(row, colIndex, "name")
		home := colValue(row, colIndex, "home")

		if inv := validateName(lineNum, name); inv != nil {
			result.InvalidEntries = append(result.InvalidEntries, *inv)
			continue
		}

		locAdded, itemAdded, inv := processRow(ctx, db, lineNum, rowType, name, home)
		if inv != nil {
			result.InvalidEntries = append(result.InvalidEntries, *inv)
			continue
		}
		if locAdded {
			result.LocationCount++
		}
		if itemAdded {
			result.ItemCount++
		}
	}

	return result, nil
}

// buildColIndex maps lowercase trimmed column names to their indices.
func buildColIndex(header []string) map[string]int {
	colIndex := make(map[string]int, len(header))
	for i, colName := range header {
		colIndex[strings.TrimSpace(strings.ToLower(colName))] = i
	}
	return colIndex
}

// colValue returns the trimmed value of the named column from a row, or "" if absent.
func colValue(row []string, colIndex map[string]int, name string) string {
	idx, ok := colIndex[name]
	if !ok || idx >= len(row) {
		return ""
	}
	return strings.TrimSpace(row[idx])
}

// isSkippableRow reports whether a row is blank or a comment and should not count toward line numbers.
func isSkippableRow(row []string) bool {
	if len(row) == 0 || (len(row) == 1 && strings.TrimSpace(row[0]) == "") {
		return true
	}
	return strings.HasPrefix(strings.TrimSpace(row[0]), "#")
}

// validateName checks that name is non-empty and contains no ':'.
// Returns a non-nil *InvalidEntry if validation fails.
func validateName(lineNum int, name string) *InvalidEntry {
	if name == "" {
		return &InvalidEntry{Line: lineNum, Error: "missing name"}
	}
	if strings.Contains(name, ":") {
		return &InvalidEntry{Line: lineNum, Entry: name, Error: "name must not contain ':'"}
	}
	return nil
}

// processRow handles a single validated CSV data row.
// It dispatches to addLocations or addItems based on rowType.
// Returns an InvalidEntry if the row cannot be processed, or nil on success.
// The caller is responsible for updating result counts.
func processRow(ctx context.Context, db loadDB, lineNum int, rowType, name, home string) (bool, bool, *InvalidEntry) {
	switch rowType {
	case "L":
		_, err := addLocations(ctx, db, []string{name}, home)
		if err != nil {
			return false, false, &InvalidEntry{Line: lineNum, Entry: name, Error: err.Error()}
		}
		return true, false, nil

	case "I":
		if home == "" {
			return false, false, &InvalidEntry{Line: lineNum, Entry: name, Error: "missing home location for item"}
		}
		err := addItems(ctx, db, []string{name}, home)
		if err != nil {
			return false, false, &InvalidEntry{Line: lineNum, Entry: name, Error: err.Error()}
		}
		return false, true, nil

	default:
		return false, false, &InvalidEntry{
			Line:  lineNum,
			Entry: name,
			Error: fmt.Sprintf("unknown type %q (expected L or I)", rowType),
		}
	}
}
