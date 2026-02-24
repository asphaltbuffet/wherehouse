package find

import (
	"context"
	"fmt"
	"io"

	"github.com/goccy/go-json"
	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

var findCmd *cobra.Command

// GetFindCmd returns the find command, initializing it if necessary.
func GetFindCmd() *cobra.Command {
	if findCmd != nil {
		return findCmd
	}

	findCmd = &cobra.Command{
		Use:   "find <name>",
		Short: "Find items or locations by name",
		Long: `Search for items or locations matching the given name.

Returns all items with names containing the search term, showing their
current locations. Also returns locations with matching names.

Results are ranked by similarity (exact matches first).

Special indicators:
  (MISSING)  - Item is marked as missing (shows last known location)
  (BORROWED) - Item is currently borrowed (shows last known location)

Examples:
  wherehouse find screwdriver          # Find all screwdrivers
  wherehouse find toolbox              # Find toolbox location
  wherehouse find socket -n 5          # Limit to 5 closest matches
  wherehouse find "10mm" -v            # Verbose output with IDs`,
		Args: cobra.ExactArgs(1),
		RunE: runFind,
	}

	findCmd.Flags().IntP("limit", "n", 0, "Limit number of results (0 = unlimited)")
	findCmd.Flags().BoolP("verbose", "v", false, "Show full details (IDs, match distance)")
	findCmd.Flags().Bool("json", false, "Output as JSON")

	return findCmd
}

// runFind implements the find command logic.
func runFind(cmd *cobra.Command, args []string) error {
	ctx := cmd.Context()

	// Get search term
	searchTerm := args[0]

	// Get flags
	limit, _ := cmd.Flags().GetInt("limit")
	verbose, _ := cmd.Flags().GetBool("verbose")
	jsonMode, _ := cmd.Flags().GetBool("json")

	// Get database connection
	db, err := openDatabase(ctx)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	// Execute search
	results, err := db.SearchByName(ctx, searchTerm, limit)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	// Handle no results
	if len(results) == 0 {
		return fmt.Errorf("no matches found for %q", searchTerm)
	}

	// Format and output results
	if jsonMode {
		return outputJSON(cmd.OutOrStdout(), results, searchTerm)
	}

	outputHuman(cmd.OutOrStdout(), results, verbose)

	return nil
}

// outputHuman formats results in human-readable format.
func outputHuman(w io.Writer, results []*database.SearchResult, verbose bool) {
	for _, r := range results {
		switch r.Type {
		case "item":
			outputItemHuman(w, r, verbose)
		case "location":
			outputLocationHuman(w, r, verbose)
		}

		fmt.Fprintf(w, "\n")
	}
}

// outputItemHuman formats a single item result in human-readable format.
func outputItemHuman(w io.Writer, r *database.SearchResult, verbose bool) {
	fmt.Fprintf(w, "%s", r.DisplayName)

	// Determine item status and format accordingly
	switch {
	case r.IsMissing:
		fmt.Fprintf(w, " (MISSING)\n")
		if r.LastNonSystemLocation != nil {
			fmt.Fprintf(w, "  Last location: %s\n", r.LastNonSystemLocation.FullPathDisplay)
		}
		fmt.Fprintf(w, "  Currently: Missing\n")
	case r.IsBorrowed:
		fmt.Fprintf(w, " (BORROWED)\n")
		if r.LastNonSystemLocation != nil {
			fmt.Fprintf(w, "  Last location: %s\n", r.LastNonSystemLocation.FullPathDisplay)
		}
		fmt.Fprintf(w, "  Currently: Borrowed\n")
	default:
		fmt.Fprintf(w, "\n")
		if r.CurrentLocation != nil {
			fmt.Fprintf(w, "  Location: %s\n", r.CurrentLocation.FullPathDisplay)
		}
	}

	if verbose {
		outputItemVerbose(w, r)
	}
}

// outputItemVerbose formats verbose item details.
func outputItemVerbose(w io.Writer, r *database.SearchResult) {
	if r.ItemID != nil {
		fmt.Fprintf(w, "  ID: %s\n", *r.ItemID)
	}

	fmt.Fprintf(w, "  Match distance: %d", r.LevenshteinDistance)

	if r.LevenshteinDistance == 0 {
		fmt.Fprintf(w, " (exact match)")
	}

	fmt.Fprintf(w, "\n")
}

// outputLocationHuman formats a single location result in human-readable format.
func outputLocationHuman(w io.Writer, r *database.SearchResult, verbose bool) {
	fmt.Fprintf(w, "%s (Location)\n", r.DisplayName)
	fmt.Fprintf(w, "  Path: %s\n", r.FullPath)

	if verbose {
		if r.LocationID != nil {
			fmt.Fprintf(w, "  ID: %s\n", *r.LocationID)
		}
		fmt.Fprintf(w, "  Match distance: %d\n", r.LevenshteinDistance)
	}
}

// JSON output structures.
type jsonOutput struct {
	SearchTerm    string        `json:"search_term"`
	Results       []*jsonResult `json:"results"`
	TotalCount    int           `json:"total_count"`
	ItemCount     int           `json:"item_count"`
	LocationCount int           `json:"location_count"`
}

type jsonResult struct {
	Type                  string            `json:"type"`
	ItemID                *string           `json:"item_id,omitempty"`
	LocationID            *string           `json:"location_id,omitempty"`
	DisplayName           string            `json:"display_name"`
	CanonicalName         string            `json:"canonical_name"`
	Location              *jsonLocationInfo `json:"location,omitempty"`
	FullPath              string            `json:"full_path,omitempty"`
	InTemporaryUse        bool              `json:"in_temporary_use,omitempty"`
	IsMissing             bool              `json:"is_missing,omitempty"`
	IsBorrowed            bool              `json:"is_borrowed,omitempty"`
	IsSystem              bool              `json:"is_system,omitempty"`
	LastNonSystemLocation *jsonLocationInfo `json:"last_non_system_location,omitempty"`
	LevenshteinDistance   int               `json:"levenshtein_distance"`
}

type jsonLocationInfo struct {
	LocationID  string `json:"location_id"`
	DisplayName string `json:"display_name"`
	FullPath    string `json:"full_path"`
}

// outputJSON formats results as JSON.
func outputJSON(w io.Writer, results []*database.SearchResult, searchTerm string) error {
	output := jsonOutput{
		SearchTerm: searchTerm,
		Results:    make([]*jsonResult, len(results)),
	}

	for i, r := range results {
		jr := &jsonResult{
			Type:                r.Type,
			DisplayName:         r.DisplayName,
			CanonicalName:       r.CanonicalName,
			LevenshteinDistance: r.LevenshteinDistance,
		}

		if r.Type == "item" {
			jr.ItemID = r.ItemID
			jr.InTemporaryUse = r.InTemporaryUse
			jr.IsMissing = r.IsMissing
			jr.IsBorrowed = r.IsBorrowed

			if r.CurrentLocation != nil {
				jr.Location = &jsonLocationInfo{
					LocationID:  r.CurrentLocation.LocationID,
					DisplayName: r.CurrentLocation.DisplayName,
					FullPath:    r.CurrentLocation.FullPathDisplay,
				}
			}

			if r.LastNonSystemLocation != nil {
				jr.LastNonSystemLocation = &jsonLocationInfo{
					LocationID:  r.LastNonSystemLocation.LocationID,
					DisplayName: r.LastNonSystemLocation.DisplayName,
					FullPath:    r.LastNonSystemLocation.FullPathDisplay,
				}
			}

			output.ItemCount++
		} else {
			jr.LocationID = r.LocationID
			jr.FullPath = r.FullPath
			jr.IsSystem = r.IsSystem
			output.LocationCount++
		}

		output.Results[i] = jr
	}

	output.TotalCount = len(results)

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(output)
}

// openDatabase opens the database connection using config settings.
func openDatabase(ctx context.Context) (*database.Database, error) {
	return cli.OpenDatabase(ctx)
}
