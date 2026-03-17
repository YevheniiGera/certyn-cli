package cli

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

const mappedProjectID = "11111111-2222-4333-8444-555555555555"

func TestTriageCommandAliasesRegistered(t *testing.T) {
	root := NewRootCommand()

	issuesCmd, _, err := root.Find([]string{"tickets"})
	if err != nil {
		t.Fatalf("expected tickets alias to resolve: %v", err)
	}
	if issuesCmd.Name() != "issues" {
		t.Fatalf("expected tickets alias to resolve to issues command, got %q", issuesCmd.Name())
	}

	envCmd, _, err := root.Find([]string{"environments"})
	if err != nil {
		t.Fatalf("expected environments alias to resolve: %v", err)
	}
	if envCmd.Name() != "env" {
		t.Fatalf("expected environments alias to resolve to env command, got %q", envCmd.Name())
	}
}

func TestTestcasesListMapsQueryParameters(t *testing.T) {
	var captured url.Values
	server := newTriageTestServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+mappedProjectID+"/testcases" {
			captured = r.URL.Query()
			_, _ = io.WriteString(w, `{"items":[],"totalCount":0,"page":2,"pageSize":30,"totalPages":0,"hasNextPage":false,"hasPreviousPage":false}`)
			return true
		}
		return false
	})
	defer server.Close()

	_, _, err := executeRootCommand(t, []string{
		"--json", "testcases", "list",
		"--project", "my-project",
		"--tag", "smoke",
		"--quarantined", "true",
		"--page", "2",
		"--page-size", "30",
	}, triageBaseEnv(t, server.URL))
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if got := captured.Get("tag"); got != "smoke" {
		t.Fatalf("expected tag=smoke, got %q", got)
	}
	if got := captured.Get("isQuarantined"); got != "true" {
		t.Fatalf("expected isQuarantined=true, got %q", got)
	}
	if got := captured.Get("page"); got != "2" {
		t.Fatalf("expected page=2, got %q", got)
	}
	if got := captured.Get("pageSize"); got != "30" {
		t.Fatalf("expected pageSize=30, got %q", got)
	}
}

func TestIssuesListMapsQueryParameters(t *testing.T) {
	var captured url.Values
	server := newTriageTestServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+mappedProjectID+"/tickets" {
			captured = r.URL.Query()
			_, _ = io.WriteString(w, `{"items":[],"totalCount":0,"page":1,"pageSize":20,"totalPages":0,"hasNextPage":false,"hasPreviousPage":false}`)
			return true
		}
		return false
	})
	defer server.Close()

	_, _, err := executeRootCommand(t, []string{
		"--json", "issues", "list",
		"--project", "my-project",
		"--activity", "active",
		"--type", "bug",
		"--severity", "major",
		"--status", "open",
		"--agent-id", "agent-1",
		"--environment", "production",
		"--environment-version", "2026.02.22",
		"--page", "3",
		"--page-size", "40",
	}, triageBaseEnv(t, server.URL))
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if got := captured.Get("activity"); got != "active" {
		t.Fatalf("expected activity=active, got %q", got)
	}
	if got := captured.Get("type"); got != "bug" {
		t.Fatalf("expected type=bug, got %q", got)
	}
	if got := captured.Get("severity"); got != "major" {
		t.Fatalf("expected severity=major, got %q", got)
	}
	if got := captured.Get("status"); got != "open" {
		t.Fatalf("expected status=open, got %q", got)
	}
	if got := captured.Get("agentId"); got != "agent-1" {
		t.Fatalf("expected agentId=agent-1, got %q", got)
	}
	if got := captured.Get("environmentKey"); got != "production" {
		t.Fatalf("expected environmentKey=production, got %q", got)
	}
	if got := captured.Get("environmentVersion"); got != "2026.02.22" {
		t.Fatalf("expected environmentVersion=2026.02.22, got %q", got)
	}
	if got := captured.Get("page"); got != "3" {
		t.Fatalf("expected page=3, got %q", got)
	}
	if got := captured.Get("pageSize"); got != "40" {
		t.Fatalf("expected pageSize=40, got %q", got)
	}
}

func TestExecutionsListMapsQueryParameters(t *testing.T) {
	var captured url.Values
	server := newTriageTestServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+mappedProjectID+"/executions" {
			captured = r.URL.Query()
			_, _ = io.WriteString(w, `{"items":[],"totalCount":0,"page":1,"pageSize":20,"totalPages":0,"hasNextPage":false,"hasPreviousPage":false}`)
			return true
		}
		return false
	})
	defer server.Close()

	_, _, err := executeRootCommand(t, []string{
		"--json", "executions", "list",
		"--project", "my-project",
		"--status", "queued,running",
		"--agent-id", "agent-2",
		"--environment", "staging",
		"--page", "4",
		"--page-size", "25",
	}, triageBaseEnv(t, server.URL))
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	if got := captured.Get("status"); got != "queued,running" {
		t.Fatalf("expected status=queued,running, got %q", got)
	}
	if got := captured.Get("agentId"); got != "agent-2" {
		t.Fatalf("expected agentId=agent-2, got %q", got)
	}
	if got := captured.Get("environmentKey"); got != "staging" {
		t.Fatalf("expected environmentKey=staging, got %q", got)
	}
	if got := captured.Get("page"); got != "4" {
		t.Fatalf("expected page=4, got %q", got)
	}
	if got := captured.Get("pageSize"); got != "25" {
		t.Fatalf("expected pageSize=25, got %q", got)
	}
}

