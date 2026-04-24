package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/goccy/go-json"
)

// handleEntityCreated processes an EntityCreatedEvent and inserts the entity
// into the entities_current projection.
func (d *Database) handleEntityCreated(ctx context.Context, tx *sql.Tx, event *Event) error {
	var payload struct {
		EntityID    string  `json:"entity_id"`
		DisplayName string  `json:"display_name"`
		EntityType  string  `json:"entity_type"`
		ParentID    *string `json:"parent_id"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("handleEntityCreated: unmarshal payload: %w", err)
	}

	entityType, err := ParseEntityType(payload.EntityType)
	if err != nil {
		return fmt.Errorf("handleEntityCreated: %w", err)
	}

	// Place entities can only be nested inside other place entities.
	if entityType == EntityTypePlace && payload.ParentID != nil {
		if validateErr := validatePlaceParentTx(ctx, tx, *payload.ParentID); validateErr != nil {
			return fmt.Errorf("handleEntityCreated: %w", validateErr)
		}
	}

	canonicalName := CanonicalizeString(payload.DisplayName)
	fullPathDisplay, fullPathCanonical, depth, err := d.ComputeEntityPathTx(
		ctx,
		tx,
		payload.DisplayName,
		canonicalName,
		payload.ParentID,
	)
	if err != nil {
		return fmt.Errorf("handleEntityCreated: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	const query = `
		INSERT INTO entities_current (
			entity_id,
			display_name,
			canonical_name,
			entity_type,
			parent_id,
			full_path_display,
			full_path_canonical,
			depth,
			status,
			status_context,
			last_event_id,
			updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, NULL, ?, ?)
	`
	_, err = tx.ExecContext(ctx, query,
		payload.EntityID,
		payload.DisplayName,
		canonicalName,
		entityType.String(),
		payload.ParentID,
		fullPathDisplay,
		fullPathCanonical,
		depth,
		EntityStatusOk.String(),
		event.EventID,
		now,
	)
	if err != nil {
		return fmt.Errorf("handleEntityCreated: insert entity %s: %w", payload.EntityID, err)
	}

	return nil
}

// handleEntityRenamed processes an EntityRenamedEvent and updates the entity's
// name fields plus propagates path changes to all descendants.
func (d *Database) handleEntityRenamed(ctx context.Context, tx *sql.Tx, event *Event) error {
	var payload struct {
		EntityID    string `json:"entity_id"`
		DisplayName string `json:"display_name"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("handleEntityRenamed: unmarshal payload: %w", err)
	}

	// Fetch current parent_id.
	var parentID sql.NullString
	err := tx.QueryRowContext(ctx,
		`SELECT parent_id FROM entities_current WHERE entity_id = ?`,
		payload.EntityID,
	).Scan(&parentID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("handleEntityRenamed: entity %q not found: %w", payload.EntityID, ErrEntityNotFound)
		}
		return fmt.Errorf("handleEntityRenamed: query entity %s: %w", payload.EntityID, err)
	}

	var parentIDPtr *string
	if parentID.Valid {
		parentIDPtr = &parentID.String
	}

	newCanonical := CanonicalizeString(payload.DisplayName)
	fullPathDisplay, fullPathCanonical, depth, err := d.ComputeEntityPathTx(
		ctx,
		tx,
		payload.DisplayName,
		newCanonical,
		parentIDPtr,
	)
	if err != nil {
		return fmt.Errorf("handleEntityRenamed: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	_, err = tx.ExecContext(ctx, `
		UPDATE entities_current
		SET display_name = ?,
		    canonical_name = ?,
		    full_path_display = ?,
		    full_path_canonical = ?,
		    depth = ?,
		    last_event_id = ?,
		    updated_at = ?
		WHERE entity_id = ?`,
		payload.DisplayName,
		newCanonical,
		fullPathDisplay,
		fullPathCanonical,
		depth,
		event.EventID,
		now,
		payload.EntityID,
	)
	if err != nil {
		return fmt.Errorf("handleEntityRenamed: update entity %s: %w", payload.EntityID, err)
	}

	return propagatePathChanges(ctx, tx, payload.EntityID, event)
}

// handleEntityReparented processes an EntityReparentedEvent. Place entities
// cannot be reparented — this is enforced here as defense-in-depth.
func (d *Database) handleEntityReparented(ctx context.Context, tx *sql.Tx, event *Event) error {
	var payload struct {
		EntityID string  `json:"entity_id"`
		ParentID *string `json:"parent_id"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("handleEntityReparented: unmarshal payload: %w", err)
	}

	// Fetch current entity_type, display_name, canonical_name.
	var entityTypeStr, displayName, canonicalName string
	err := tx.QueryRowContext(ctx,
		`SELECT entity_type, display_name, canonical_name FROM entities_current WHERE entity_id = ?`,
		payload.EntityID,
	).Scan(&entityTypeStr, &displayName, &canonicalName)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("handleEntityReparented: entity %q not found: %w", payload.EntityID, ErrEntityNotFound)
		}
		return fmt.Errorf("handleEntityReparented: query entity %s: %w", payload.EntityID, err)
	}

	entityType, err := ParseEntityType(entityTypeStr)
	if err != nil {
		return fmt.Errorf("handleEntityReparented: %w", err)
	}

	// Place entities cannot be moved.
	if entityType == EntityTypePlace {
		return errors.New("handleEntityReparented: place entities cannot be reparented")
	}

	// Non-place entities can be moved anywhere (nil parent → top-level is fine too).
	// If moving under a parent, no place-type restriction applies for non-places.

	fullPathDisplay, fullPathCanonical, depth, err := d.ComputeEntityPathTx(
		ctx,
		tx,
		displayName,
		canonicalName,
		payload.ParentID,
	)
	if err != nil {
		return fmt.Errorf("handleEntityReparented: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	_, err = tx.ExecContext(ctx, `
		UPDATE entities_current
		SET parent_id = ?,
		    full_path_display = ?,
		    full_path_canonical = ?,
		    depth = ?,
		    last_event_id = ?,
		    updated_at = ?
		WHERE entity_id = ?`,
		payload.ParentID,
		fullPathDisplay,
		fullPathCanonical,
		depth,
		event.EventID,
		now,
		payload.EntityID,
	)
	if err != nil {
		return fmt.Errorf("handleEntityReparented: update entity %s: %w", payload.EntityID, err)
	}

	return propagatePathChanges(ctx, tx, payload.EntityID, event)
}

// handleEntityPathChanged processes an EntityPathChangedEvent (derived event)
// and updates the path fields of the entity.
func (d *Database) handleEntityPathChanged(ctx context.Context, tx *sql.Tx, event *Event) error {
	var payload struct {
		EntityID          string `json:"entity_id"`
		FullPathDisplay   string `json:"full_path_display"`
		FullPathCanonical string `json:"full_path_canonical"`
		Depth             int    `json:"depth"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("handleEntityPathChanged: unmarshal payload: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	_, err := tx.ExecContext(ctx, `
		UPDATE entities_current
		SET full_path_display = ?,
		    full_path_canonical = ?,
		    depth = ?,
		    last_event_id = ?,
		    updated_at = ?
		WHERE entity_id = ?`,
		payload.FullPathDisplay,
		payload.FullPathCanonical,
		payload.Depth,
		event.EventID,
		now,
		payload.EntityID,
	)
	if err != nil {
		return fmt.Errorf("handleEntityPathChanged: update entity %s: %w", payload.EntityID, err)
	}

	return nil
}

// handleEntityStatusChanged processes an EntityStatusChangedEvent and updates
// the status and status_context fields.
func (d *Database) handleEntityStatusChanged(ctx context.Context, tx *sql.Tx, event *Event) error {
	var payload struct {
		EntityID      string  `json:"entity_id"`
		Status        string  `json:"status"`
		StatusContext *string `json:"status_context"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("handleEntityStatusChanged: unmarshal payload: %w", err)
	}

	status, err := ParseEntityStatus(payload.Status)
	if err != nil {
		return fmt.Errorf("handleEntityStatusChanged: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	_, err = tx.ExecContext(ctx, `
		UPDATE entities_current
		SET status = ?,
		    status_context = ?,
		    last_event_id = ?,
		    updated_at = ?
		WHERE entity_id = ?`,
		status.String(),
		payload.StatusContext,
		event.EventID,
		now,
		payload.EntityID,
	)
	if err != nil {
		return fmt.Errorf("handleEntityStatusChanged: update entity %s: %w", payload.EntityID, err)
	}

	return nil
}

// handleEntityRemoved processes an EntityRemovedEvent. It refuses to remove
// an entity that still has non-removed children.
func (d *Database) handleEntityRemoved(ctx context.Context, tx *sql.Tx, event *Event) error {
	var payload struct {
		EntityID string `json:"entity_id"`
	}
	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("handleEntityRemoved: unmarshal payload: %w", err)
	}

	var childCount int
	err := tx.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM entities_current WHERE parent_id = ? AND status != ?`,
		payload.EntityID,
		EntityStatusRemoved.String(),
	).Scan(&childCount)
	if err != nil {
		return fmt.Errorf("handleEntityRemoved: count children of %s: %w", payload.EntityID, err)
	}

	if childCount > 0 {
		return fmt.Errorf(
			"entity %q has %d non-removed children; remove or relocate them first",
			payload.EntityID, childCount,
		)
	}

	now := time.Now().UTC().Format(time.RFC3339)

	_, err = tx.ExecContext(ctx, `
		UPDATE entities_current
		SET status = ?,
		    status_context = NULL,
		    last_event_id = ?,
		    updated_at = ?
		WHERE entity_id = ?`,
		EntityStatusRemoved.String(),
		event.EventID,
		now,
		payload.EntityID,
	)
	if err != nil {
		return fmt.Errorf("handleEntityRemoved: update entity %s: %w", payload.EntityID, err)
	}

	return nil
}

// validatePlaceParentTx checks that the entity identified by parentID exists
// and is itself a place entity.
func validatePlaceParentTx(ctx context.Context, tx *sql.Tx, parentID string) error {
	var entityTypeStr string
	err := tx.QueryRowContext(ctx,
		`SELECT entity_type FROM entities_current WHERE entity_id = ?`,
		parentID,
	).Scan(&entityTypeStr)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("parent entity %q not found", parentID)
		}
		return fmt.Errorf("validatePlaceParentTx: query parent %s: %w", parentID, err)
	}

	parentType, err := ParseEntityType(entityTypeStr)
	if err != nil {
		return fmt.Errorf("validatePlaceParentTx: %w", err)
	}

	if parentType != EntityTypePlace {
		return fmt.Errorf("a place entity can only be nested inside another place, not %q", entityTypeStr)
	}

	return nil
}

