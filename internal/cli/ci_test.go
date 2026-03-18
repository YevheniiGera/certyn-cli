package cli

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/certyn/certyn-cli/internal/api"
	"github.com/certyn/certyn-cli/internal/output"
)

func TestResolveProcessSlug(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{input: "smoke", want: "smoke-suite"},
		{input: "regression", want: "regression-suite"},
		{input: "explore", want: "app-explorer"},
		{input: "custom-suite", want: "custom-suite"},
	}

	for _, tt := range tests {
		if got := resolveProcessSlug(tt.input); got != tt.want {
			t.Fatalf("resolveProcessSlug(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestGenerateIDUsesUUIDv4Format(t *testing.T) {
	got := generateID()
	uuidV4Pattern := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	if !uuidV4Pattern.MatchString(got) {
		t.Fatalf("generateID() = %q, expected UUID v4 format", got)
	}
}

func TestEmitStatusOutputsIncludesStatusURL(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "github_output.txt")
	t.Setenv("GITHUB_OUTPUT", outputPath)

	emitStatusOutputs(api.CiRunStatusResponse{
		RunID:      "run-123",
		State:      "completed",
		Conclusion: "passed",
		Total:      10,
		Passed:     10,
	}, "https://api.certyn.io/api/ci/runs/run-123")

	raw, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed reading output file: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "run_id=run-123") {
		t.Fatalf("missing run_id in output: %s", text)
	}
	if !strings.Contains(text, "status_url=https://api.certyn.io/api/ci/runs/run-123") {
		t.Fatalf("missing status_url in output: %s", text)
	}
}

func TestCIWaitJSONSuccessEmitsStatusPayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/ci/runs/run-success" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, `{
			"runId":"run-success",
			"state":"completed",
			"conclusion":"success",
			"total":2,
			"passed":2,
			"failed":0,
			"blocked":0,
			"pending":0,
			"aborted":0,
			"retryAfterSeconds":15
		}`)
	}))
	defer server.Close()

	stdout, _, err := executeRootCommand(t, []string{"--json", "run", "wait", "run-success", "--timeout", "2s"}, map[string]string{
		"CERTYN_API_URL":     server.URL,
		"CERTYN_API_KEY":     "test-key",
		"XDG_CONFIG_HOME":    t.TempDir(),
		"CERTYN_PROFILE":     "",
		"CERTYN_PROJECT":     "",
		"CERTYN_ENVIRONMENT": "",
	})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	var payload api.CiRunStatusResponse
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, stdout)
	}
	if payload.RunID != "run-success" {
		t.Fatalf("unexpected run id: %q", payload.RunID)
	}
	if payload.Conclusion != "success" {
		t.Fatalf("unexpected conclusion: %q", payload.Conclusion)
	}
}

func TestCIWaitJSONTimeoutEmitsFailurePayload(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/ci/runs/run-timeout" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, `{
			"runId":"run-timeout",
			"state":"running",
			"conclusion":"",
			"total":0,
			"passed":0,
			"failed":0,
			"blocked":0,
			"pending":0,
			"aborted":0,
			"retryAfterSeconds":60
		}`)
	}))
	defer server.Close()

	stdout, _, err := executeRootCommand(t, []string{"--json", "run", "wait", "run-timeout", "--timeout", "50ms"}, map[string]string{
		"CERTYN_API_URL":  server.URL,
		"CERTYN_API_KEY":  "test-key",
		"XDG_CONFIG_HOME": t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected timeout error")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitTimeout {
		t.Fatalf("expected timeout exit code %d, got %d", ExitTimeout, cmdErr.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, stdout)
	}
	if payload["run_id"] != "run-timeout" {
		t.Fatalf("expected run_id run-timeout, got %#v", payload["run_id"])
	}
	if int(payload["exit_code"].(float64)) != ExitTimeout {
		t.Fatalf("expected exit_code %d, got %#v", ExitTimeout, payload["exit_code"])
	}
	errorText, _ := payload["error"].(string)
	if !strings.Contains(errorText, "timed out waiting for run completion") {
		t.Fatalf("unexpected error field: %q", errorText)
	}
}

func TestCIListSlugFallbackIgnoresAuthErrorAfterEmptyResult(t *testing.T) {
	var overviewRequested atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/projects/my-slug/ci-runs":
			_, _ = io.WriteString(w, `{"items":[],"totalCount":0}`)
			return
		case "/api/projects/overview":
			overviewRequested.Store(true)
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"error":"forbidden"}`)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer server.Close()

	stdout, _, err := executeRootCommand(t, []string{"--json", "run", "list", "--project", "my-slug"}, map[string]string{
		"CERTYN_API_URL":  server.URL,
		"CERTYN_API_KEY":  "test-key",
		"XDG_CONFIG_HOME": t.TempDir(),
	})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}
	if !overviewRequested.Load() {
		t.Fatal("expected fallback project resolution to be attempted")
	}

	var payload api.ListCiRunsResponse
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, stdout)
	}
	if payload.TotalCount != 0 {
		t.Fatalf("expected empty list, got totalCount=%d", payload.TotalCount)
	}
}

func TestEmitCIWaitJSONFailurePayload(t *testing.T) {
	stdout := captureStdout(t, func() {
		err := emitCIWaitJSON(output.Printer{JSON: true}, "run-interrupted", nil, ExitInterrupted, &CommandError{
			Code:    ExitInterrupted,
			Message: "interrupted",
		})
		if err != nil {
			t.Fatalf("emitCIWaitJSON returned error: %v", err)
		}
	})

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, stdout)
	}
	if payload["run_id"] != "run-interrupted" {
		t.Fatalf("unexpected run_id: %#v", payload["run_id"])
	}
	if int(payload["exit_code"].(float64)) != ExitInterrupted {
		t.Fatalf("expected exit_code %d, got %#v", ExitInterrupted, payload["exit_code"])
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	oldStdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = writer

	done := make(chan string, 1)
	go func() {
		raw, _ := io.ReadAll(reader)
		done <- string(raw)
	}()

	fn()

	_ = writer.Close()
	os.Stdout = oldStdout
	return <-done
}
