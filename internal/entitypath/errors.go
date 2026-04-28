package entitypath

import "errors"

// Sentinel errors returned by entitypath functions. Use [errors.Is] to test.
var (
	// ErrInvalidPath is returned when a path string fails structural validation.
	ErrInvalidPath = errors.New("entitypath: invalid path")
	// ErrEmptySegment is returned when a segment is an empty string.
	ErrEmptySegment = errors.New("entitypath: empty segment")
	// ErrSegmentContainsSeparator is returned when a segment contains Separator.
	ErrSegmentContainsSeparator = errors.New("entitypath: segment contains separator")
	// ErrInvalidSegment is returned when a segment has leading/trailing whitespace
	// or contains control characters.
	ErrInvalidSegment = errors.New("entitypath: invalid segment")
	// ErrNotDescendant is returned by Rel when base is not an ancestor of p.
	ErrNotDescendant = errors.New("entitypath: not a descendant of base")
)
