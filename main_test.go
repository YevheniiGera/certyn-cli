package main

import (
	"fmt"
	"testing"

	"github.com/certyn/certyn-cli/internal/cli"
)

func TestFindCommandErrorHandlesWrappedErrors(t *testing.T) {
	expected := &cli.CommandError{
		Code:    cli.ExitTimeout,
		Message: "timed out waiting for run completion",
	}
	wrapped := fmt.Errorf("wrapped: %w", expected)

	found := findCommandError(wrapped)
	if found == nil {
		t.Fatal("expected wrapped CommandError to be detected")
	}
	if found.Code != cli.ExitTimeout {
		t.Fatalf("expected exit code %d, got %d", cli.ExitTimeout, found.Code)
	}
	if found.Message != expected.Message {
		t.Fatalf("expected message %q, got %q", expected.Message, found.Message)
	}
}
