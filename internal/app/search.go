package app

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// FindEntities searches for non-removed entities whose display name contains the query string.
// Results are sorted by Levenshtein distance (exact matches first), then alphabetically.
func (a *App) FindEntities(ctx context.Context, req FindEntitiesRequest) ([]FindResult, error) {
	return a.findMatching(ctx, req, func(EntityResult) bool { return true })
}

// FindParentCandidates searches like FindEntities but returns only eligible parents.
// Filtering happens before the limit so ineligible matches cannot crowd out eligible ones.
func (a *App) FindParentCandidates(ctx context.Context, req FindEntitiesRequest) ([]FindResult, error) {
	return a.findMatching(ctx, req, EntityResult.IsEligibleParent)
}

// Relocation names the action a destination search serves. For a displaced
// entity the two differ: a move may name the current parent (it re-homes there),
// a use may not (ADR 0039).
type Relocation string

// Relocations accepted by FindMoveDestinations.
const (
	RelocationMove Relocation = "move"
	RelocationUse  Relocation = "use"
)

// FindMoveDestinations returns eligible parents matching req.Query that
// entityID may be moved or used into: everything except the entity itself, its
// descendants, and its current parent (ADR 0034). A displaced entity also
// excludes its home (going home is a return), and for RelocationMove keeps its
// current parent, which re-homes it in place (ADR 0039). Exclusions apply
// before req.Limit so they never crowd out valid matches.
func (a *App) FindMoveDestinations(
	ctx context.Context, entityID string, relocation Relocation, req FindEntitiesRequest,
) ([]FindResult, error) {
	moving, err := a.store.GetEntity(ctx, entityID)
	if err != nil {
		return nil, wrapEntityError(entityID, err)
	}
	// Include removed entities so the walk reaches live entities orphaned
	// under a removed ancestor (#287); findMatching drops removed candidates.
	all, err := a.store.ListAllEntities(ctx)
	if err != nil {
		return nil, fmt.Errorf("find move destinations: %w", err)
	}

	excluded := subtreeIDs(all, moving.EntityID)
	rehomable := moving.IsDisplaced() && relocation == RelocationMove
	if moving.ParentID != nil && !rehomable {
		excluded[*moving.ParentID] = true
	}
	if moving.IsDisplaced() && moving.HomeID != nil {
		excluded[*moving.HomeID] = true
	}

	// One listing serves both the walk and the candidates (#315).
	live := slices.DeleteFunc(all, func(e *inventory.Entity) bool {
		return e.Status == inventory.EntityStatusRemoved
	})
	return a.matchEntities(ctx, live, req, func(r EntityResult) bool {
		return r.IsEligibleParent() && !excluded[r.EntityID]
	})
}

// subtreeIDs returns rootID plus the IDs of every entity in all beneath it.
func subtreeIDs(all []*inventory.Entity, rootID string) map[string]bool {
	children := make(map[string][]string)
	for _, e := range all {
		if e.ParentID != nil {
			children[*e.ParentID] = append(children[*e.ParentID], e.EntityID)
		}
	}
	ids := map[string]bool{rootID: true}
	queue := []string{rootID}
	for len(queue) > 0 {
		id := queue[0]
		queue = queue[1:]
		for _, c := range children[id] {
			if !ids[c] {
				ids[c] = true
				queue = append(queue, c)
			}
		}
	}
	return ids
}

func (a *App) findMatching(
	ctx context.Context, req FindEntitiesRequest, keep func(EntityResult) bool,
) ([]FindResult, error) {
	all, err := a.store.ListEntities(ctx)
	if err != nil {
		return nil, fmt.Errorf("find entities: %w", err)
	}
	return a.matchEntities(ctx, all, req, keep)
}

// matchEntities ranks the entries of all (live entities only) whose name
// contains req.Query and that keep accepts.
func (a *App) matchEntities(
	ctx context.Context, all []*inventory.Entity, req FindEntitiesRequest, keep func(EntityResult) bool,
) ([]FindResult, error) {
	ids := make([]string, len(all))
	for i, e := range all {
		ids[i] = e.EntityID
	}

	tagsByID, err := a.store.GetTagsByEntities(ctx, ids)
	if err != nil {
		return nil, fmt.Errorf("find entities tags: %w", err)
	}

	paths := pathsByID(all)
	query := strings.ToLower(req.Query)
	var results []FindResult

	for _, e := range all {
		name := strings.ToLower(e.DisplayName)
		if !strings.Contains(name, query) {
			continue
		}
		r := entityToResult(e, tagsByID[e.EntityID])
		fillHomePath(&r, paths)
		if !keep(r) {
			continue
		}
		results = append(results, FindResult{
			Entity:   r,
			Distance: levenshtein(query, name),
		})
	}

	slices.SortFunc(results, func(a, b FindResult) int {
		if a.Distance != b.Distance {
			return a.Distance - b.Distance
		}
		if a.Entity.DisplayName < b.Entity.DisplayName {
			return -1
		}
		if a.Entity.DisplayName > b.Entity.DisplayName {
			return 1
		}
		return 0
	})

	if req.Limit > 0 && len(results) > req.Limit {
		results = results[:req.Limit]
	}

	return results, nil
}

func levenshtein(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	la, lb := len(ra), len(rb)

	dp := make([][]int, la+1)
	for i := range dp {
		dp[i] = make([]int, lb+1)
		dp[i][0] = i
	}
	for j := 0; j <= lb; j++ {
		dp[0][j] = j
	}
	for i := 1; i <= la; i++ {
		for j := 1; j <= lb; j++ {
			if ra[i-1] == rb[j-1] {
				dp[i][j] = dp[i-1][j-1]
			} else {
				dp[i][j] = 1 + min(dp[i-1][j], dp[i][j-1], dp[i-1][j-1])
			}
		}
	}
	return dp[la][lb]
}
