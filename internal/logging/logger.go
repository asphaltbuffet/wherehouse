package logging

//go:generate go run github.com/vektra/mockery/v2@latest

// Logger is the interface for structured, leveled logging.
// All methods match the [slog.Logger] method signatures to allow
// drop-in substitution. With returns a Logger (not [slog.Logger])
// to keep the interface self-contained and mockable.
type Logger interface {
	// Debug logs a message at DEBUG level.
	Debug(msg string, args ...any)

	// Info logs a message at INFO level.
	Info(msg string, args ...any)

	// Warn logs a message at WARN level.
	Warn(msg string, args ...any)

	// Error logs a message at ERROR level.
	Error(msg string, args ...any)

	// With returns a new Logger with the given key-value attributes
	// pre-attached to every subsequent log record.
	With(args ...any) Logger
}

// Ensure slogAdapter satisfies Logger at compile time.
var _ Logger = (*slogAdapter)(nil)
