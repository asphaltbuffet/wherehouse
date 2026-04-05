package database

import (
	"context"
	"database/sql"
	"fmt"
)

// Project represents a project in the projection.
type Project struct {
	ProjectID string
	Status    string
	UpdatedAt string
}

// CreateProject creates a new project projection entry.
func (d *Database) CreateProject(ctx context.Context, projectID, status, timestamp string) error {
	const query = `
		INSERT INTO projects_current (
			project_id,
			status,
			updated_at
		) VALUES (?, ?, ?)
	`

	_, err := d.db.ExecContext(ctx, query, projectID, status, timestamp)
	if err != nil {
		return fmt.Errorf("failed to create project: %w", err)
	}

	return nil
}

// GetProject retrieves a project by its ID.
func (d *Database) GetProject(ctx context.Context, projectID string) (*Project, error) {
	const query = `
		SELECT
			project_id,
			status,
			updated_at
		FROM projects_current
		WHERE project_id = ?
	`

	var project Project
	err := d.db.QueryRowContext(ctx, query, projectID).Scan(
		&project.ProjectID,
		&project.Status,
		&project.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, ErrProjectNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get project: %w", err)
	}

	return &project, nil
}

// UpdateProject updates a project's status.
func (d *Database) UpdateProject(ctx context.Context, projectID, status, timestamp string) error {
	const query = `
		UPDATE projects_current
		SET status = ?, updated_at = ?
		WHERE project_id = ?
	`

	result, err := d.db.ExecContext(ctx, query, status, timestamp, projectID)
	if err != nil {
		return fmt.Errorf("failed to update project: %w", err)
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

// DeleteProject removes a project from the projection.
func (d *Database) DeleteProject(ctx context.Context, projectID string) error {
	const query = `DELETE FROM projects_current WHERE project_id = ?`

	result, err := d.db.ExecContext(ctx, query, projectID)
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

// GetAllProjects retrieves all projects, optionally filtered by status.
func (d *Database) GetAllProjects(ctx context.Context, status *string) ([]*Project, error) {
	var query string
	var args []any

	if status != nil {
		query = `
			SELECT project_id, status, updated_at
			FROM projects_current
			WHERE status = ?
			ORDER BY project_id
		`
		args = append(args, *status)
	} else {
		query = `
			SELECT project_id, status, updated_at
			FROM projects_current
			ORDER BY project_id
		`
	}

	rows, err := d.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query projects: %w", err)
	}
	defer rows.Close()

	var projects []*Project
	for rows.Next() {
		var project Project
		//nolint:govet // Shadow is idiomatic in loop
		err := rows.Scan(
			&project.ProjectID,
			&project.Status,
			&project.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan project: %w", err)
		}

		projects = append(projects, &project)
	}

	//nolint:govet // Shadow is idiomatic for final error check
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating projects: %w", err)
	}

	return projects, nil
}
