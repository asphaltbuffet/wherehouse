package scry

import (
	"context"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/cli"
	"github.com/asphaltbuffet/wherehouse/internal/database"
)

// labelWidth is the fixed width of the label column (right-padded with spaces).
const labelWidth = 26

const scryLongDescription = `Suggest where to look for an item currently marked as missing.

Scry analyzes event history to rank likely locations:
  1. Home location: where the item was created or originally lived
  2. Found here before: locations where the item was recovered (item.found events)
  3. Used here temporarily: locations the item was taken to temporarily
  4. Similar items: current locations of items with similar names

The item must be in the Missing system location before scrying.

Examples:
  wherehouse scry "10mm socket"        # Suggest locations for a missing item
  wherehouse scry missing:screwdriver  # Same, with explicit Missing: prefix`

// NewScryCmd returns a scry command that uses the provided db for all database
// operations. The caller retains no reference to db after this call; the
// returned command's RunE closes it via defer before returning.
func NewScryCmd(db scryDB) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scry <name>",
		Short: "Suggest locations for a missing item based on history",
		Long:  scryLongDescription,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			defer func() {
				if closeErr := db.Close(); closeErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to close database: %v\n", closeErr)
				}
			}()
			return runScryCore(cmd, args, db)
		},
	}

	registerScryFlags(cmd)
	return cmd
}

// NewDefaultScryCmd returns a scry command that opens the database from context
// configuration at runtime. This is the production entry point registered with
// the root command.
func NewDefaultScryCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "scry <name>",
		Short: "Suggest locations for a missing item based on history",
		Long:  scryLongDescription,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			db, err := cli.OpenDatabase(cmd.Context())
			if err != nil {
				return fmt.Errorf("failed to open database: %w", err)
			}
			defer func() {
				if closeErr := db.Close(); closeErr != nil {
					fmt.Fprintf(cmd.ErrOrStderr(), "warning: failed to close database: %v\n", closeErr)
				}
			}()
			return runScryCore(cmd, args, db)
		},
	}

	registerScryFlags(cmd)
	return cmd
}

// registerScryFlags attaches all scry-specific flags to cmd.
// Called by both NewScryCmd and NewDefaultScryCmd to ensure identical flag sets.
func registerScryFlags(cmd *cobra.Command) {
	cmd.Flags().BoolP("verbose", "v", false, "Show full details (IDs, match distance)")
}

// GetScryCmd returns the scry command using the default database.
//
// Deprecated: Use NewDefaultScryCmd instead.
func GetScryCmd() *cobra.Command {
	return NewDefaultScryCmd()
}

// ensure *database.Database satisfies scryDB at compile time.
var _ scryDB = (*database.Database)(nil)

// runScryCore implements the scry command logic.
func runScryCore(cmd *cobra.Command, args []string, db scryDB) error {
	ctx := cmd.Context()

	verbose, _ := cmd.Flags().GetBool("verbose")
	cfg := cli.MustGetConfig(ctx)
	out := cli.NewOutputWriterFromConfig(cmd.OutOrStdout(), cmd.ErrOrStderr(), cfg)

	itemID, err := cli.ResolveItemSelector(ctx, db, args[0], "wherehouse scry")
	if err != nil {
		return err
	}

	item, err := db.GetItem(ctx, itemID)
	if err != nil {
		return fmt.Errorf("failed to get item: %w", err)
	}

	if err = validateItemIsMissing(ctx, db, item); err != nil {
		return err
	}

	result, err := db.ScryItem(ctx, item)
	if err != nil {
		return fmt.Errorf("scry failed: %w", err)
	}

	if result.HomeLocation == nil &&
		len(result.FoundLocations) == 0 &&
		len(result.TempUseLocations) == 0 &&
		len(result.SimilarItemLocations) == 0 {
		out.Println(fmt.Sprintf("No suggestions found for %q", item.DisplayName))
		return nil
	}

	if cfg.IsJSON() {
		return outputJSON(out, result)
	}

	outputHuman(out.Writer(), result, verbose)

	return nil
}

