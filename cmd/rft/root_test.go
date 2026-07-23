package main

import (
	"log/slog"
	"testing"
)

func TestConfigureLoggingVerbose(t *testing.T) {
	configureLogging(false)
	if slog.Default().Enabled(t.Context(), slog.LevelDebug) {
		t.Fatal("without verbose, debug must be disabled")
	}
	if !slog.Default().Enabled(t.Context(), slog.LevelInfo) {
		t.Fatal("without verbose, info must be enabled")
	}

	configureLogging(true)
	if !slog.Default().Enabled(t.Context(), slog.LevelDebug) {
		t.Fatal("with verbose, debug must be enabled")
	}
}
