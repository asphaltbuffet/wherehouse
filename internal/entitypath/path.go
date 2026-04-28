package entitypath

import (
	"encoding/json"
	"errors"
	"fmt"
	"slices"
	"strings"
	"unicode"
)

// Path is a syntactic entity path. The zero value Path("") is the valid empty
// relative path. Paths with a leading Separator are absolute (rooted).
type Path string //nolint:recvcheck // UnmarshalJSON requires *Path to satisfy json.Unmarshaler; all other methods use value receivers by design

// Root is the canonical absolute root path: a leading separator with no segments.
var Root = Path(Separator)

// MustParse panics if s is not a valid path. Intended for test helpers and
// package-level vars only.
func MustParse(s string) Path {
	p, err := Parse(s)
	if err != nil {
		panic(err)
	}

	return p
}

// String implements [fmt.Stringer].
func (p Path) String() string { return string(p) }

// MarshalJSON emits the path as a JSON string.
func (p Path) MarshalJSON() ([]byte, error) {
	return json.Marshal(string(p))
}

// UnmarshalJSON accepts a JSON string and validates it via Parse.
// The pointer receiver is required to satisfy [json.Unmarshaler].
func (p *Path) UnmarshalJSON(b []byte) error {
	var s string

	if err := json.Unmarshal(b, &s); err != nil {
		return err
	}

	parsed, err := Parse(s)
	if err != nil {
		return err
	}

	*p = parsed

	return nil
}

// ValidateSegment returns an error if name is not a valid path segment.
// It rejects empty strings, strings containing Separator, strings with
// leading/trailing whitespace, and strings containing control characters.
func ValidateSegment(name string) error {
	if name == "" {
		return ErrEmptySegment
	}

	if strings.ContainsRune(name, ':') {
		return ErrSegmentContainsSeparator
	}

	// Reject leading/trailing whitespace and control characters (Unicode Cc).
	runes := []rune(name)

	if unicode.IsSpace(runes[0]) || unicode.IsSpace(runes[len(runes)-1]) {
		return ErrInvalidSegment
	}

	if slices.ContainsFunc(runes, unicode.IsControl) {
		return ErrInvalidSegment
	}

	return nil
}

// Parse validates s and returns it as a Path. It does not Clean; invalid paths
// return ErrInvalidPath (wrapping a more specific inner error).
func Parse(s string) (Path, error) {
	if s == "" || s == Separator {
		return Path(s), nil
	}

	// Strip leading separator for abs paths before splitting.
	body := s
	if strings.HasPrefix(s, Separator) {
		body = s[len(Separator):]
	}

	segs := strings.SplitSeq(body, Separator)
	for seg := range segs {
		if err := ValidateSegment(seg); err != nil {
			return Path(""), fmt.Errorf("entitypath.Parse %q: %w", s, errors.Join(ErrInvalidPath, err))
		}
	}

	return Path(s), nil
}

// New builds a relative path from segments. Each segment is validated.
// New() with no arguments returns Path("").
func New(segments ...string) (Path, error) {
	if len(segments) == 0 {
		return Path(""), nil
	}

	for _, seg := range segments {
		if err := ValidateSegment(seg); err != nil {
			return Path(""), err
		}
	}

	return Path(strings.Join(segments, Separator)), nil
}

// IsAbs reports whether p is an absolute (rooted) path.
func (p Path) IsAbs() bool {
	return strings.HasPrefix(string(p), Separator)
}

// IsRoot reports whether p is the absolute root path (Separator with no segments).
func (p Path) IsRoot() bool {
	return p == Root
}

// IsEmpty reports whether p is the empty relative path (zero value).
func (p Path) IsEmpty() bool {
	return p == ""
}

// Depth returns the number of segments. Root and the empty path both have depth 0.
func (p Path) Depth() int {
	return len(p.Segments())
}

// Segments returns the path segments. Never returns nil; empty paths and Root
// return an empty slice.
func (p Path) Segments() []string {
	s := string(p)
	if s == "" || s == Separator {
		return []string{}
	}

	body := s
	if strings.HasPrefix(s, Separator) {
		body = s[len(Separator):]
	}

	return strings.Split(body, Separator)
}

// Base returns the last segment, or "" for Root and the empty path.
func (p Path) Base() string {
	segs := p.Segments()
	if len(segs) == 0 {
		return ""
	}

	return segs[len(segs)-1]
}

// Dir returns all but the last segment of p. Dir of a 1-segment absolute path
// returns Root; Dir of a 1-segment relative path returns Path(""). Dir of Root
// returns Root; Dir of Path("") returns Path("").
func (p Path) Dir() Path {
	segs := p.Segments()
	if len(segs) == 0 {
		return p
	}

	if len(segs) == 1 {
		if p.IsAbs() {
			return Root
		}
		return Path("")
	}

	parent := strings.Join(segs[:len(segs)-1], Separator)

	if p.IsAbs() {
		return Path(Separator + parent)
	}

	return Path(parent)
}