func TestIssuesListRejectsInvalidSeverityBeforeListCall(t *testing.T) {
	var listCalled bool
	server := newTriageTestServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/api/projects/"+mappedProjectID+"/tickets" {
			listCalled = true
		}
		return false
	})
	defer server.Close()

	_, _, err := executeRootCommand(t, []string{
		"issues", "list",
		"--project", "my-project",
		"--severity", "urgent",
	}, triageBaseEnv(t, server.URL))
	if err == nil {
		t.Fatal("expected validation error")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitUsage {
		t.Fatalf("expected usage exit code %d, got %d", ExitUsage, cmdErr.Code)
	}
	if listCalled {
		t.Fatal("tickets list endpoint should not be called when validation fails")
	}
}

func TestExecutionsListRejectsInvalidStatus(t *testing.T) {
	server := newTriageTestServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		return false
	})
	defer server.Close()

	_, _, err := executeRootCommand(t, []string{
		"executions", "list",
		"--project", "my-project",
		"--status", "queued,unknown",
	}, triageBaseEnv(t, server.URL))
	if err == nil {
		t.Fatal("expected validation error")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitUsage {
		t.Fatalf("expected usage exit code %d, got %d", ExitUsage, cmdErr.Code)
	}
}

func TestTriageGetCommandsEmitJSON(t *testing.T) {
	server := newTriageTestServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		switch r.URL.Path {
		case "/api/projects/" + mappedProjectID + "/testcases/tc-1":
			_, _ = io.WriteString(w, `{"id":"tc-1","number":1,"projectId":"`+mappedProjectID+`","name":"Login","description":"Login path","instructions":"Run steps","tags":["smoke"],"isQuarantined":false,"needsReview":false,"minimumSupportedEnvironmentVersion":"2026.01","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-02T00:00:00Z"}`)
			return true
		case "/api/projects/" + mappedProjectID + "/tickets/iss-1":
			_, _ = io.WriteString(w, `{"id":"iss-1","number":1,"projectId":"`+mappedProjectID+`","title":"Bug","description":"details","type":"bug","severity":"major","status":"open","labels":[],"needsReview":false,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-02T00:00:00Z","runActivity":"active","hasActiveExecution":true}`)
			return true
		case "/api/projects/" + mappedProjectID + "/executions/exe-1":
			_, _ = io.WriteString(w, `{"id":"exe-1","ticketId":"iss-1","status":"completed","triggerType":"manual","startedAt":"2026-01-01T00:00:00Z","completedAt":"2026-01-01T00:01:00Z","testCases":[],"notes":[],"artifacts":[]}`)
			return true
		default:
			return false
		}
	})
	defer server.Close()

	tests := []struct {
		name   string
		args   []string
		wantID string
	}{
		{
			name:   "testcases get",
			args:   []string{"--json", "testcases", "get", "--project", "my-project", "tc-1"},
			wantID: "tc-1",
		},
		{
			name:   "issues get",
			args:   []string{"--json", "issues", "get", "--project", "my-project", "iss-1"},
			wantID: "iss-1",
		},
		{
			name:   "executions get",
			args:   []string{"--json", "executions", "get", "--project", "my-project", "exe-1"},
			wantID: "exe-1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, _, err := executeRootCommand(t, tt.args, triageBaseEnv(t, server.URL))
			if err != nil {
				t.Fatalf("expected success, got error: %v", err)
			}

			var payload map[string]any
			if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
				t.Fatalf("failed to parse JSON output: %v\noutput: %s", err, stdout)
			}
			if payload["id"] != tt.wantID {
				t.Fatalf("expected id=%q, got %#v", tt.wantID, payload["id"])
			}
		})
	}
}

