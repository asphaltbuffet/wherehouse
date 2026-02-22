package database

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/goccy/go-json"
)

// --- Project Event Handlers ---

func (d *Database) handleProjectCreated(ctx context.Context, tx *sql.Tx, event *Event) error {
	var payload struct {
		ProjectID string `json:"project_id"`
	}

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	const query = `
		INSERT INTO projects_current (project_id, status, updated_at)
		VALUES (?, 'active', ?)
	`

	_, err := tx.ExecContext(ctx, query, payload.ProjectID, event.TimestampUTC)
	if err != nil {
		return fmt.Errorf("failed to insert project: %w", err)
	}

	return nil
}

func (d *Database) handleProjectCompleted(ctx context.Context, tx *sql.Tx, event *Event) error {
	var payload struct {
		ProjectID string `json:"project_id"`
	}

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	const query = `
		UPDATE projects_current
		SET status = 'completed', updated_at = ?
		WHERE project_id = ?
	`

	_, err := tx.ExecContext(ctx, query, event.TimestampUTC, payload.ProjectID)
	if err != nil {
		return fmt.Errorf("failed to complete project: %w", err)
	}

	return nil
}

func (d *Database) handleProjectReopened(ctx context.Context, tx *sql.Tx, event *Event) error {
	var payload struct {
		ProjectID string `json:"project_id"`
	}

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	const query = `
		UPDATE projects_current
		SET status = 'active', updated_at = ?
		WHERE project_id = ?
	`

	_, err := tx.ExecContext(ctx, query, event.TimestampUTC, payload.ProjectID)
	if err != nil {
		return fmt.Errorf("failed to reopen project: %w", err)
	}

	return nil
}

func (d *Database) handleProjectDeleted(ctx context.Context, tx *sql.Tx, event *Event) error {
	var payload struct {
		ProjectID string `json:"project_id"`
	}

	if err := json.Unmarshal(event.Payload, &payload); err != nil {
		return fmt.Errorf("failed to unmarshal payload: %w", err)
	}

	const query = `DELETE FROM projects_current WHERE project_id = ?`

	result, err := tx.ExecContext(ctx, query, payload.ProjectID)
	if err != nil {
		return fmt.Errorf("failed to delete project: %w", err)
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrProjectNotFound
	}

	return nil
}
