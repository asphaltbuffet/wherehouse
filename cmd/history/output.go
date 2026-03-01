package history

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/goccy/go-json"
	"github.com/spf13/cobra"

	"github.com/asphaltbuffet/wherehouse/internal/database"
	"github.com/asphaltbuffet/wherehouse/internal/styles"
)

const (
	hoursPerDay         = 24
	recentDaysThreshold = 7
	uuidPrefixLength    = 8
	eventTypeMissing    = "item.missing"
)

// formatOutput formats and writes the event history to the output.
func formatOutput(
	ctx context.Context,
	cmd *cobra.Command,
	db *database.Database,
	events []*database.Event,
	jsonMode bool,
) error {
	if jsonMode {
		return formatJSON(cmd.OutOrStdout(), events)
	}

	return formatHuman(ctx, cmd.OutOrStdout(), db, events)
}

// formatJSON outputs events as JSON array.
func formatJSON(w io.Writer, events []*database.Event) error {
	output := &JSONHistoryOutput{
		Events: make([]*JSONEvent, len(events)),
		Count:  len(events),
	}

	for i, event := range events {
		output.Events[i] = convertToJSONEvent(event)
	}

	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")

	return encoder.Encode(output)
}

// JSONHistoryOutput is the top-level JSON structure.
type JSONHistoryOutput struct {
	Events []*JSONEvent `json:"events"`
	Count  int          `json:"count"`
}

// JSONEvent represents a single event in JSON output.
type JSONEvent struct {
	EventID      int64           `json:"event_id"`
	EventType    string          `json:"event_type"`
	TimestampUTC string          `json:"timestamp_utc"`
	ActorUserID  string          `json:"actor_user_id"`
	Payload      json.RawMessage `json:"payload"`
	Note         *string         `json:"note,omitempty"`
}

// convertToJSONEvent converts a database Event to JSONEvent.
func convertToJSONEvent(event *database.Event) *JSONEvent {
	return &JSONEvent{
		EventID:      event.EventID,
		EventType:    event.EventType,
		TimestampUTC: event.TimestampUTC,
		ActorUserID:  event.ActorUserID,
		Payload:      event.Payload,
		Note:         event.Note,
	}
}

// formatHuman outputs events in human-readable timeline format.
func formatHuman(ctx context.Context, w io.Writer, db *database.Database, events []*database.Event) error {
	// Build location cache for efficient lookups
	locationCache := make(map[string]string)

	for i, event := range events {
		isLast := i == len(events)-1
		if err := formatEvent(ctx, w, db, event, isLast, locationCache); err != nil {
			return err
		}
	}
	return nil
}

// formatEvent renders a single event in timeline format.
func formatEvent(
	ctx context.Context,
	w io.Writer,
	db *database.Database,
	event *database.Event,
	isLast bool,
	locationCache map[string]string,
) error {
	// Visual structure (newest first):
	//   ○  2 hours ago (alice)  item.moved
	//   │  Moved: Home/Garage/Toolbox → Home/Workshop/Bench
	//   │  Type: temporary_use
	//   │  Project: engine-rebuild
	//   │
	//   ○  2026-02-20 14:30 (bob)  item.created
	//      Created at: Home/Garage/Toolbox
	appStyles := styles.DefaultStyles()

	connector := "│"
	if event.EventType == "item.found" || event.EventType == eventTypeMissing {
		connector = "⸾"
	}
	if isLast {
		connector = " "
	}

	// Event marker
	marker := "○"
	if event.EventType == "item.deleted" {
		marker = "●" // Terminal event
	}
	if event.EventType == eventTypeMissing {
		marker = "◌"
	}

	// Parse timestamp for relative display
	timestamp := formatTimestamp(event.TimestampUTC)

	// Header line
	fmt.Fprintf(w, "%s  %s  (%s)  %s\n",
		appStyles.EventStyle(event.EventType).Render(marker),
		appStyles.EventStyle(event.EventType).Render(event.EventType),
		event.ActorUserID,
		timestamp,
	)

	// Detail lines (event-specific)
	details, err := formatEventDetails(ctx, db, event, locationCache)
	if err != nil {
		return err
	}
	for _, line := range details {
		fmt.Fprintf(w, "%s  %s\n",
			appStyles.EventStyle(event.EventType).Render(connector),
			appStyles.EventStyle(event.EventType).Render(line),
		)
	}

	// Note (if present)
	if event.Note != nil && *event.Note != "" {
		fmt.Fprintf(w, "%s  Note: %s\n",
			appStyles.EventStyle(event.EventType).Render(connector),
			appStyles.ItalicDim().Render(*event.Note),
		)
	}

	// Blank line separator
	if !isLast {
		fmt.Fprintf(w,
			"%s\n",
			appStyles.EventStyle(event.EventType).Render(connector),
		)
	}

	return nil
}

// formatTimestamp converts UTC timestamp to relative or absolute format.
func formatTimestamp(timestampUTC string) string {
	t, err := time.Parse(time.RFC3339, timestampUTC)
	if err != nil {
		return timestampUTC // Fallback to raw
	}

	now := time.Now()
	diff := now.Sub(t)

	// Relative for recent events
	if diff < recentDaysThreshold*hoursPerDay*time.Hour && diff > 0 {
		return formatRelativeTime(diff)
	}

	// Absolute for older events
	return t.Format("2006-01-02 15:04")
}