func TestExecutionsConversationUsesExecutionSessionAndMapsQuery(t *testing.T) {
	var captured url.Values
	server := newTriageTestServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		switch r.URL.Path {
		case "/api/projects/" + mappedProjectID + "/executions/exe-1":
			_, _ = io.WriteString(w, `{"id":"exe-1","ticketId":"iss-1","agentSessionId":"session-1","status":"completed","triggerType":"manual","startedAt":"2026-01-01T00:00:00Z","completedAt":"2026-01-01T00:01:00Z","testCases":[],"notes":[],"artifacts":[]}`)
			return true
		case "/api/projects/" + mappedProjectID + "/agentsessions/session-1/conversation":
			captured = r.URL.Query()
			_, _ = io.WriteString(w, `{"items":[{"type":"tool_result","timestampUtc":"2026-02-23T08:01:00Z","payload":{"tool":"open_url","ok":true}}],"totalCount":1,"page":2,"pageSize":50,"totalPages":1,"hasNextPage":false,"hasPreviousPage":true}`)
			return true
		default:
			return false
		}
	})
	defer server.Close()

	stdout, _, err := executeRootCommand(t, []string{
		"--json", "executions", "conversation",
		"--project", "my-project",
		"exe-1",
		"--before", "2026-02-23T08:00:00Z",
		"--page", "2",
		"--page-size", "50",
	}, triageBaseEnv(t, server.URL))
	if err != nil {
		t.Fatalf("expected conversation command success, got %v", err)
	}

	if got := captured.Get("before"); got != "2026-02-23T08:00:00Z" {
		t.Fatalf("expected before query, got %q", got)
	}
	if got := captured.Get("page"); got != "2" {
		t.Fatalf("expected page=2, got %q", got)
	}
	if got := captured.Get("pageSize"); got != "50" {
		t.Fatalf("expected pageSize=50, got %q", got)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("parse conversation json output: %v\noutput: %s", err, stdout)
	}
	if payload["execution_id"] != "exe-1" {
		t.Fatalf("expected execution_id=exe-1, got %#v", payload["execution_id"])
	}
	if payload["agent_session_id"] != "session-1" {
		t.Fatalf("expected agent_session_id=session-1, got %#v", payload["agent_session_id"])
	}
	items, ok := payload["items"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected 1 conversation item, got %#v", payload["items"])
	}
}

func TestExecutionsConversationFailsWhenExecutionHasNoSession(t *testing.T) {
	server := newTriageTestServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/api/projects/"+mappedProjectID+"/executions/exe-no-session" {
			_, _ = io.WriteString(w, `{"id":"exe-no-session","ticketId":"iss-1","status":"completed","triggerType":"manual","startedAt":"2026-01-01T00:00:00Z","completedAt":"2026-01-01T00:01:00Z","testCases":[],"notes":[],"artifacts":[]}`)
			return true
		}
		return false
	})
	defer server.Close()

	_, _, err := executeRootCommand(t, []string{
		"executions", "conversation",
		"--project", "my-project",
		"exe-no-session",
	}, triageBaseEnv(t, server.URL))
	if err == nil {
		t.Fatal("expected missing session error")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitNotFound {
		t.Fatalf("expected not found exit code %d, got %d", ExitNotFound, cmdErr.Code)
	}
}

func TestExecutionsConversationRejectsInvalidBeforeTimestamp(t *testing.T) {
	server := newTriageTestServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/api/projects/"+mappedProjectID+"/executions/exe-1" {
			_, _ = io.WriteString(w, `{"id":"exe-1","ticketId":"iss-1","agentSessionId":"session-1","status":"completed","triggerType":"manual","startedAt":"2026-01-01T00:00:00Z","completedAt":"2026-01-01T00:01:00Z","testCases":[],"notes":[],"artifacts":[]}`)
			return true
		}
		if strings.Contains(r.URL.Path, "/agentsessions/session-1/conversation") {
			t.Fatal("conversation endpoint should not be called for invalid --before")
		}
		return false
	})
	defer server.Close()

	_, _, err := executeRootCommand(t, []string{
		"executions", "conversation",
		"--project", "my-project",
		"exe-1",
		"--before", "2026-02-23",
	}, triageBaseEnv(t, server.URL))
	if err == nil {
		t.Fatal("expected usage error")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitUsage {
		t.Fatalf("expected usage exit code %d, got %d", ExitUsage, cmdErr.Code)
	}
}

