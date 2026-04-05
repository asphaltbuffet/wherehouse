# Task 0 Changes

## Files Modified

- `/home/grue/dev/wherehouse/internal/cli/output.go`
  - Added `Writer() io.Writer` accessor method to `OutputWriter`
  - Method returns the `out` field (the underlying `io.Writer` for standard output)
  - No other changes made

## Added Code

```go
// Writer returns the underlying io.Writer used for standard output.
// This allows callers to funnel output through the same writer that OutputWriter controls.
func (w *OutputWriter) Writer() io.Writer {
	return w.out
}
```