// formatRelativeTime converts duration to human-readable relative time.
func formatRelativeTime(d time.Duration) string {
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		m := int(d.Minutes())
		if m == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", m)
	case d < 24*time.Hour:
		h := int(d.Hours())
		if h == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", h)
	default:
		days := int(d.Hours() / hoursPerDay)
		if days == 1 {
			return "1 day ago"
		}
		return fmt.Sprintf("%d days ago", days)
	}
}

// formatEventDetails extracts event-specific details from payload.
func formatEventDetails(
	ctx context.Context,
	db *database.Database,
	event *database.Event,
	locationCache map[string]string,
) ([]string, error) {
	// Parse payload as generic map
	var payload map[string]any
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		// Return error so caller can decide how to handle
		return nil, fmt.Errorf("failed to parse event payload: %w", err)
	}

	switch event.EventType {
	case "item.created":
		return formatItemCreatedDetails(ctx, db, payload, locationCache), nil
	case "item.moved":
		return formatItemMovedDetails(ctx, db, payload, locationCache), nil
	case "item.borrowed":
		return formatItemBorrowedDetails(payload), nil
	case eventTypeMissing:
		return formatItemMissingDetails(ctx, db, payload, locationCache), nil
	case "item.found":
		return formatItemFoundDetails(ctx, db, payload, locationCache), nil
	case "item.deleted":
		return []string{"Item permanently deleted"}, nil
	default:
		return nil, nil
	}
}

func formatItemCreatedDetails(
	ctx context.Context,
	db *database.Database,
	payload map[string]any,
	cache map[string]string,
) []string {
	var details []string
	if locID, ok := payload["location_id"].(string); ok {
		path := resolveLocationPath(ctx, db, locID, cache)
		details = append(details, fmt.Sprintf("Created at: %s", path))
	}
	return details
}

func formatItemMovedDetails(
	ctx context.Context,
	db *database.Database,
	payload map[string]any,
	cache map[string]string,
) []string {
	var details []string
	fromID, _ := payload["from_location_id"].(string)
	toID, _ := payload["to_location_id"].(string)

	fromPath := resolveLocationPath(ctx, db, fromID, cache)
	toPath := resolveLocationPath(ctx, db, toID, cache)

	details = append(details, fmt.Sprintf("Moved: %s → %s", fromPath, toPath))

	if moveType, ok := payload["move_type"].(string); ok {
		details = append(details, fmt.Sprintf("Type: %s", moveType))
	}
	if projectID, ok := payload["project_id"].(string); ok && projectID != "" {
		details = append(details, fmt.Sprintf("Project: %s", projectID))
	}
	return details
}

func formatItemBorrowedDetails(payload map[string]any) []string {
	var details []string
	if borrowedBy, ok := payload["borrowed_by"].(string); ok {
		details = append(details, fmt.Sprintf("Borrowed by: %s", borrowedBy))
	}
	return details
}

func formatItemMissingDetails(
	ctx context.Context,
	db *database.Database,
	payload map[string]any,
	cache map[string]string,
) []string {
	var details []string
	if locID, ok := payload["previous_location_id"].(string); ok {
		path := resolveLocationPath(ctx, db, locID, cache)
		details = append(details, fmt.Sprintf("Last seen: %s", path))
	}
	return details
}

func formatItemFoundDetails(
	ctx context.Context,
	db *database.Database,
	payload map[string]any,
	cache map[string]string,
) []string {
	var details []string
	foundID, _ := payload["found_location_id"].(string)
	homeID, _ := payload["home_location_id"].(string)

	foundPath := resolveLocationPath(ctx, db, foundID, cache)
	homePath := resolveLocationPath(ctx, db, homeID, cache)

	details = append(details, fmt.Sprintf("Found at: %s", foundPath))
	details = append(details, fmt.Sprintf("Home: %s", homePath))
	return details
}

// resolveLocationPath gets a location path from cache or database, with fallback to ID.
func resolveLocationPath(
	ctx context.Context,
	db *database.Database,
	locationID string,
	cache map[string]string,
) string {
	if locationID == "" {
		return "unknown"
	}
	path, err := getLocationPath(ctx, db, locationID, cache)
	if err != nil {
		if len(locationID) >= uuidPrefixLength {
			return fmt.Sprintf("location:%s", locationID[:uuidPrefixLength])
		}
		return fmt.Sprintf("location:%s", locationID)
	}
	return path
}

// getLocationPath retrieves full hierarchical path for a location with caching.
func getLocationPath(
	ctx context.Context,
	db *database.Database,
	locationID string,
	cache map[string]string,
) (string, error) {
	// Check cache
	if path, ok := cache[locationID]; ok {
		return path, nil
	}

	// Query database
	location, err := db.GetLocation(ctx, locationID)
	if err != nil {
		return "", err
	}

	// Cache and return
	cache[locationID] = location.FullPathDisplay
	return location.FullPathDisplay, nil
}