func TestExecutionsDiagnoseEmitsStructuredDiagnostics(t *testing.T) {
	server := newTriageTestServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		switch r.URL.Path {
		case "/api/projects/" + mappedProjectID + "/executions/exe-1":
			_, _ = io.WriteString(w, `{"id":"exe-1","ticketId":"iss-1","agentSessionId":"session-1","status":"failed","triggerType":"manual","startedAt":"2026-01-01T00:00:00Z","completedAt":"2026-01-01T00:01:00Z","testCases":[],"notes":[],"artifacts":[]}`)
			return true
		case "/api/projects/" + mappedProjectID + "/agentsessions/session-1/conversation":
			_, _ = io.WriteString(w, `{
				"items":[
					{"type":"tool_use_requested","timestampUtc":"2026-02-23T08:01:00Z","payload":{"toolName":"computer","toolCallId":"call-1"}},
					{"type":"policy_decision","timestampUtc":"2026-02-23T08:01:01Z","payload":{"allowed":false,"toolName":"computer","toolCallId":"call-1"}},
					{"type":"tool_result","timestampUtc":"2026-02-23T08:01:02Z","payload":{"status":"failed","output":"click failed","metadata":{"toolName":"computer","toolCallId":"call-1"}}},
					{"type":"browser_network_request","timestampUtc":"2026-02-23T08:01:03Z","payload":{"requestId":"req-1","method":"POST","url":"https://api.example.com/do?token=raw-token"}},
					{"type":"browser_network_response","timestampUtc":"2026-02-23T08:01:04Z","payload":{"requestId":"req-1","status":500,"statusText":"Server Error","url":"https://api.example.com/do?token=raw-token"}},
					{"type":"conversation_completed","timestampUtc":"2026-02-23T08:01:05Z","payload":{"status":"failed"}}
				],
				"totalCount":6,
				"page":1,
				"pageSize":500,
				"totalPages":1,
				"hasNextPage":false,
				"hasPreviousPage":false
			}`)
			return true
		default:
			return false
		}
	})
	defer server.Close()

	stdout, _, err := executeRootCommand(t, []string{
		"--json", "executions", "diagnose",
		"--project", "my-project",
		"exe-1",
	}, triageBaseEnv(t, server.URL))
	if err != nil {
		t.Fatalf("expected diagnose success, got %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("parse diagnose json output: %v\noutput: %s", err, stdout)
	}
	if payload["schema_version"] != executionDiagnoseSchemaVersion {
		t.Fatalf("expected schema_version=%s, got %#v", executionDiagnoseSchemaVersion, payload["schema_version"])
	}
	if payload["execution_id"] != "exe-1" {
		t.Fatalf("expected execution_id=exe-1, got %#v", payload["execution_id"])
	}
	if payload["primary_failure_reason"] != "conversation_failed" {
		t.Fatalf("expected primary_failure_reason=conversation_failed, got %#v", payload["primary_failure_reason"])
	}
	counts, ok := payload["counts"].(map[string]any)
	if !ok {
		t.Fatalf("expected counts object, got %#v", payload["counts"])
	}
	if int(counts["tool_errors"].(float64)) != 1 {
		t.Fatalf("expected tool_errors=1, got %#v", counts["tool_errors"])
	}
	if int(counts["policy_denied"].(float64)) != 1 {
		t.Fatalf("expected policy_denied=1, got %#v", counts["policy_denied"])
	}
	if int(counts["network_5xx"].(float64)) != 1 {
		t.Fatalf("expected network_5xx=1, got %#v", counts["network_5xx"])
	}
	rawNetworkFailures, ok := payload["network_failures"].([]any)
	if !ok || len(rawNetworkFailures) != 1 {
		t.Fatalf("expected network_failures with one item, got %#v", payload["network_failures"])
	}
	networkFailure, ok := rawNetworkFailures[0].(map[string]any)
	if !ok {
		t.Fatalf("expected network failure object, got %#v", rawNetworkFailures[0])
	}
	if !strings.Contains(networkFailure["url"].(string), "token=raw-token") {
		t.Fatalf("expected no redaction in network url, got %#v", networkFailure["url"])
	}
	if !strings.Contains(payload["conversation_command"].(string), "executions conversation") {
		t.Fatalf("expected conversation command hint, got %#v", payload["conversation_command"])
	}
}

