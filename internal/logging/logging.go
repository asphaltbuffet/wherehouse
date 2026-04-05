package logging

import (
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/spf13/afero"
	"gopkg.in/natefinch/lumberjack.v2"
)

// logFileMode is the file permission for the log file.
// Owner-only read/write since logs may contain sensitive path information.
const logFileMode = 0o600

// slogAdapter wraps [slog.Logger] to implement the Logger interface.
// With() on slogAdapter returns another slogAdapter, satisfying Logger.
type slogAdapter struct {
	inner *slog.Logger
}

func (a *slogAdapter) Debug(msg string, args ...any) { a.inner.Debug(msg, args...) }
func (a *slogAdapter) Info(msg string, args ...any)  { a.inner.Info(msg, args...) }
func (a *slogAdapter) Warn(msg string, args ...any)  { a.inner.Warn(msg, args...) }
func (a *slogAdapter) Error(msg string, args ...any) { a.inner.Error(msg, args...) }
func (a *slogAdapter) With(args ...any) Logger       { return &slogAdapter{inner: a.inner.With(args...)} }

// Package-level state for the singleton logger.
var (
	once    sync.Once
	active  Logger       // nil until Init succeeds
	file    io.Closer    // held for Close()
	mu      sync.RWMutex // guards active reads/writes after init
	errInit error        // set by first Init() attempt; persisted for subsequent calls
)

// Init initializes the package-level logger using the provided parameters.
//
// fs is used for directory creation and (when maxSizeMB == 0) file opening.
// Lumberjack always uses the real OS for rotation because it owns the file handle.
//
// Only the first call takes effect (sync.Once semantics). If the first call
// fails, subsequent calls return the same error without retrying.
//
// logPath must be the resolved absolute path.
// level is a string level name (ParseLevel converts it).
// maxSizeMB == 0 disables rotation; > 0 enables lumberjack.
// maxBackups is coerced to 3 if maxSizeMB > 0 and maxBackups == 0.
func Init(fs afero.Fs, logPath, level string, maxSizeMB, maxBackups int) error {
	once.Do(func() {
		l, c, err := newLogger(fs, logPath, level, maxSizeMB, maxBackups)
		if err != nil {
			errInit = err
			return
		}
		mu.Lock()
		active = l
		file = c
		mu.Unlock()
	})
	return errInit
}

// Close flushes and closes the underlying log file or lumberjack logger.
// Safe to call multiple times; subsequent calls are no-ops.
// Should be deferred immediately after a successful Init().
func Close() error {
	mu.Lock()
	defer mu.Unlock()
	if file == nil {
		return nil
	}
	err := file.Close()
	file = nil
	return err
}

// Debug logs a message at DEBUG level.
// No-op (discarded) if Init() has not been called or failed.
func Debug(msg string, args ...any) { activeLogger().Debug(msg, args...) }

// Info logs a message at INFO level.
// No-op if Init() has not been called or failed.
func Info(msg string, args ...any) { activeLogger().Info(msg, args...) }

// Warn logs a message at WARN level.
// No-op if Init() has not been called or failed.
func Warn(msg string, args ...any) { activeLogger().Warn(msg, args...) }

// Error logs a message at ERROR level.
// No-op if Init() has not been called or failed.
func Error(msg string, args ...any) { activeLogger().Error(msg, args...) }

// With returns a Logger with the given attributes pre-attached.
// If Init() has not been called, returns a no-op Logger (writes to [io.Discard]).
//
// Use for component-scoped loggers:
//
//	dbLog := logging.With("component", "database")
func With(args ...any) Logger { return activeLogger().With(args...) }

// GetLogger returns the package-level Logger.
// Returns a no-op Logger ([io.Discard]) if Init() has not been called.
// Prefer the package-level functions for ordinary use;
// use GetLogger() when you need to pass a Logger to another component.
func GetLogger() Logger { return activeLogger() }

// ParseLevel converts a string level name to [slog.Level].
// Accepts "debug", "info", "warn", "warning", "error" (case-insensitive).
// Returns [slog.LevelWarn] for any unrecognized or empty value.
func ParseLevel(s string) slog.Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelWarn
	}
}

// newLogger is the pure, testable factory underlying Init().
// It creates the log directory via fs.MkdirAll, opens the file via fs (plain)
// or configures lumberjack (rotation), and returns a Logger and its Closer.
// No global state is touched.
func newLogger(fs afero.Fs, logPath, level string, maxSizeMB, maxBackups int) (Logger, io.Closer, error) {
	dir := filepath.Dir(logPath)

	// Use afero for directory creation - enables MemMapFs in tests.
	if err := fs.MkdirAll(dir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("logging: create log directory %q: %w", dir, err)
	}

	var (
		w      io.Writer
		closer io.Closer
	)

	if maxSizeMB > 0 {
		// Lumberjack uses real OS - afero used above to verify/create the directory.
		if maxBackups == 0 {
			maxBackups = 3
		}
		lj := &lumberjack.Logger{
			Filename:   logPath,
			MaxSize:    maxSizeMB,
			MaxBackups: maxBackups,
			Compress:   true,
		}
		w = lj
		closer = lj
	} else {
		// Use afero to open the file - fully controllable in tests.
		f, err := fs.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, logFileMode)
		if err != nil {
			return nil, nil, fmt.Errorf("logging: open log file %q: %w", logPath, err)
		}
		w = f
		closer = f
	}

	lvl := ParseLevel(level)
	handler := slog.NewTextHandler(w, &slog.HandlerOptions{Level: lvl})
	inner := slog.New(handler)
	return &slogAdapter{inner: inner}, closer, nil
}

// activeLogger returns the initialized Logger or a no-op Logger.
func activeLogger() Logger {
	mu.RLock()
	l := active
	mu.RUnlock()
	if l == nil {
		return noop()
	}
	return l
}

// noop returns a Logger that discards all output.
func noop() Logger {
	return &slogAdapter{inner: slog.New(slog.DiscardHandler)}
}