// descendantRow holds the fields fetched from the recursive CTE in propagatePathChanges.
type descendantRow struct {
	entityID      string
	displayName   string
	canonicalName string
	parentID      string
	depth         int
}

// propagatePathChanges updates the full_path_display, full_path_canonical, and
// depth of all descendants of entityID (ordered shallowest-first so ancestors
// are processed before children). A derived entity.path_changed event is
// appended for each descendant.
func propagatePathChanges(ctx context.Context, tx *sql.Tx, entityID string, triggerEvent *Event) error {
	descendants, err := queryDescendants(ctx, tx, entityID)
	if err != nil {
		return err
	}

	now := time.Now().UTC().Format(time.RFC3339)

	for _, desc := range descendants {
		if applyErr := applyDescendantPathUpdate(ctx, tx, desc, triggerEvent, now); applyErr != nil {
			return applyErr
		}
	}

	return nil
}

// queryDescendants fetches all descendants of entityID ordered by depth ascending.
func queryDescendants(ctx context.Context, tx *sql.Tx, entityID string) ([]descendantRow, error) {
	const query = `
		WITH RECURSIVE descendants AS (
			SELECT
				entity_id,
				display_name,
				canonical_name,
				parent_id,
				depth
			FROM entities_current
			WHERE parent_id = ?
			UNION ALL
			SELECT
				e.entity_id,
				e.display_name,
				e.canonical_name,
				e.parent_id,
				e.depth
			FROM entities_current e
			INNER JOIN descendants d ON e.parent_id = d.entity_id
		)
		SELECT entity_id, display_name, canonical_name, parent_id, depth
		FROM descendants
		ORDER BY depth ASC, entity_id ASC
	`

	rows, err := tx.QueryContext(ctx, query, entityID)
	if err != nil {
		return nil, fmt.Errorf("queryDescendants: query descendants of %s: %w", entityID, err)
	}
	defer rows.Close()

	var descendants []descendantRow
	for rows.Next() {
		var d descendantRow
		if scanErr := rows.Scan(&d.entityID, &d.displayName, &d.canonicalName, &d.parentID, &d.depth); scanErr != nil {
			return nil, fmt.Errorf("queryDescendants: scan: %w", scanErr)
		}
		descendants = append(descendants, d)
	}
	if rowsErr := rows.Err(); rowsErr != nil {
		return nil, fmt.Errorf("queryDescendants: iterate: %w", rowsErr)
	}

	return descendants, nil
}