func TestExecutionsDiagnoseRespectsBeforeAndMaxEventsTruncation(t *testing.T) {
	var pages []string
	var befores []string
	server := newTriageTestServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		switch r.URL.Path {
		case "/api/projects/" + mappedProjectID + "/executions/exe-1":
			_, _ = io.WriteString(w, `{"id":"exe-1","ticketId":"iss-1","agentSessionId":"session-1","status":"failed","triggerType":"manual","testCases":[],"notes":[],"artifacts":[]}`)
			return true
		case "/api/projects/" + mappedProjectID + "/agentsessions/session-1/conversation":
			query := r.URL.Query()
			pages = append(pages, query.Get("page"))
			befores = append(befores, query.Get("before"))
			switch query.Get("page") {
			case "1":
				_, _ = io.WriteString(w, `{"items":[{"type":"browser_network_response","timestampUtc":"2026-02-23T08:01:00Z","payload":{"requestId":"req-1","status":401,"statusText":"Unauthorized","url":"https://api.example.com/a"}}],"totalCount":3,"page":1,"pageSize":1,"totalPages":3,"hasNextPage":true,"hasPreviousPage":false}`)
				return true
			case "2":
				_, _ = io.WriteString(w, `{"items":[{"type":"browser_network_response","timestampUtc":"2026-02-23T08:01:01Z","payload":{"requestId":"req-2","status":500,"statusText":"Server Error","url":"https://api.example.com/b"}}],"totalCount":3,"page":2,"pageSize":1,"totalPages":3,"hasNextPage":true,"hasPreviousPage":true}`)
				return true
			case "3":
				t.Fatal("did not expect page 3 fetch when max-events reached")
				return true
			default:
				t.Fatalf("unexpected page query: %q", query.Get("page"))
				return true
			}
		default:
			return false
		}
	})
	defer server.Close()

	before := "2026-02-23T08:00:00Z"
	stdout, _, err := executeRootCommand(t, []string{
		"--json", "executions", "diagnose",
		"--project", "my-project",
		"exe-1",
		"--before", before,
		"--page-size", "1",
		"--max-events", "2",
		"--sample-size", "1",
	}, triageBaseEnv(t, server.URL))
	if err != nil {
		t.Fatalf("expected diagnose success, got %v", err)
	}

	if len(pages) != 2 || pages[0] != "1" || pages[1] != "2" {
		t.Fatalf("expected page sequence [1 2], got %#v", pages)
	}
	for _, value := range befores {
		if value != before {
			t.Fatalf("expected before=%s on every request, got %q", before, value)
		}
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("parse diagnose json output: %v\noutput: %s", err, stdout)
	}
	eventWindow, ok := payload["event_window"].(map[string]any)
	if !ok {
		t.Fatalf("expected event_window object, got %#v", payload["event_window"])
	}
	if scanned := int(eventWindow["scanned"].(float64)); scanned != 2 {
		t.Fatalf("expected scanned=2, got %d", scanned)
	}
	if truncated, _ := eventWindow["truncated"].(bool); !truncated {
		t.Fatalf("expected truncated=true, got %#v", eventWindow["truncated"])
	}
	if hasMore, _ := eventWindow["has_more"].(bool); !hasMore {
		t.Fatalf("expected has_more=true, got %#v", eventWindow["has_more"])
	}
	if nextPage := int(eventWindow["next_page"].(float64)); nextPage != 3 {
		t.Fatalf("expected next_page=3, got %d", nextPage)
	}
	if eventWindow["before"] != before {
		t.Fatalf("expected event_window.before=%s, got %#v", before, eventWindow["before"])
	}
	rawNetworkFailures, ok := payload["network_failures"].([]any)
	if !ok || len(rawNetworkFailures) != 1 {
		t.Fatalf("expected sample-size cap to 1 network failure, got %#v", payload["network_failures"])
	}
	counts, ok := payload["counts"].(map[string]any)
	if !ok {
		t.Fatalf("expected counts object, got %#v", payload["counts"])
	}
	if int(counts["network_4xx"].(float64)) != 1 || int(counts["network_5xx"].(float64)) != 1 {
		t.Fatalf("expected network_4xx=1 and network_5xx=1, got %#v", counts)
	}
}

func TestTriageListWithMissingProjectMappingFailsBeforeAPICall(t *testing.T) {
	var called atomic.Bool
	server := newTriageTestServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		called.Store(true)
		return false
	})
	defer server.Close()

	_, _, err := executeRootCommand(t, []string{
		"testcases", "list",
		"--project", "unknown-project",
	}, triageBaseEnv(t, server.URL))
	if err == nil {
		t.Fatal("expected usage error for missing project mapping")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitUsage {
		t.Fatalf("expected usage exit code %d, got %d", ExitUsage, cmdErr.Code)
	}
	if called.Load() {
		t.Fatal("expected no API call when project mapping is missing")
	}
}

func TestTriageListWithStaleProjectMappingReturnsRemediationHint(t *testing.T) {
	server := newTriageTestServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.URL.Path == "/api/projects/"+mappedProjectID+"/testcases" {
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"error":"project not found"}`)
			return true
		}
		return false
	})
	defer server.Close()

	_, _, err := executeRootCommand(t, []string{
		"testcases", "list",
		"--project", "my-project",
	}, triageBaseEnv(t, server.URL))
	if err == nil {
		t.Fatal("expected stale mapping error")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitUsage {
		t.Fatalf("expected usage exit code %d, got %d", ExitUsage, cmdErr.Code)
	}
	if got := cmdErr.Error(); got == "" || !containsAll(got, "appears stale", "config set --profile", "--project my-project") {
		t.Fatalf("expected remediation hint, got %q", got)
	}
}

func TestTestcasesCreateUsesMappedProjectAndPayload(t *testing.T) {
	var captured map[string]any
	server := newTriageTestServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodPost && r.URL.Path == "/api/projects/"+mappedProjectID+"/testcases" {
			var err error
			captured, err = decodeJSONMap(r.Body)
			if err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			_, _ = io.WriteString(w, `{"id":"tc-42","number":42,"projectId":"`+mappedProjectID+`","name":"Checkout","description":"desc","instructions":"steps","tags":["smoke","critical"],"isQuarantined":true,"needsReview":false,"minimumSupportedEnvironmentVersion":"2026.02","createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}`)
			return true
		}
		return false
	})
	defer server.Close()

	stdout, _, err := executeRootCommand(t, []string{
		"--json", "testcases", "create",
		"--project", "my-project",
		"--name", "Checkout",
		"--instructions", "steps",
		"--description", "desc",
		"--tag", "smoke",
		"--tag", "critical",
		"--quarantined=true",
		"--needs-review=false",
		"--min-env-version", "2026.02",
		"--created-by-agent", "agent-template",
		"--created-during-execution", "exe-1",
	}, triageBaseEnv(t, server.URL))
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if captured["name"] != "Checkout" || captured["instructions"] != "steps" {
		t.Fatalf("unexpected payload: %#v", captured)
	}
	if captured["isQuarantined"] != true {
		t.Fatalf("expected isQuarantined=true, got %#v", captured["isQuarantined"])
	}
	if captured["needsReview"] != false {
		t.Fatalf("expected needsReview=false, got %#v", captured["needsReview"])
	}
	if captured["minimumSupportedEnvironmentVersion"] != "2026.02" {
		t.Fatalf("expected min env version in payload, got %#v", captured["minimumSupportedEnvironmentVersion"])
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("parse JSON output: %v", err)
	}
	if payload["id"] != "tc-42" {
		t.Fatalf("expected created testcase id tc-42, got %#v", payload["id"])
	}
}

