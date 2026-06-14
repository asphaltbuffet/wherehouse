package tui

import (
	"strings"

	"charm.land/bubbles/v2/viewport"

	"github.com/asphaltbuffet/wherehouse/internal/app"
	"github.com/asphaltbuffet/wherehouse/internal/inventory"
	"github.com/asphaltbuffet/wherehouse/internal/styles"
)

const (
	treeIndent    = "  "
	treeItemArrow = "▶ "
	treeItemLeaf  = "  "
)

// treeNode represents one entity in the tree, whether visible or not.
type treeNode struct {
	result      app.EntityResult // full result for detail pane rendering
	hasChildren bool
	loaded      bool // true once GetChildren has been called for this node
	expanded    bool
	depth       int
	parentID    string // "" for root nodes
}

func treeNodeFromResult(r app.EntityResult, depth int, parentID string) treeNode {
	return treeNode{
		result:      r,
		hasChildren: r.HasChildren,
		depth:       depth,
		parentID:    parentID,
	}
}

// treeModel is a viewport-backed tree widget.
type treeModel struct {
	vp      viewport.Model
	st      *styles.Styles
	cursor  int   // index into the visible slice
	visible []int // indices into Model.nodes that are currently shown
}

func newTreeModel(st *styles.Styles) treeModel {
	vp := viewport.New()
	return treeModel{vp: vp, st: st}
}

func (t treeModel) SetSize(w, h int) treeModel {
	t.vp.SetWidth(w)
	t.vp.SetHeight(h)
	return t
}

// render rebuilds the viewport content from the visible slice of nodes.
func (t treeModel) render(nodes []treeNode) treeModel {
	if len(t.visible) == 0 {
		t.vp.SetContent("")
		return t
	}
	lines := make([]string, len(t.visible))
	for i, ni := range t.visible {
		n := nodes[ni]
		lines[i] = t.renderLine(n, i == t.cursor)
	}
	t.vp.SetContent(strings.Join(lines, "\n"))
	return t
}

func (t treeModel) renderLine(n treeNode, selected bool) string {
	indent := strings.Repeat(treeIndent, n.depth)

	var arrow string
	if n.hasChildren {
		if n.expanded {
			arrow = "▼ "
		} else {
			arrow = "▶ "
		}
	} else {
		arrow = treeItemLeaf
	}

	label := n.result.DisplayName
	if n.result.Locked {
		label += " 🔒"
	}

	statusSuffix := ""
	if n.result.Status != inventory.EntityStatusOk {
		statusSuffix = " [" + n.result.Status.String() + "]"
	}

	line := indent + arrow + label + statusSuffix
	if selected {
		return t.st.TUISelected().Render(line)
	}
	return line
}

// rebuildVisible rebuilds the visible index slice from nodes, respecting expanded state.
// Call this after any structural change (expand, collapse, splice, delete).
func rebuildVisible(nodes []treeNode) []int {
	visible := make([]int, 0, len(nodes))
	expandedIDs := make(map[string]bool)
	for _, n := range nodes {
		if n.expanded {
			expandedIDs[n.result.EntityID] = true
		}
	}
	for i, n := range nodes {
		if n.depth == 0 {
			visible = append(visible, i)
			continue
		}
		if allAncestorsExpanded(nodes, n, expandedIDs) {
			visible = append(visible, i)
		}
	}
	return visible
}

// allAncestorsExpanded returns true if every ancestor of n is expanded.
func allAncestorsExpanded(nodes []treeNode, n treeNode, expandedIDs map[string]bool) bool {
	pid := n.parentID
	for pid != "" {
		if !expandedIDs[pid] {
			return false
		}
		found := false
		for _, p := range nodes {
			if p.result.EntityID == pid {
				pid = p.parentID
				found = true
				break
			}
		}
		if !found {
			break
		}
	}
	return true
}

// clampCursor keeps cursor in [0, len(visible)-1].
func clampCursor(cursor, visibleLen int) int {
	if visibleLen == 0 {
		return 0
	}
	if cursor < 0 {
		return 0
	}
	if cursor >= visibleLen {
		return visibleLen - 1
	}
	return cursor
}

// scrollToCursor adjusts the viewport so the cursor line is visible.
func (t treeModel) scrollToCursor() treeModel {
	if t.cursor < t.vp.YOffset() {
		t.vp.SetYOffset(t.cursor)
	} else if t.cursor >= t.vp.YOffset()+t.vp.Height() {
		t.vp.SetYOffset(t.cursor - t.vp.Height() + 1)
	}
	return t
}

// spliceChildren inserts children of parentID immediately after the parent in nodes.
// Existing children of that parent (already loaded) are removed first.
func spliceChildren(nodes []treeNode, parentID string, children []treeNode) []treeNode {
	nodes = removeSubtree(nodes, parentID)
	parentIdx := -1
	for i, n := range nodes {
		if n.result.EntityID == parentID {
			parentIdx = i
			break
		}
	}
	if parentIdx < 0 {
		return nodes
	}
	result := make([]treeNode, 0, len(nodes)+len(children))
	result = append(result, nodes[:parentIdx+1]...)
	result = append(result, children...)
	result = append(result, nodes[parentIdx+1:]...)
	return result
}

// removeSubtree removes all descendants of parentID from nodes.
func removeSubtree(nodes []treeNode, parentID string) []treeNode {
	descendants := make(map[string]bool)
	descendants[parentID] = true
	changed := true
	for changed {
		changed = false
		for _, n := range nodes {
			if !descendants[n.result.EntityID] && descendants[n.parentID] {
				descendants[n.result.EntityID] = true
				changed = true
			}
		}
	}
	delete(descendants, parentID)
	result := make([]treeNode, 0, len(nodes))
	for _, n := range nodes {
		if !descendants[n.result.EntityID] {
			result = append(result, n)
		}
	}
	return result
}

// findNodeIndex returns the first index in nodes where entityID matches, or -1.
func findNodeIndex(nodes []treeNode, entityID string) int {
	for i, n := range nodes {
		if n.result.EntityID == entityID {
			return i
		}
	}
	return -1
}

// findNodeIndexByPath returns the first index in nodes where fullPath matches, or -1.
func findNodeIndexByPath(nodes []treeNode, fullPath string) int {
	for i, n := range nodes {
		if n.result.FullPathDisplay == fullPath {
			return i
		}
	}
	return -1
}

// setCursorToEntity moves the cursor to the visible row for entityID.
// Returns the new cursor position (unchanged if not found).
func setCursorToEntity(visible []int, nodes []treeNode, entityID string, current int) int {
	for i, ni := range visible {
		if nodes[ni].result.EntityID == entityID {
			return i
		}
	}
	return current
}

// View returns the rendered viewport string.
func (t treeModel) View() string {
	return t.vp.View()
}