// applyDescendantPathUpdate recomputes and stores the path for a single descendant,
// then appends a derived EntityPathChangedEvent.
func applyDescendantPathUpdate(
	ctx context.Context,
	tx *sql.Tx,
	desc descendantRow,
	triggerEvent *Event,
	now string,
) error {
	// Read the parent's already-updated path.
	var parentPathDisplay, parentPathCanonical string
	var parentDepth int
	err := tx.QueryRowContext(ctx,
		`SELECT full_path_display, full_path_canonical, depth FROM entities_current WHERE entity_id = ?`,
		desc.parentID,
	).Scan(&parentPathDisplay, &parentPathCanonical, &parentDepth)
	if err != nil {
		return fmt.Errorf("applyDescendantPathUpdate: query parent %s: %w", desc.parentID, err)
	}

	newPathDisplay := parentPathDisplay + "::" + desc.displayName
	newPathCanonical := parentPathCanonical + "::" + desc.canonicalName
	newDepth := parentDepth + 1

	_, err = tx.ExecContext(ctx, `
		UPDATE entities_current
		SET full_path_display = ?,
		    full_path_canonical = ?,
		    depth = ?,
		    last_event_id = ?,
		    updated_at = ?
		WHERE entity_id = ?`,
		newPathDisplay,
		newPathCanonical,
		newDepth,
		triggerEvent.EventID,
		now,
		desc.entityID,
	)
	if err != nil {
		return fmt.Errorf("applyDescendantPathUpdate: update %s: %w", desc.entityID, err)
	}

	derivedPayload := map[string]any{
		"entity_id":           desc.entityID,
		"full_path_display":   newPathDisplay,
		"full_path_canonical": newPathCanonical,
		"depth":               newDepth,
	}
	if appendErr := appendDerivedEventInTx(
		ctx,
		tx,
		EntityPathChangedEvent,
		triggerEvent.ActorUserID,
		derivedPayload,
		nil,
	); appendErr != nil {
		return fmt.Errorf("applyDescendantPathUpdate: append derived event for %s: %w", desc.entityID, appendErr)
	}

	return nil
}