func TestTestcasesUpdateRequiresAtLeastOneField(t *testing.T) {
	var called atomic.Bool
	server := newTriageTestServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		called.Store(true)
		return false
	})
	defer server.Close()

	_, _, err := executeRootCommand(t, []string{
		"testcases", "update", "--project", "my-project", "tc-1",
	}, triageBaseEnv(t, server.URL))
	if err == nil {
		t.Fatal("expected usage error")
	}
	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitUsage {
		t.Fatalf("expected usage exit code %d, got %d", ExitUsage, cmdErr.Code)
	}
	if called.Load() {
		t.Fatal("API should not be called when no update fields are provided")
	}
}

func TestTestcasesExecuteBulkRequiresIDs(t *testing.T) {
	var called atomic.Bool
	server := newTriageTestServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		called.Store(true)
		return false
	})
	defer server.Close()

	_, _, err := executeRootCommand(t, []string{
		"testcases", "execute-bulk", "--project", "my-project",
	}, triageBaseEnv(t, server.URL))
	if err == nil {
		t.Fatal("expected usage error")
	}
	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitUsage {
		t.Fatalf("expected usage exit code %d, got %d", ExitUsage, cmdErr.Code)
	}
	if called.Load() {
		t.Fatal("API should not be called when --id is missing")
	}
}

func TestTestcasesExecuteBulkSendsArrayPayload(t *testing.T) {
	var captured []string
	server := newTriageTestServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodPost && r.URL.Path == "/api/projects/"+mappedProjectID+"/testcases/execute" {
			defer func() { _ = r.Body.Close() }()
			raw, err := io.ReadAll(r.Body)
			if err != nil {
				t.Fatalf("read request body: %v", err)
			}
			if err := json.Unmarshal(raw, &captured); err != nil {
				t.Fatalf("expected JSON string array body, got %s (err=%v)", string(raw), err)
			}
			_, _ = io.WriteString(w, `{"successCount":1,"failureCount":0,"executions":[{"testCaseId":"tc-1","executionId":"exe-1","status":"Created"}]}`)
			return true
		}
		return false
	})
	defer server.Close()

	stdout, _, err := executeRootCommand(t, []string{
		"--json", "testcases", "execute-bulk",
		"--project", "my-project",
		"--id", "tc-1",
	}, triageBaseEnv(t, server.URL))
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if len(captured) != 1 || captured[0] != "tc-1" {
		t.Fatalf("expected captured ids [tc-1], got %#v", captured)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("parse JSON output: %v", err)
	}
	if payload["successCount"] != float64(1) {
		t.Fatalf("expected successCount=1, got %#v", payload["successCount"])
	}
}

func TestIssuesCreateViaTicketsAliasUsesMappedProject(t *testing.T) {
	var captured map[string]any
	server := newTriageTestServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodPost && r.URL.Path == "/api/projects/"+mappedProjectID+"/tickets" {
			var err error
			captured, err = decodeJSONMap(r.Body)
			if err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			_, _ = io.WriteString(w, `{"id":"iss-7","number":7,"projectId":"`+mappedProjectID+`","title":"Broken login","description":"details","type":"bug","severity":"major","status":"open","labels":["ui"],"needsReview":true,"createdAt":"2026-01-01T00:00:00Z","updatedAt":"2026-01-01T00:00:00Z"}`)
			return true
		}
		return false
	})
	defer server.Close()

	stdout, _, err := executeRootCommand(t, []string{
		"--json", "tickets", "create",
		"--project", "my-project",
		"--title", "Broken login",
		"--type", "bug",
		"--severity", "major",
		"--status", "open",
		"--label", "ui",
		"--needs-review=true",
	}, triageBaseEnv(t, server.URL))
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if captured["title"] != "Broken login" || captured["type"] != "bug" {
		t.Fatalf("unexpected payload: %#v", captured)
	}
	if captured["needsReview"] != true {
		t.Fatalf("expected needsReview=true, got %#v", captured["needsReview"])
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("parse JSON output: %v", err)
	}
	if payload["id"] != "iss-7" {
		t.Fatalf("expected issue id iss-7, got %#v", payload["id"])
	}
}

