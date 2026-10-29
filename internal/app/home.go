package app

import (
	"context"
	"fmt"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// pathsByID maps entity ID to FullPathDisplay. List-style callers already hold
// every live entity, and homes are always live (CONTEXT.md "Home"), so homes
// resolve with no extra query (ADR 0015).
func pathsByID(entities []*inventory.Entity) map[string]string {
	m := make(map[string]string, len(entities))
	for _, e := range entities {
		m[e.EntityID] = e.FullPathDisplay
	}
	return m
}

// fillHomePath sets HomePathDisplay on a displaced result with a non-root home.
func fillHomePath(r *EntityResult, paths map[string]string) {
	if r.IsDisplaced() && r.HomeID != "" {
		r.HomePathDisplay = paths[r.HomeID]
	}
}

// fillHomePaths applies fillHomePath to every result.
func fillHomePaths(results []EntityResult, paths map[string]string) {
	for i := range results {
		fillHomePath(&results[i], paths)
	}
}

// withHomePath fills HomePathDisplay for a single result (point lookups).
func (a *App) withHomePath(ctx context.Context, r EntityResult) (EntityResult, error) {
	if !r.IsDisplaced() || r.HomeID == "" {
		return r, nil
	}
	home, err := a.store.GetEntityAnyStatus(ctx, r.HomeID)
	if err != nil {
		return EntityResult{}, fmt.Errorf("get home of %q: %w", r.EntityID, err)
	}
	r.HomePathDisplay = home.FullPathDisplay
	return r, nil
}
