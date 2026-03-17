package cli

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestAskJSONSuccessWithExplicitProjectSlug(t *testing.T) {
	var advisorPayload map[string]any

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/overview":
			_, _ = io.WriteString(w, `{"defaultProjectId":"`+verifyProjectID+`","projects":[{"id":"`+verifyProjectID+`","slug":"my-project","name":"My Project"}]}`)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/chat/advisor":
			var err error
			advisorPayload, err = decodeJSONMap(r.Body)
			if err != nil {
				t.Fatalf("decode advisor payload: %v", err)
			}
			_, _ = io.WriteString(w, `{
				"conversationId":"advisor-1",
				"messageId":"msg-1",
				"content":"Investigate checkout token refresh.",
				"role":"assistant",
				"toolCalls":[{"toolName":"query_execution_history","success":true,"result":{"count":3},"error":""}],
				"createdAt":"2026-02-23T09:00:00Z"
			}`)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer apiServer.Close()

	stdout, _, err := executeRootCommand(t, []string{
		"--json",
		"ask",
		"--project", "my-project",
		"--context", "verify failed with network_401 on /api/checkout",
		"--max-tool-iterations", "7",
		"--max-output-tokens", "900",
		"Why did checkout fail after login?",
	}, verifyBaseEnv(t.TempDir(), apiServer.URL))
	if err != nil {
		t.Fatalf("ask should succeed, got %v", err)
	}

	if advisorPayload["projectId"] != verifyProjectID {
		t.Fatalf("expected resolved projectId=%s, got %#v", verifyProjectID, advisorPayload["projectId"])
	}
	if advisorPayload["message"] != "Why did checkout fail after login?\n\nAdditional context:\nverify failed with network_401 on /api/checkout" {
		t.Fatalf("unexpected message payload: %#v", advisorPayload["message"])
	}
	if int(advisorPayload["maxToolIterations"].(float64)) != 7 {
		t.Fatalf("expected maxToolIterations=7, got %#v", advisorPayload["maxToolIterations"])
	}
	if int(advisorPayload["maxOutputTokenCount"].(float64)) != 900 {
		t.Fatalf("expected maxOutputTokenCount=900, got %#v", advisorPayload["maxOutputTokenCount"])
	}

	payload := parseAskJSONOutput(t, stdout)
	if payload["schema_version"] != askSchemaVersion {
		t.Fatalf("expected schema_version=%s, got %#v", askSchemaVersion, payload["schema_version"])
	}
	if payload["mode"] != "advisor" {
		t.Fatalf("expected mode=advisor, got %#v", payload["mode"])
	}
	if payload["question"] != "Why did checkout fail after login?" {
		t.Fatalf("unexpected question: %#v", payload["question"])
	}
	if payload["project_input"] != "my-project" {
		t.Fatalf("expected project_input=my-project, got %#v", payload["project_input"])
	}
	if payload["project_id"] != verifyProjectID {
		t.Fatalf("expected project_id=%s, got %#v", verifyProjectID, payload["project_id"])
	}
	if used, _ := payload["used_project_context"].(bool); !used {
		t.Fatalf("expected used_project_context=true, got %#v", payload["used_project_context"])
	}
	if payload["conversation_id"] != "advisor-1" {
		t.Fatalf("expected conversation_id=advisor-1, got %#v", payload["conversation_id"])
	}
	if payload["content"] != "Investigate checkout token refresh." {
		t.Fatalf("unexpected content: %#v", payload["content"])
	}
	if code := int(payload["exit_code"].(float64)); code != ExitOK {
		t.Fatalf("expected exit_code=%d, got %d", ExitOK, code)
	}
}

func TestAskJSONSuccessWithoutProject(t *testing.T) {
	var advisorPayload map[string]any

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/chat/advisor" {
			http.NotFound(w, r)
			return
		}
		var err error
		advisorPayload, err = decodeJSONMap(r.Body)
		if err != nil {
			t.Fatalf("decode advisor payload: %v", err)
		}
		_, _ = io.WriteString(w, `{
			"conversationId":"advisor-2",
			"messageId":"msg-2",
			"content":"Start with smoke-suite and check execution diagnostics.",
			"role":"assistant",
			"toolCalls":[],
			"createdAt":"2026-02-23T09:10:00Z"
		}`)
	}))
	defer apiServer.Close()

	stdout, _, err := executeRootCommand(t, []string{
		"--json",
		"ask",
		"What should I check next?",
	}, verifyBaseEnv(t.TempDir(), apiServer.URL))
	if err != nil {
		t.Fatalf("ask without project should succeed, got %v", err)
	}

	if _, hasProject := advisorPayload["projectId"]; hasProject {
		t.Fatalf("expected no projectId in advisor request, got %#v", advisorPayload["projectId"])
	}

	payload := parseAskJSONOutput(t, stdout)
	if used, _ := payload["used_project_context"].(bool); used {
		t.Fatalf("expected used_project_context=false, got %#v", payload["used_project_context"])
	}
	if _, hasProjectID := payload["project_id"]; hasProjectID {
		t.Fatalf("expected no project_id output, got %#v", payload["project_id"])
	}
	if code := int(payload["exit_code"].(float64)); code != ExitOK {
		t.Fatalf("expected exit_code=%d, got %d", ExitOK, code)
	}
}

