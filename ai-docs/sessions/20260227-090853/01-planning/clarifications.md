# User Clarifications

## Q1: Non-missing items
**Answer**: Warn and proceed
- If item is NOT at Missing, print a warning like "item is not currently missing" but still update current_location
- Same behavior for Borrowed items (warn, proceed)

## Q2: --return no-op (found location = home location)
**Answer**: Fire found, skip move, print note
- Record item as found at location (fire item.found event)
- Skip the return move since item is already at home
- Print note: "already at home location"

## Q3: Home location source for --return
**Answer**: Use temp_origin_location_id
- If temp_origin_location_id is set: use it as the return destination
- If temp_origin_location_id is NULL: skip the move entirely, print warning "unable to determine home location"

## Additional notes from user
- No project association flags on `found` (keep interface simple)
- Fail-fast for multiple items (same as `move` command)
