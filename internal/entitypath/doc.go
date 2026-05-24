// Package entitypath provides filepath-like manipulation of wherehouse entity
// paths. Paths are sequences of segments joined by a separator (":"). A path
// with a leading separator is absolute (rooted at the entity tree root); one
// without is relative. The package is pure syntax — it never touches the DB.
package entitypath

// Separator is the string between path segments.
const Separator = ":"
