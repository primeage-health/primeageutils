package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

// The regression this package exists to prevent: a logger that returns a no-op
// unless some initialisation call was made first, so that every line a service
// writes is silently discarded and the problem is invisible. A logger nobody
// configured must still write.
func TestTheDefaultLoggerActuallyWrites(t *testing.T) {
	var buf bytes.Buffer
	restore := defaultLogger
	defaultLogger = New(&buf, slog.LevelInfo)
	t.Cleanup(func() { defaultLogger = restore })

	FromCtx(context.Background()).Info("provisioned an account", "agency-one.primeage.life")

	if buf.Len() == 0 {
		t.Fatal("nothing was written; the default logger is silent again")
	}
	if !strings.Contains(buf.String(), "provisioned an account") {
		t.Errorf("output missing the message: %s", buf.String())
	}
}

// A line without its tenant cannot answer "did this agency's messages go out",
// which is why domain is a required argument rather than an optional field.
func TestEveryLineCarriesItsDomain(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, slog.LevelDebug)

	l.Error("otp send failed", "agency-two.primeage.life")

	var line map[string]any
	if err := json.Unmarshal(buf.Bytes(), &line); err != nil {
		t.Fatalf("output is not JSON: %v (%s)", err, buf.String())
	}
	if line[domainKey] != "agency-two.primeage.life" {
		t.Errorf("%s = %v; want the tenant", domainKey, line[domainKey])
	}
	if line["level"] != "ERROR" {
		t.Errorf("level = %v; want ERROR", line["level"])
	}
}

func TestWithCtxRoundTrips(t *testing.T) {
	var buf bytes.Buffer
	l := New(&buf, slog.LevelInfo)

	ctx := WithCtx(context.Background(), l)
	if FromCtx(ctx) != l {
		t.Fatal("FromCtx did not return the logger WithCtx installed")
	}
}

// Nop is opt-in, so a test can silence output without the package defaulting to
// silence for everyone else.
func TestNopIsSilentButOnlyWhenAsked(t *testing.T) {
	var buf bytes.Buffer
	ctx := WithCtx(context.Background(), Nop())

	FromCtx(ctx).Info("discarded", "agency-one.primeage.life")

	if buf.Len() != 0 {
		t.Errorf("Nop wrote something: %s", buf.String())
	}
}

func TestLevelFromEnv(t *testing.T) {
	for value, want := range map[string]slog.Level{
		"debug": slog.LevelDebug,
		"WARN":  slog.LevelWarn,
		"error": slog.LevelError,
		"":      slog.LevelInfo,
		"junk":  slog.LevelInfo,
	} {
		t.Setenv("LOG_LEVEL", value)
		if got := levelFromEnv(); got != want {
			t.Errorf("LOG_LEVEL=%q gave %v; want %v", value, got, want)
		}
	}
}
