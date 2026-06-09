package listcmd

import (
	"fmt"
	"strings"

	libtree "charm.land/lipgloss/v2/tree"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/styles"
)

// buildTree renders all entities as a lipgloss tree. matched is the filtered
// subset; any entity not in matched is shown only if it is an ancestor of a
// matched entity, and is rendered dimmed. verbose adds EntityID and tags.
func buildTree(all, matched []app.EntityResult, verbose bool) string {
	if len(all) == 0 {
		return ""
	}

	s := styles.DefaultStyles()
	matchedIDs := idSet(matched)
	shown := shownEntities(all, matchedIDs)

	// Pre-compute which shown entity IDs have at least one shown child,
	// so we push a subtree node only when children will actually be added.
	shownChildOf := make(map[string]bool, len(shown))
	for _, e := range all {
		if !shown[e.EntityID] {
			continue
		}
		prefix := e.FullPathDisplay + ":"
		for _, candidate := range all {
			if shown[candidate.EntityID] && strings.HasPrefix(candidate.FullPathDisplay, prefix) {
				shownChildOf[e.EntityID] = true
				break
			}
		}
	}

	root := libtree.New()
	stack := []*libtree.Tree{root}

	for _, e := range all {
		if !shown[e.EntityID] {
			continue
		}

		depth := strings.Count(e.FullPathDisplay, ":")
		for len(stack) > depth+1 {
			stack = stack[:len(stack)-1]
		}

		label := formatLabel(e, matchedIDs[e.EntityID], verbose, s)

		if shownChildOf[e.EntityID] {
			sub := libtree.Root(label)
			stack[len(stack)-1].Child(sub)
			stack = append(stack, sub)
		} else {
			stack[len(stack)-1].Child(label)
		}
	}

	out := root.String()
	if out == "" {
		return ""
	}
	return out + "\n"
}

func idSet(entities []app.EntityResult) map[string]bool {
	ids := make(map[string]bool, len(entities))
	for _, e := range entities {
		ids[e.EntityID] = true
	}
	return ids
}

// shownEntities returns the set of entity IDs that should appear in the tree:
// every matched entity plus any entity that is an ancestor of a matched entity.
func shownEntities(all []app.EntityResult, matchedIDs map[string]bool) map[string]bool {
	// Build the set of matched full paths once — O(m).
	matchedPaths := make(map[string]bool, len(matchedIDs))
	for _, e := range all {
		if matchedIDs[e.EntityID] {
			matchedPaths[e.FullPathDisplay] = true
		}
	}

	shown := make(map[string]bool, len(all))
	for _, e := range all {
		if matchedIDs[e.EntityID] {
			shown[e.EntityID] = true
			continue
		}
		// Show this entity if any matched entity is a descendant of it.
		prefix := e.FullPathDisplay + ":"
		for p := range matchedPaths {
			if strings.HasPrefix(p, prefix) {
				shown[e.EntityID] = true
				break
			}
		}
	}
	return shown
}

func formatLabel(e app.EntityResult, isMatch, verbose bool, s *styles.Styles) string {
	if !isMatch {
		return s.TextDim().Render(e.DisplayName)
	}

	var b strings.Builder

	if verbose {
		b.WriteString(s.Muted().Render(e.EntityID))
		b.WriteString("  ")
	}

	if e.Discrete {
		b.WriteString(s.AccentText().Render(e.DisplayName))
	} else {
		b.WriteString(s.AccentBold().Render(e.DisplayName))
	}

	if e.Locked {
		b.WriteString(" 🔒")
	}

	if verbose {
		for _, tag := range e.Tags {
			b.WriteString("  ")
			b.WriteString(s.SecondaryText().Render("#" + tag))
		}
	}

	if e.Status != inventory.EntityStatusOk {
		badge := fmt.Sprintf("[%s]", strings.ToUpper(e.Status.String()))
		b.WriteString("  ")
		b.WriteString(s.LocationStyle(e.Status.String()).Render(badge))
	}

	return b.String()
}