func TestIssuesAttachmentAddFileEncodesBase64(t *testing.T) {
	var captured map[string]any
	server := newTriageTestServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodPost && r.URL.Path == "/api/projects/"+mappedProjectID+"/tickets/iss-1/attachments" {
			var err error
			captured, err = decodeJSONMap(r.Body)
			if err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			_, _ = io.WriteString(w, `{"id":"att-1","kind":"screenshot","url":null,"description":"evidence","createdAt":"2026-01-01T00:00:00Z"}`)
			return true
		}
		return false
	})
	defer server.Close()

	filePath := filepath.Join(t.TempDir(), "shot.txt")
	if err := os.WriteFile(filePath, []byte("hello-world"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	_, _, err := executeRootCommand(t, []string{
		"--json", "issues", "attachment", "add", "--project", "my-project", "iss-1",
		"--kind", "screenshot",
		"--file", filePath,
		"--description", "evidence",
	}, triageBaseEnv(t, server.URL))
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if captured["kind"] != "screenshot" {
		t.Fatalf("expected kind=screenshot, got %#v", captured["kind"])
	}
	if captured["description"] != "evidence" {
		t.Fatalf("expected description=evidence, got %#v", captured["description"])
	}
	if captured["url"] != nil {
		t.Fatalf("expected url to be null/absent, got %#v", captured["url"])
	}
	gotBase64, ok := captured["base64Data"].(string)
	if !ok || gotBase64 == "" {
		t.Fatalf("expected base64Data in payload, got %#v", captured["base64Data"])
	}
	if gotBase64 != base64.StdEncoding.EncodeToString([]byte("hello-world")) {
		t.Fatalf("unexpected base64Data value: %q", gotBase64)
	}
}

func TestIssuesAttachmentAddURLPayload(t *testing.T) {
	var captured map[string]any
	server := newTriageTestServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodPost && r.URL.Path == "/api/projects/"+mappedProjectID+"/tickets/iss-1/attachments" {
			var err error
			captured, err = decodeJSONMap(r.Body)
			if err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			_, _ = io.WriteString(w, `{"id":"att-2","kind":"log","url":"https://example.com/file.txt","description":"ref","createdAt":"2026-01-01T00:00:00Z"}`)
			return true
		}
		return false
	})
	defer server.Close()

	_, _, err := executeRootCommand(t, []string{
		"--json", "issues", "attachment", "add", "--project", "my-project", "iss-1",
		"--kind", "log",
		"--url", "https://example.com/file.txt",
		"--description", "ref",
	}, triageBaseEnv(t, server.URL))
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if captured["url"] != "https://example.com/file.txt" {
		t.Fatalf("expected url in payload, got %#v", captured["url"])
	}
	if _, ok := captured["base64Data"]; ok {
		t.Fatalf("base64Data should not be set for URL mode, payload: %#v", captured)
	}
}

func TestIssuesAttachmentAddRejectsFileAndURL(t *testing.T) {
	var called atomic.Bool
	server := newTriageTestServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		called.Store(true)
		return false
	})
	defer server.Close()

	filePath := filepath.Join(t.TempDir(), "shot.txt")
	if err := os.WriteFile(filePath, []byte("content"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	_, _, err := executeRootCommand(t, []string{
		"issues", "attachment", "add", "--project", "my-project", "iss-1",
		"--kind", "log",
		"--file", filePath,
		"--url", "https://example.com/file.txt",
	}, triageBaseEnv(t, server.URL))
	if err == nil {
		t.Fatal("expected usage error")
	}
	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitUsage {
		t.Fatalf("expected usage exit code %d, got %d", ExitUsage, cmdErr.Code)
	}
	if called.Load() {
		t.Fatal("API should not be called for invalid attachment input")
	}
}

func TestIssuesVerificationAddRejectsInvalidDecision(t *testing.T) {
	var called atomic.Bool
	server := newTriageTestServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		called.Store(true)
		return false
	})
	defer server.Close()

	_, _, err := executeRootCommand(t, []string{
		"issues", "verification", "add", "--project", "my-project", "iss-1",
		"--decision", "unknown",
		"--summary", "summary",
	}, triageBaseEnv(t, server.URL))
	if err == nil {
		t.Fatal("expected usage error")
	}
	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitUsage {
		t.Fatalf("expected usage exit code %d, got %d", ExitUsage, cmdErr.Code)
	}
	if called.Load() {
		t.Fatal("API should not be called for invalid decision")
	}
}