// validateItemIsMissing checks that the item is in the Missing system location.
// Returns specific errors for Borrowed and Loaned items.
func validateItemIsMissing(ctx context.Context, db scryDB, item *database.Item) error {
	missingID, borrowedID, loanedID, _, err := db.GetSystemLocationIDs(ctx)
	if err != nil {
		return fmt.Errorf("failed to get system locations: %w", err)
	}

	switch item.LocationID {
	case missingID:
		return nil // OK
	case borrowedID:
		return fmt.Errorf("item %q is borrowed, not missing", item.DisplayName)
	case loanedID:
		return fmt.Errorf("item %q is loaned out, not missing", item.DisplayName)
	default:
		loc, locErr := db.GetLocation(ctx, item.LocationID)
		locName := item.LocationID
		if locErr == nil && loc != nil {
			locName = loc.FullPathDisplay
		}

		return fmt.Errorf("item %q is not missing (currently at: %s)", item.DisplayName, locName)
	}
}

// outputHuman formats scry results in human-readable format.
func outputHuman(w io.Writer, result *database.ScryResult, verbose bool) {
	fmt.Fprintf(w, "Scrying for: %s (MISSING)\n\n", result.DisplayName)

	// Category 1: Home location (always shown, guaranteed non-nil)
	if result.HomeLocation != nil {
		printLabeledRow(w, "Home location:", result.HomeLocation.FullPathDisplay, "")
	}

	// Category 2: Previously found locations
	printScoredCategory(w, "Found here before:", result.FoundLocations, verbose)

	// Category 3: Temporary use locations
	printScoredCategory(w, "Used here temporarily:", result.TempUseLocations, verbose)

	// Category 4: Similar item locations
	printSimilarItemCategory(w, "Where similar items are:", result.SimilarItemLocations, verbose)
}

// printScoredCategory prints a category of scored locations with occurrence counts.
func printScoredCategory(w io.Writer, label string, locations []*database.ScoredLocation, verbose bool) {
	for i, sl := range locations {
		suffix := ""
		if verbose {
			if sl.Occurrences == 1 {
				suffix = "  (1 time)"
			} else {
				suffix = fmt.Sprintf("  (%d times)", sl.Occurrences)
			}
		}

		if i == 0 {
			printLabeledRow(w, label, sl.Location.FullPathDisplay, suffix)
		} else {
			printContinuationRow(w, sl.Location.FullPathDisplay, suffix)
		}
	}
}

// printSimilarItemCategory prints the similar-item category, annotating each entry with the similar item name.
func printSimilarItemCategory(w io.Writer, label string, locations []*database.ScoredLocation, verbose bool) {
	for i, sl := range locations {
		// Use SimilarItemDisplayName when available; fall back to SimilarItemName (canonical).
		displayName := sl.SimilarItemDisplayName
		if displayName == "" {
			displayName = sl.SimilarItemName
		}

		var itemAnnotation string
		if verbose {
			itemAnnotation = fmt.Sprintf("  [%s, dist=%d]", displayName, sl.LevenshteinDistance)
		} else {
			itemAnnotation = fmt.Sprintf("  [%s]", displayName)
		}

		if i == 0 {
			printLabeledRow(w, label, sl.Location.FullPathDisplay, itemAnnotation)
		} else {
			printContinuationRow(w, sl.Location.FullPathDisplay, itemAnnotation)
		}
	}
}

// printLabeledRow prints a row with a label in the first column and a value in the second.
// suffix is appended directly after the value (no extra spacing).
func printLabeledRow(w io.Writer, label, value, suffix string) {
	fmt.Fprintf(w, "  %-*s%s%s\n", labelWidth, label, value, suffix)
}