func TestAskMissingQuestionReturnsUsageError(t *testing.T) {
	stdout, _, err := executeRootCommand(t, []string{
		"--json",
		"ask",
	}, verifyBaseEnv(t.TempDir(), "https://api.certyn.example"))
	if err == nil {
		t.Fatal("expected ask to fail with missing question")
	}
	assertCommandErrorCode(t, err, ExitUsage)

	payload := parseAskJSONOutput(t, stdout)
	if code := int(payload["exit_code"].(float64)); code != ExitUsage {
		t.Fatalf("expected exit_code=%d, got %d", ExitUsage, code)
	}
	if !strings.Contains(strings.ToLower(payload["error"].(string)), "missing question") {
		t.Fatalf("expected missing question error, got %#v", payload["error"])
	}
}

func TestAskExplicitInvalidProjectFails(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/overview":
			_, _ = io.WriteString(w, `{"defaultProjectId":"`+verifyProjectID+`","projects":[{"id":"`+verifyProjectID+`","slug":"my-project","name":"My Project"}]}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/missing-project":
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/chat/advisor":
			t.Fatal("advisor endpoint should not be called when explicit project resolution fails")
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer apiServer.Close()

	stdout, _, err := executeRootCommand(t, []string{
		"--json",
		"ask",
		"--project", "missing-project",
		"Why did this fail?",
	}, verifyBaseEnv(t.TempDir(), apiServer.URL))
	if err == nil {
		t.Fatal("expected ask to fail for explicit missing project")
	}
	assertCommandErrorCode(t, err, ExitNotFound)

	payload := parseAskJSONOutput(t, stdout)
	if code := int(payload["exit_code"].(float64)); code != ExitNotFound {
		t.Fatalf("expected exit_code=%d, got %d", ExitNotFound, code)
	}
}

func TestAskAuthFailureReturnsExitAuth(t *testing.T) {
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/chat/advisor" {
			http.NotFound(w, r)
			return
		}
		http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
	}))
	defer apiServer.Close()

	stdout, _, err := executeRootCommand(t, []string{
		"--json",
		"ask",
		"What should I do?",
	}, verifyBaseEnv(t.TempDir(), apiServer.URL))
	if err == nil {
		t.Fatal("expected ask to fail with auth error")
	}
	assertCommandErrorCode(t, err, ExitAuth)

	payload := parseAskJSONOutput(t, stdout)
	if code := int(payload["exit_code"].(float64)); code != ExitAuth {
		t.Fatalf("expected exit_code=%d, got %d", ExitAuth, code)
	}
}

func TestAskImplicitStaleProjectFallsBackWithWarning(t *testing.T) {
	var advisorPayload map[string]any

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/overview":
			_, _ = io.WriteString(w, `{"defaultProjectId":"`+verifyProjectID+`","projects":[{"id":"`+verifyProjectID+`","slug":"my-project","name":"My Project"}]}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/stale-project":
			http.Error(w, `{"error":"not found"}`, http.StatusNotFound)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/chat/advisor":
			var err error
			advisorPayload, err = decodeJSONMap(r.Body)
			if err != nil {
				t.Fatalf("decode advisor payload: %v", err)
			}
			_, _ = io.WriteString(w, `{
				"conversationId":"advisor-3",
				"messageId":"msg-3",
				"content":"Check failing execution diagnostics first.",
				"role":"assistant",
				"toolCalls":[],
				"createdAt":"2026-02-23T09:20:00Z"
			}`)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer apiServer.Close()

	env := verifyBaseEnv(t.TempDir(), apiServer.URL)
	env["CERTYN_PROJECT"] = "stale-project"

	stdout, _, err := executeRootCommand(t, []string{
		"--json",
		"ask",
		"How should I continue?",
	}, env)
	if err != nil {
		t.Fatalf("expected ask success with implicit stale project fallback, got %v", err)
	}

	if _, hasProject := advisorPayload["projectId"]; hasProject {
		t.Fatalf("expected advisor request without projectId, got %#v", advisorPayload["projectId"])
	}

	payload := parseAskJSONOutput(t, stdout)
	if payload["project_input"] != "stale-project" {
		t.Fatalf("expected project_input=stale-project, got %#v", payload["project_input"])
	}
	if used, _ := payload["used_project_context"].(bool); used {
		t.Fatalf("expected used_project_context=false, got %#v", payload["used_project_context"])
	}
	warnings, ok := payload["warnings"].([]any)
	if !ok || len(warnings) == 0 {
		t.Fatalf("expected non-empty warnings, got %#v", payload["warnings"])
	}
	if !strings.Contains(strings.ToLower(warnings[0].(string)), "continuing without project context") {
		t.Fatalf("expected fallback warning text, got %#v", warnings[0])
	}
}

func parseAskJSONOutput(t *testing.T, raw string) map[string]any {
	t.Helper()

	var payload map[string]any
	if err := json.Unmarshal([]byte(raw), &payload); err != nil {
		t.Fatalf("parse ask json output: %v\noutput: %s", err, raw)
	}
	return payload
}

func assertCommandErrorCode(t *testing.T, err error, expected int) {
	t.Helper()

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != expected {
		t.Fatalf("expected command error code=%d, got %d (%v)", expected, cmdErr.Code, cmdErr)
	}
}