func TestIssuesVerificationAddScreenshotEncodesBase64(t *testing.T) {
	var captured map[string]any
	server := newTriageTestServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		if r.Method == http.MethodPost && r.URL.Path == "/api/projects/"+mappedProjectID+"/tickets/iss-1/verifications" {
			var err error
			captured, err = decodeJSONMap(r.Body)
			if err != nil {
				t.Fatalf("decode request body: %v", err)
			}
			_, _ = io.WriteString(w, `{"id":"ver-1","decision":"passed","summary":"done","expected":"a","observed":"b","base64Screenshot":"x","verifiedAt":"2026-01-01T00:00:00Z"}`)
			return true
		}
		return false
	})
	defer server.Close()

	filePath := filepath.Join(t.TempDir(), "shot.txt")
	if err := os.WriteFile(filePath, []byte("shot"), 0o600); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	_, _, err := executeRootCommand(t, []string{
		"--json", "issues", "verification", "add", "--project", "my-project", "iss-1",
		"--decision", "passed",
		"--summary", "done",
		"--expected", "a",
		"--observed", "b",
		"--screenshot-file", filePath,
	}, triageBaseEnv(t, server.URL))
	if err != nil {
		t.Fatalf("expected success, got %v", err)
	}

	if captured["decision"] != "passed" || captured["summary"] != "done" {
		t.Fatalf("unexpected payload: %#v", captured)
	}
	if captured["base64Screenshot"] != base64.StdEncoding.EncodeToString([]byte("shot")) {
		t.Fatalf("unexpected screenshot payload: %#v", captured["base64Screenshot"])
	}
}

func TestExecutionsRetryAndStopUseMappedProjectRoutes(t *testing.T) {
	var retryCalled atomic.Bool
	var stopCalled atomic.Bool
	server := newTriageTestServer(t, func(w http.ResponseWriter, r *http.Request) bool {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/api/projects/"+mappedProjectID+"/executions/exe-1/retry":
			retryCalled.Store(true)
			_, _ = io.WriteString(w, `{"id":"exe-2","ticketId":"iss-1","status":"queued","triggerType":"manual","testCases":[],"notes":[],"artifacts":[]}`)
			return true
		case r.Method == http.MethodPost && r.URL.Path == "/api/projects/"+mappedProjectID+"/executions/exe-2/stop":
			stopCalled.Store(true)
			w.WriteHeader(http.StatusNoContent)
			return true
		default:
			return false
		}
	})
	defer server.Close()

	stdout, _, err := executeRootCommand(t, []string{
		"--json", "executions", "retry", "--project", "my-project", "exe-1",
	}, triageBaseEnv(t, server.URL))
	if err != nil {
		t.Fatalf("retry should succeed, got %v", err)
	}
	if !retryCalled.Load() {
		t.Fatal("expected retry endpoint call")
	}
	var retryPayload map[string]any
	if err := json.Unmarshal([]byte(stdout), &retryPayload); err != nil {
		t.Fatalf("parse retry json: %v", err)
	}
	if retryPayload["id"] != "exe-2" {
		t.Fatalf("expected retried execution id exe-2, got %#v", retryPayload["id"])
	}

	stopOut, _, err := executeRootCommand(t, []string{
		"--json", "executions", "stop", "--project", "my-project", "exe-2",
	}, triageBaseEnv(t, server.URL))
	if err != nil {
		t.Fatalf("stop should succeed, got %v", err)
	}
	if !stopCalled.Load() {
		t.Fatal("expected stop endpoint call")
	}
	var stopPayload map[string]any
	if err := json.Unmarshal([]byte(stopOut), &stopPayload); err != nil {
		t.Fatalf("parse stop json: %v", err)
	}
	if stopped, ok := stopPayload["stopped"].(bool); !ok || !stopped {
		t.Fatalf("expected stopped=true, got %#v", stopPayload["stopped"])
	}
}

func triageBaseEnv(t *testing.T, apiURL string) map[string]string {
	t.Helper()
	configHome := t.TempDir()
	configPath := filepath.Join(configHome, "certyn", "config.yaml")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o700); err != nil {
		t.Fatalf("create config directory: %v", err)
	}
	configBody := "active_profile: default\nprofiles:\n  default:\n    project_ids:\n      my-project: " + mappedProjectID + "\n"
	if err := os.WriteFile(configPath, []byte(configBody), 0o600); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	return map[string]string{
		"CERTYN_API_URL":  apiURL,
		"CERTYN_API_KEY":  "test-key",
		"XDG_CONFIG_HOME": configHome,
	}
}

func newTriageTestServer(t *testing.T, additional func(http.ResponseWriter, *http.Request) bool) *httptest.Server {
	t.Helper()

	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if additional != nil && additional(w, r) {
			return
		}
		http.NotFound(w, r)
	}))
}

func containsAll(value string, needles ...string) bool {
	for _, needle := range needles {
		if !strings.Contains(value, needle) {
			return false
		}
	}
	return true
}

func decodeJSONMap(body io.ReadCloser) (map[string]any, error) {
	defer func() { _ = body.Close() }()
	var payload map[string]any
	raw, err := io.ReadAll(body)
	if err != nil {
		return nil, err
	}
	if len(raw) == 0 {
		return map[string]any{}, nil
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, err
	}
	return payload, nil
}
