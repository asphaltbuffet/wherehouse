package app

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/asphaltbuffet/wherehouse/internal/entitypath"
	"github.com/asphaltbuffet/wherehouse/internal/eventbus"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/store"
)

// BulkAddRow is one parsed row from a bulk-add CSV file.
type BulkAddRow struct {
	Path     string
	Locked   bool
	Discrete bool
	Tags     []string
}

// BulkAddOptions controls bulk-add behaviour.
type BulkAddOptions struct {
	AllowDuplicates bool
	CreateParents   bool
	ActorID         string
}

// BulkAddResult is the outcome of a BulkAddEntities call.
type BulkAddResult struct {
	Created  []EntityResult
	Skipped  []BulkSkip
	Warnings []string
}

// BulkSkip records one skipped row and the reason.
type BulkSkip struct {
	Path   string `json:"path"`
	Reason string `json:"reason"`
}

// ParseBulkCSV reads r as a CSV stream and returns the parsed rows.
// Lines beginning with '#' (no leading whitespace) are treated as comments and skipped.
// Column order: path, locked, discrete, tags (trailing columns are optional).
// When allowDuplicates is false, within-file duplicate paths are silently deduplicated (first-wins).
func ParseBulkCSV(r io.Reader, allowDuplicates bool) ([]BulkAddRow, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1 // variable columns
	cr.Comment = '#'

	var rows []BulkAddRow
	seen := make(map[string]struct{})

	for {
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse CSV: %w", err)
		}

		if len(record) == 0 || (len(record) == 1 && strings.TrimSpace(record[0]) == "") {
			continue
		}

		line, _ := cr.FieldPos(0)
		row, parseErr := parseBulkRow(record, line)
		if parseErr != nil {
			return nil, parseErr
		}

		if !allowDuplicates {
			key := canonicalPathKey(row.Path)
			if _, exists := seen[key]; exists {
				continue
			}
			seen[key] = struct{}{}
		}
		rows = append(rows, row)
	}

	return rows, nil
}

func parseBulkRow(record []string, lineNum int) (BulkAddRow, error) {
	path := strings.TrimSpace(record[0])
	if path == "" {
		return BulkAddRow{}, fmt.Errorf("line %d: path is required", lineNum)
	}

	var locked bool
	if len(record) >= 2 && strings.TrimSpace(record[1]) != "" {
		b, err := parseBool(record[1], lineNum, "locked")
		if err != nil {
			return BulkAddRow{}, err
		}
		locked = b
	}

	var discrete bool
	if len(record) >= 3 && strings.TrimSpace(record[2]) != "" {
		b, err := parseBool(record[2], lineNum, "discrete")
		if err != nil {
			return BulkAddRow{}, err
		}
		discrete = b
	}

	var tags []string
	if len(record) >= 4 && strings.TrimSpace(record[3]) != "" {
		for t := range strings.FieldsSeq(record[3]) {
			tags = append(tags, t)
		}
	}

	return BulkAddRow{
		Path:     path,
		Locked:   locked,
		Discrete: discrete,
		Tags:     tags,
	}, nil
}

func parseBool(s string, lineNum int, field string) (bool, error) {
	switch strings.TrimSpace(s) {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf(
			"line %d: invalid value for %q: %q (expected \"true\" or \"false\")",
			lineNum, field, strings.TrimSpace(s),
		)
	}
}

func canonicalPathKey(path string) string {
	p, err := entitypath.Parse(path)
	if err != nil {
		return strings.ToLower(strings.TrimSpace(path))
	}
	segs := p.Segments()
	canonical := make([]string, len(segs))
	for i, s := range segs {
		canonical[i] = inventory.CanonicalizeString(s)
	}
	return strings.Join(canonical, ":")
}

// BulkAddEntities creates entities from rows in a single transaction.
// Warnings (DB duplicate, existing parent) skip the row; hard errors abort all.
func (a *App) BulkAddEntities(ctx context.Context, rows []BulkAddRow, opts BulkAddOptions) (BulkAddResult, error) {
	var result BulkAddResult

	err := a.store.ExecInTransaction(ctx, func(tx store.Tx) error {
		for _, row := range rows {
			created, skip, warnings, err := a.bulkAddRowInTx(ctx, tx, row, opts)
			if err != nil {
				return err
			}
			result.Warnings = append(result.Warnings, warnings...)
			if skip != nil {
				result.Skipped = append(result.Skipped, *skip)
				continue
			}
			result.Created = append(result.Created, created...)
		}
		return nil
	})
	if err != nil {
		return BulkAddResult{}, err
	}

	return result, nil
}

