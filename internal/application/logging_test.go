package application

import (
	"io"
	"log/slog"
	"testing"
)

func TestDefaultLogger(t *testing.T) {
	t.Parallel()

	custom := slog.New(slog.NewTextHandler(io.Discard, nil))
	if got := defaultLogger(custom); got != custom {
		t.Fatalf("expected custom logger to be returned")
	}

	if got := defaultLogger(nil); got != slog.Default() {
		t.Fatalf("expected default logger when none provided")
	}
}
