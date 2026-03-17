package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRunnerPoolsCreateOmitsOptionalScalingFieldsWhenFlagsUnset(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/self-hosted/pools" {
			http.NotFound(w, r)
			return
		}

		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &requestBody)
		_, _ = io.WriteString(w, `{"id":"pool-1","name":"pool-a","poolKind":"self_hosted","isActive":true}`)
	}))
	defer server.Close()

	_, _, err := executeRootCommand(t, []string{"--json", "runners", "pools", "create", "--name", "pool-a"}, map[string]string{
		"CERTYN_API_URL":  server.URL,
		"CERTYN_API_KEY":  "test-key",
		"XDG_CONFIG_HOME": t.TempDir(),
	})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if _, ok := requestBody["minRunners"]; ok {
		t.Fatalf("minRunners should be omitted when flag is unset: %#v", requestBody["minRunners"])
	}
	if _, ok := requestBody["maxRunners"]; ok {
		t.Fatalf("maxRunners should be omitted when flag is unset: %#v", requestBody["maxRunners"])
	}
	if _, ok := requestBody["slotsPerRunner"]; ok {
		t.Fatalf("slotsPerRunner should be omitted when flag is unset: %#v", requestBody["slotsPerRunner"])
	}
}

func TestRunnerPoolsCreateIncludesOptionalScalingFieldsWhenFlagsSet(t *testing.T) {
	var requestBody map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/self-hosted/pools" {
			http.NotFound(w, r)
			return
		}

		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &requestBody)
		_, _ = io.WriteString(w, `{"id":"pool-1","name":"pool-a","poolKind":"self_hosted","isActive":true}`)
	}))
	defer server.Close()

	_, _, err := executeRootCommand(t, []string{
		"--json", "runners", "pools", "create",
		"--name", "pool-a",
		"--min-runners", "2",
		"--max-runners", "5",
		"--slots-per-runner", "3",
	}, map[string]string{
		"CERTYN_API_URL":  server.URL,
		"CERTYN_API_KEY":  "test-key",
		"XDG_CONFIG_HOME": t.TempDir(),
	})
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if got := int(requestBody["minRunners"].(float64)); got != 2 {
		t.Fatalf("expected minRunners=2, got %d", got)
	}
	if got := int(requestBody["maxRunners"].(float64)); got != 5 {
		t.Fatalf("expected maxRunners=5, got %d", got)
	}
	if got := int(requestBody["slotsPerRunner"].(float64)); got != 3 {
		t.Fatalf("expected slotsPerRunner=3, got %d", got)
	}
}
