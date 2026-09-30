// Package logger is the structured log every service writes through.
//
// The default is a real logger on stderr. A logger that quietly discards
// everything unless the service remembered some initialisation call is worse
// than no logger at all, because it looks like it is working.
//
// Backed by log/slog, so logging costs the binary no third-party dependency.
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

// domainKey names the tenant a line belongs to.
//
// This is a multi-tenant platform, so a line without its tenant cannot answer
// "did this agency's messages go out" — which is the question the delivery
// metrics exist to answer. It is a required argument rather than an optional
// field for that reason.
const domainKey = "domain"

var defaultLogger = New(os.Stderr, levelFromEnv())

// New builds a logger writing JSON to w.
func New(w io.Writer, level slog.Level) *Logger {
	return &Logger{log: slog.New(slog.NewJSONHandler(w, &slog.HandlerOptions{Level: level}))}
}

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

// FromCtx returns the default logger. The context is kept in the signature so
// its callers need no change should a per-request logger ever return.
func FromCtx(context.Context) *Logger {
	return defaultLogger
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