// appendDerivedEventInTx inserts a derived event directly into the events table
// without invoking processEventInTx (the projection is already updated by the caller).
func appendDerivedEventInTx(
	ctx context.Context,
	tx *sql.Tx,
	eventType EventType,
	actorUserID string,
	payload any,
	note *string,
) error {
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("appendDerivedEventInTx: marshal payload: %w", err)
	}

	// Extract entity_id for the events.entity_id column.
	var payloadMap map[string]any
	if unmarshalErr := json.Unmarshal(payloadJSON, &payloadMap); unmarshalErr != nil {
		return fmt.Errorf("appendDerivedEventInTx: unmarshal payload map: %w", unmarshalErr)
	}

	var entityID *string
	if id, ok := payloadMap["entity_id"].(string); ok && id != "" {
		entityID = &id
	}

	timestamp := time.Now().UTC().Format(time.RFC3339)

	const query = `
		INSERT INTO events (
			event_type,
			timestamp_utc,
			actor_user_id,
			payload,
			note,
			entity_id
		) VALUES (?, ?, ?, ?, ?, ?)
	`
	_, err = tx.ExecContext(ctx, query,
		eventType,
		timestamp,
		actorUserID,
		string(payloadJSON),
		note,
		entityID,
	)
	if err != nil {
		return fmt.Errorf("appendDerivedEventInTx: insert event: %w", err)
	}

	return nil
}
