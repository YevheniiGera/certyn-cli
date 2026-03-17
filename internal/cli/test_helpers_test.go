package cli

import (
	"bytes"
	"io"
	"os"
	"testing"
)

func executeRootCommand(t *testing.T, args []string, env map[string]string) (string, string, error) {
	t.Helper()

	for key, value := range env {
		t.Setenv(key, value)
	}

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	stdoutReader, stdoutWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	stderrReader, stderrWriter, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stderr pipe: %v", err)
	}

	os.Stdout = stdoutWriter
	os.Stderr = stderrWriter

	outCh := make(chan string, 1)
	errCh := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stdoutReader)
		outCh <- buf.String()
	}()
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, stderrReader)
		errCh <- buf.String()
	}()

	root := NewRootCommand()
	root.SetArgs(args)
	runErr := root.Execute()

	_ = stdoutWriter.Close()
	_ = stderrWriter.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	stdout := <-outCh
	stderr := <-errCh

	return stdout, stderr, runErr
}