// printContinuationRow prints a continuation row (no label, indented to value column).
func printContinuationRow(w io.Writer, value, suffix string) {
	// 2 leading spaces + labelWidth spaces = indent to value column
	fmt.Fprintf(w, "  %-*s%s%s\n", labelWidth, "", value, suffix)
}

// JSON output structures.

type jsonScryOutput struct {
	ItemID               string            `json:"item_id"`
	DisplayName          string            `json:"display_name"`
	CanonicalName        string            `json:"canonical_name"`
	HomeLocation         *jsonScryLocation `json:"home_location,omitempty"`
	FoundLocations       []*jsonScoredLoc  `json:"found_locations"`
	TempUseLocations     []*jsonScoredLoc  `json:"temp_use_locations"`
	SimilarItemLocations []*jsonSimilarLoc `json:"similar_item_locations"`
}

type jsonScryLocation struct {
	LocationID  string `json:"location_id"`
	DisplayName string `json:"display_name"`
	FullPath    string `json:"full_path"`
}

type jsonScoredLoc struct {
	LocationID  string `json:"location_id"`
	DisplayName string `json:"display_name"`
	FullPath    string `json:"full_path"`
	Occurrences int    `json:"occurrences"`
}

type jsonSimilarLoc struct {
	LocationID             string `json:"location_id"`
	DisplayName            string `json:"display_name"`
	FullPath               string `json:"full_path"`
	SimilarItem            string `json:"similar_item"`
	SimilarItemDisplayName string `json:"similar_item_display_name"`
	LevenshteinDistance    int    `json:"levenshtein_distance"`
}

// outputJSON formats scry results as JSON.
func outputJSON(out *cli.OutputWriter, result *database.ScryResult) error {
	output := jsonScryOutput{
		ItemID:               result.ItemID,
		DisplayName:          result.DisplayName,
		CanonicalName:        result.CanonicalName,
		FoundLocations:       make([]*jsonScoredLoc, 0, len(result.FoundLocations)),
		TempUseLocations:     make([]*jsonScoredLoc, 0, len(result.TempUseLocations)),
		SimilarItemLocations: make([]*jsonSimilarLoc, 0, len(result.SimilarItemLocations)),
		HomeLocation:         nil,
	}

	if result.HomeLocation != nil {
		output.HomeLocation = &jsonScryLocation{
			LocationID:  result.HomeLocation.LocationID,
			DisplayName: result.HomeLocation.DisplayName,
			FullPath:    result.HomeLocation.FullPathDisplay,
		}
	}

	for _, sl := range result.FoundLocations {
		output.FoundLocations = append(output.FoundLocations, &jsonScoredLoc{
			LocationID:  sl.Location.LocationID,
			DisplayName: sl.Location.DisplayName,
			FullPath:    sl.Location.FullPathDisplay,
			Occurrences: sl.Occurrences,
		})
	}

	for _, sl := range result.TempUseLocations {
		output.TempUseLocations = append(output.TempUseLocations, &jsonScoredLoc{
			LocationID:  sl.Location.LocationID,
			DisplayName: sl.Location.DisplayName,
			FullPath:    sl.Location.FullPathDisplay,
			Occurrences: sl.Occurrences,
		})
	}

	for _, sl := range result.SimilarItemLocations {
		// Use SimilarItemDisplayName when available; fall back to SimilarItemName (canonical).
		displayName := sl.SimilarItemDisplayName
		if displayName == "" {
			displayName = sl.SimilarItemName
		}

		output.SimilarItemLocations = append(output.SimilarItemLocations, &jsonSimilarLoc{
			LocationID:             sl.Location.LocationID,
			DisplayName:            sl.Location.DisplayName,
			FullPath:               sl.Location.FullPathDisplay,
			SimilarItem:            sl.SimilarItemName,
			SimilarItemDisplayName: displayName,
			LevenshteinDistance:    sl.LevenshteinDistance,
		})
	}

	return out.JSON(output)
}
