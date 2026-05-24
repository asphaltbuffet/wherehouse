package app

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"github.com/asphaltbuffet/wherehouse/internal/inventory"
)

// FindEntities searches for entities whose display name contains the query string.
// Results are sorted by Levenshtein distance (exact matches first), then alphabetically.
// Removed entities are excluded.
func (a *App) FindEntities(ctx context.Context, req FindEntitiesRequest) ([]FindResult, error) {
	all, err := a.store.ListEntities(ctx)
	if err != nil {
		return nil, fmt.Errorf("find entities: %w", err)
	}

	query := strings.ToLower(req.Query)
	var results []FindResult

	for _, e := range all {
		if e.Status == inventory.EntityStatusRemoved {
			continue
		}
		name := strings.ToLower(e.DisplayName)
		if !strings.Contains(name, query) {
			continue
		}
		results = append(results, FindResult{
			Entity:   entityToResult(e),
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
