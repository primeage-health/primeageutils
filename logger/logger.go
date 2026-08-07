// Package logger is the structured log every service writes through.
//
// The default is a real logger on stderr, and silence has to be asked for by
// name with Nop(). That is the whole design: a logger that quietly discards
// everything unless the service remembered some initialisation call is worse
// than no logger at all, because it looks like it is working. FromCtx therefore
// never returns a no-op unless one was deliberately installed.
//
// Backed by log/slog from the standard library, so logging costs the binary no
// third-party dependency.
package logger

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
)

// Logger writes structured lines. The zero value is not usable; get one from
// FromCtx or New.
type Logger struct {
	log *slog.Logger
}

type ctxKey struct{}

// domainKey names the tenant a line belongs to.
//
// This is a multi-tenant platform, so a line without its tenant cannot answer
// "did this agency's messages go out" — which is the question the delivery
// metrics exist to answer. It is a required argument rather than an optional
// field for that reason.
const domainKey = "domain"

// defaultLogger is what FromCtx falls back to. Built once, at first use.
var defaultLogger = New(os.Stderr, levelFromEnv())

// New builds a logger writing JSON to w.
func New(w io.Writer, level slog.Level) *Logger {
	return &Logger{log: slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))}
}

// Nop builds a logger that discards everything. For tests that assert on
// behaviour rather than output.
func Nop() *Logger { return New(io.Discard, slog.LevelError) }

// levelFromEnv reads LOG_LEVEL, defaulting to info.
func levelFromEnv() slog.Level {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("LOG_LEVEL"))) {
	case "debug":
		return slog.LevelDebug
	case "warn", "warning":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// FromCtx returns the logger carried by ctx, or the default one.
//
// A service that never sets a logger up still logs, which is the point.
func FromCtx(ctx context.Context) *Logger {
	if l, ok := ctx.Value(ctxKey{}).(*Logger); ok && l != nil {
		return l
	}
	return defaultLogger
}

// WithCtx returns a copy of ctx carrying l.
func WithCtx(ctx context.Context, l *Logger) context.Context {
	if current, ok := ctx.Value(ctxKey{}).(*Logger); ok && current == l {
		return ctx
	}
	return context.WithValue(ctx, ctxKey{}, l)
}

// With returns a logger that adds attrs to every line it writes.
func (l *Logger) With(attrs ...any) *Logger {
	return &Logger{log: l.log.With(attrs...)}
}

func (l *Logger) Debug(msg, domain string, attrs ...any) {
	l.log.Debug(msg, append([]any{domainKey, domain}, attrs...)...)
}

func (l *Logger) Info(msg, domain string, attrs ...any) {
	l.log.Info(msg, append([]any{domainKey, domain}, attrs...)...)
}

func (l *Logger) Warn(msg, domain string, attrs ...any) {
	l.log.Warn(msg, append([]any{domainKey, domain}, attrs...)...)
}

func (l *Logger) Error(msg, domain string, attrs ...any) {
	l.log.Error(msg, append([]any{domainKey, domain}, attrs...)...)
}
