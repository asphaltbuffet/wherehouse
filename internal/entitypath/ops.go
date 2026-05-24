package entitypath

import (
	"errors"
	"fmt"
	"iter"
	"strings"
)

// Join appends segments to p. Each segment is validated. Passing no segments
// returns p unchanged. An empty segment string is an error.
func (p Path) Join(segments ...string) (Path, error) {
	if len(segments) == 0 {
		return p, nil
	}

	for _, seg := range segments {
		if err := ValidateSegment(seg); err != nil {
			return Path(""), fmt.Errorf("entitypath.Join: %w", errors.Join(ErrInvalidPath, err))
		}
	}

	tail := strings.Join(segments, Separator)
	if p.IsEmpty() {
		return Path(tail), nil
	}
	if p.IsRoot() {
		return Path(Separator + tail), nil
	}

	return Path(string(p) + Separator + tail), nil
}

// Append appends exactly one segment to p. It is equivalent to Join with a
// single argument and is the dominant call shape (parent path + child name).
func (p Path) Append(name string) (Path, error) {
	// validation of `name` will be done within Join()
	return p.Join(name)
}

// Clean collapses adjacent separators and trims a single trailing separator
// (except for Root). It is idempotent. It does not resolve "." or "..".
func (p Path) Clean() Path {
	s := string(p)
	if s == "" {
		return p
	}

	abs := strings.HasPrefix(s, Separator)

	// Split the whole string on Separator, drop empty segments, rejoin.
	// This handles adjacent separators and leading/trailing separators uniformly.
	parts := strings.Split(s, Separator)
	kept := parts[:0]

	for _, part := range parts {
		if part != "" {
			kept = append(kept, part)
		}
	}

	if len(kept) == 0 {
		if abs {
			return Root
		}

		return Path("")
	}

	joined := strings.Join(kept, Separator)

	if abs {
		return Path(Separator + joined)
	}

	return Path(joined)
}

// Rel returns the path of p relative to base. base must be an ancestor of p or
// equal p. If base == p, the empty relative path is returned. If base is not an
// ancestor, ErrNotDescendant is returned.
func (p Path) Rel(base Path) (Path, error) {
	if p == base {
		return Path(""), nil
	}

	if !base.IsAncestor(p) {
		return Path(""), fmt.Errorf("entitypath.Rel: %q is not an ancestor of %q: %w", base, p, ErrNotDescendant)
	}

	// base is an ancestor: strip base prefix + separator.
	// Root is special: its string representation is already the separator,
	// so stripping "Root + Separator" would strip "::::::" which is wrong.
	var prefix string

	if base.IsRoot() {
		prefix = Separator
	} else {
		prefix = string(base) + Separator
	}

	return Path(strings.TrimPrefix(string(p), prefix)), nil
}

// IsAncestor reports whether p is a strict proper ancestor of other.
// p.IsAncestor(p) is always false. Mismatched absolute/relative paths
// return false.
func (p Path) IsAncestor(other Path) bool {
	if p == other {
		return false
	}

	if p.IsAbs() != other.IsAbs() {
		return false
	}

	// Root is the ancestor of all absolute paths except itself.
	if p.IsRoot() {
		return other.IsAbs() && !other.IsRoot()
	}

	prefix := string(p) + Separator

	return strings.HasPrefix(string(other), prefix)
}

// HasPrefix reports whether p == other or other.IsAncestor(p).
// Useful for subtree containment checks.
func (p Path) HasPrefix(other Path) bool {
	return p == other || other.IsAncestor(p)
}

// Ancestors returns all ancestors of p from the root-most to the nearest,
// excluding p itself. Returns an empty slice for Root and Path("").
func (p Path) Ancestors() []Path {
	segs := p.Segments()

	if len(segs) == 0 {
		return []Path{}
	}

	abs := p.IsAbs()
	// Absolute paths include Root as the first ancestor.
	capacity := len(segs) - 1
	if abs {
		capacity = len(segs) // Root + intermediate ancestors
	}

	result := make([]Path, 0, capacity)
	if abs {
		result = append(result, Root)
	}

	for i := 1; i < len(segs); i++ {
		joined := strings.Join(segs[:i], Separator)

		if abs {
			result = append(result, Path(Separator+joined))
		} else {
			result = append(result, Path(joined))
		}
	}

	return result
}

// Walk yields ancestors of p nearest-first (closest ancestor first), excluding
// p itself. Stops if yield returns false. Yields nothing for Root and Path("").
// The range-over-func signature supports Go 1.23+ range loops.
func (p Path) Walk(yield func(ancestor Path) bool) {
	segs := p.Segments()
	if len(segs) == 0 {
		return
	}

	abs := p.IsAbs()

	// Emit intermediate ancestors nearest-first, then Root (for abs paths).
	for i := len(segs) - 1; i >= 1; i-- {
		joined := strings.Join(segs[:i], Separator)
		var ancestor Path

		if abs {
			ancestor = Path(Separator + joined)
		} else {
			ancestor = Path(joined)
		}

		if !yield(ancestor) {
			return
		}
	}

	// For absolute paths, Root is the final (farthest) ancestor.
	if abs && len(segs) >= 1 {
		yield(Root)
	}
}

// WalkSeq returns an iterator over ancestors of p nearest-first.
// Equivalent to Walk but usable directly in range loops via [iter.Seq].
func (p Path) WalkSeq() iter.Seq[Path] {
	return func(yield func(Path) bool) {
		p.Walk(yield)
	}
}
