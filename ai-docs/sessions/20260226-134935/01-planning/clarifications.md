# User Clarifications

1. **Non-recursive display**: `wherehouse list Garage` (no --recurse) should show items AND direct child location names (as hints, without recursing into them). This is similar to how `ls` shows directories.

2. **Display order**: Within each tree node, items appear FIRST, then sub-locations below.

3. **Annotations**: Locations should show item count annotation in non-recursive mode (e.g., `Garage (3 items)`). This helps users know if a location has content without recursing.

4. **Markers**: No specific markers for borrowed/missing items requested (beyond what already exists).

5. **Depth limit**: No depth limit on --recurse (always show full tree).

6. **Sort order**: Alphabetical by display_name (already how DB functions work).
