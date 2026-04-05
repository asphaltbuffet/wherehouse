// Package list implements the wherehouse list command for displaying
// locations and their items in a tree view.
//
// The list command shows items in one or more locations using a tree-style
// display. Without arguments, it shows all root-level locations. With
// arguments, it shows only the specified locations.
//
// In non-recursive mode (default), each location shows its direct items and
// hints for direct child locations (with item and location counts). In
// recursive mode (--recurse / -r), the full subtree is displayed.
//
// Location arguments that cannot be resolved are rendered inline as
// "[arg] [not found]" and do not cause a non-zero exit code.
//
// Examples:
//
//	wherehouse list
//	wherehouse list Garage
//	wherehouse list -r Garage Office
//	wherehouse list --json Garage
package list
