# User Request

Add a new command `wherehouse list <location>...` that shows the items in locations.

## Requirements

- `-r`/`--recurse` flag lists all contents, sub-locations, and their items and so on
- Displays like a file-tree (tree-style output)
- Shows all locations with no parent if no location is given (including system locations like Missing/Borrowed)

## Examples

```
# List all top-level locations
wherehouse list

# List items in a specific location
wherehouse list "Garage"

# List multiple locations
wherehouse list "Garage" "Office"

# Recursively list all sub-locations and items
wherehouse list --recurse
wherehouse list -r "Garage"
```