// bulkAddRowInTx handles one CSV row within an open transaction.
// Returns (created entities, skip reason, warnings, error).
// A non-nil skip means the row was skipped without error.
// A non-nil error aborts the whole transaction.
func (a *App) bulkAddRowInTx(
	ctx context.Context,
	tx store.Tx,
	row BulkAddRow,
	opts BulkAddOptions,
) ([]EntityResult, *BulkSkip, []string, error) {
	p, err := entitypath.Parse(row.Path)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("invalid path %q: %w", row.Path, err)
	}

	segments := p.Segments()

	parentEntities, warnings, err := a.ensureAncestors(ctx, tx, segments, opts)
	if err != nil {
		return nil, nil, nil, err
	}

	// Check for DB duplicate.
	if !opts.AllowDuplicates {
		existing, resolveErr := a.resolveEntityPathTx(ctx, tx, row.Path)
		if resolveErr == nil && existing != nil {
			skip := BulkSkip{Path: row.Path, Reason: "entity already exists"}
			warnings = append(warnings, fmt.Sprintf("%q already exists, skipping", row.Path))
			return nil, &skip, warnings, nil
		}
	}

	// Create the entity itself.
	result, createErr := a.createEntityInTx(ctx, tx, CreateEntityRequest{
		DisplayName: p.Base(),
		Locked:      row.Locked,
		Discrete:    row.Discrete,
		ParentPath:  p.Dir().String(),
		ActorID:     opts.ActorID,
	})
	if createErr != nil {
		return nil, nil, nil, fmt.Errorf("create %q: %w", row.Path, createErr)
	}

	// Apply tags within the same transaction.
	for _, tag := range row.Tags {
		canonical := inventory.CanonicalizeString(tag)
		payload := eventbus.EntityTagAddedPayload{EntityID: result.EntityID, Tag: canonical}
		raw, marshalErr := json.Marshal(payload)
		if marshalErr != nil {
			return nil, nil, nil, fmt.Errorf("marshal tag payload: %w", marshalErr)
		}
		_, dispErr := a.bus.DispatchInTx(ctx, tx, inventory.EntityTagAddedEvent, opts.ActorID, raw, nil)
		if dispErr != nil {
			return nil, nil, nil, fmt.Errorf("add tag %q to %q: %w", tag, row.Path, dispErr)
		}
	}

	parentEntities = append(parentEntities, result)
	return parentEntities, nil, warnings, nil
}

// ensureAncestors walks path segments [0..len-1) and ensures each ancestor exists.
// With CreateParents=true, missing ancestors are created. Returns created ancestor
// entities and any warnings.
func (a *App) ensureAncestors(
	ctx context.Context,
	tx store.Tx,
	segments []string,
	opts BulkAddOptions,
) ([]EntityResult, []string, error) {
	var created []EntityResult
	var warnings []string

	for depth := 1; depth < len(segments); depth++ {
		ancestorPath, pathErr := entitypath.New(segments[:depth]...)
		if pathErr != nil {
			return nil, nil, fmt.Errorf("build ancestor path: %w", pathErr)
		}

		_, resolveErr := a.resolveEntityPathTx(ctx, tx, ancestorPath.String())
		if resolveErr == nil {
			// discrete enforcement is handled by createEntityInTx on the direct parent.
			if opts.CreateParents {
				warnings = append(warnings, fmt.Sprintf("parent %q already exists", ancestorPath.String()))
			}
			continue
		}

		if !opts.CreateParents {
			return nil, nil, fmt.Errorf(
				"parent %q not found (use --create-parents to create missing ancestors)",
				ancestorPath.String(),
			)
		}

		parentPath, _ := entitypath.New(segments[:depth-1]...)
		parentResult, createErr := a.createEntityInTx(ctx, tx, CreateEntityRequest{
			DisplayName: segments[depth-1],
			ParentPath:  parentPath.String(),
			ActorID:     opts.ActorID,
		})
		if createErr != nil {
			return nil, nil, fmt.Errorf("create parent %q: %w", ancestorPath.String(), createErr)
		}
		created = append(created, parentResult)
	}

	return created, warnings, nil
}
