package cli

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/certyn/certyn-cli/internal/api"
)

const verifyProjectID = "11111111-2222-4333-8444-555555555555"

func TestVerifySuiteModeCreatesEnvironmentRunsAndCleansUp(t *testing.T) {
	localApp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer localApp.Close()

	var ciPayload map[string]any
	var envPayload map[string]any
	var deleteCalled atomic.Bool

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/overview":
			_, _ = io.WriteString(w, `{"defaultProjectId":"`+verifyProjectID+`","projects":[{"id":"`+verifyProjectID+`","slug":"my-project","name":"My Project"}]}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+verifyProjectID+"/processes":
			_, _ = io.WriteString(w, `[
				{
					"id":"proc-smoke",
					"projectId":"`+verifyProjectID+`",
					"name":"Smoke",
					"slug":"smoke-suite",
					"isExploratory":false,
					"isActive":true,
					"configuration":{"ticketLabels":[],"testCaseTags":["smoke"]}
				}
			]`)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/projects/"+verifyProjectID+"/environments":
			var err error
			envPayload, err = decodeJSONMap(r.Body)
			if err != nil {
				t.Fatalf("decode environment body: %v", err)
			}
			_, _ = io.WriteString(w, `{"id":"env-1","projectId":"`+verifyProjectID+`","key":"`+envPayload["key"].(string)+`","label":"`+envPayload["label"].(string)+`","baseUrl":"`+envPayload["baseUrl"].(string)+`","isDefault":false,"version":"`+envPayload["version"].(string)+`","anthropicModel":"","executionTarget":"cloud_managed","runnerPoolId":""}`)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/ci/runs":
			var err error
			ciPayload, err = decodeJSONMap(r.Body)
			if err != nil {
				t.Fatalf("decode ci body: %v", err)
			}
			_, _ = io.WriteString(w, `{"runId":"run-1","testCaseCount":2,"statusPath":"/api/ci/runs/run-1","cancelPath":"/api/ci/runs/run-1/cancel","statusUrl":"`+apiServerURL(r)+`/api/ci/runs/run-1","appUrl":"https://app.example/runs/run-1","message":"Run created"}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/ci/runs/run-1":
			_, _ = io.WriteString(w, `{"runId":"run-1","state":"completed","conclusion":"success","total":2,"passed":2,"failed":0,"blocked":0,"pending":0,"aborted":0,"retryAfterSeconds":1,"appUrl":"https://app.example/runs/run-1"}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+verifyProjectID+"/process-runs/run-1":
			_, _ = io.WriteString(w, `{
				"id":"run-1",
				"number":11,
				"processId":"proc-smoke",
				"processName":"Smoke",
				"processSlug":"smoke-suite",
				"status":"completed",
				"triggerType":"ci",
				"startedAt":"2026-02-23T08:00:00Z",
				"completedAt":"2026-02-23T08:01:00Z",
				"summaryMarkdown":"done",
				"totalItems":2,
				"passedItems":2,
				"failedItems":0,
				"blockedItems":0,
				"runningItems":0,
				"pendingItems":0,
				"items":[
					{
						"id":"pri-1",
						"testCaseId":"tc-1",
						"testCaseName":"Login page",
						"testCaseNumber":1,
						"executionId":"exe-1",
						"agentSessionId":"session-1",
						"status":"completed",
						"testOutcome":"passed",
						"summaryMarkdown":"ok",
						"startedAt":"2026-02-23T08:00:10Z",
						"completedAt":"2026-02-23T08:00:30Z"
					}
				]
			}`)
			return
		case r.Method == http.MethodDelete && r.URL.Path == "/api/projects/"+verifyProjectID+"/environments/env-1":
			deleteCalled.Store(true)
			w.WriteHeader(http.StatusNoContent)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer apiServer.Close()

	configHome := t.TempDir()
	stdout, _, err := executeRootCommand(t, []string{
		"--json", "verify",
		"--project", "my-project",
		"--url", localApp.URL,
	}, verifyBaseEnv(configHome, apiServer.URL))
	if err != nil {
		t.Fatalf("verify should succeed, got %v", err)
	}

	if ciPayload["processSlug"] != "smoke-suite" {
		t.Fatalf("expected smoke-suite process slug, got %#v", ciPayload["processSlug"])
	}
	if _, hasTags := ciPayload["tags"]; hasTags {
		t.Fatalf("did not expect tags payload, got %#v", ciPayload["tags"])
	}
	if envPayload["baseUrl"] != localApp.URL {
		t.Fatalf("expected baseUrl=%s, got %#v", localApp.URL, envPayload["baseUrl"])
	}
	if !deleteCalled.Load() {
		t.Fatal("expected ephemeral environment deletion")
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("parse verify output: %v\noutput: %s", err, stdout)
	}
	if payload["mode"] != "suite" {
		t.Fatalf("expected mode=suite, got %#v", payload["mode"])
	}
	if payload["schema_version"] != verifySchemaVersion {
		t.Fatalf("expected schema_version=%s, got %#v", verifySchemaVersion, payload["schema_version"])
	}
	if payload["suite"] != "smoke" {
		t.Fatalf("expected suite=smoke, got %#v", payload["suite"])
	}
	if payload["process_slug"] != "smoke-suite" {
		t.Fatalf("expected process_slug=smoke-suite, got %#v", payload["process_slug"])
	}
	if total := int(payload["execution_total"].(float64)); total != 2 {
		t.Fatalf("expected execution_total=2, got %d", total)
	}
	items, ok := payload["executions"].([]any)
	if !ok || len(items) != 1 {
		t.Fatalf("expected one execution summary item, got %#v", payload["executions"])
	}
	execItem, ok := items[0].(map[string]any)
	if !ok {
		t.Fatalf("expected execution item object, got %#v", items[0])
	}
	if execItem["execution_id"] != "exe-1" {
		t.Fatalf("expected execution_id=exe-1, got %#v", execItem["execution_id"])
	}
	if !strings.Contains(execItem["conversation_command"].(string), "executions conversation") {
		t.Fatalf("expected conversation command hint, got %#v", execItem["conversation_command"])
	}
	if payload["run_id"] != "run-1" {
		t.Fatalf("expected run_id=run-1, got %#v", payload["run_id"])
	}
	if deleted, _ := payload["environment_deleted"].(bool); !deleted {
		t.Fatalf("expected environment_deleted=true, got %#v", payload["environment_deleted"])
	}
	if code := int(payload["exit_code"].(float64)); code != ExitOK {
		t.Fatalf("expected exit_code=%d, got %d", ExitOK, code)
	}
	if collected, ok := payload["diagnostics_collected"].(bool); !ok || collected {
		t.Fatalf("expected diagnostics_collected=false, got %#v", payload["diagnostics_collected"])
	}

	configPath := filepath.Join(configHome, "certyn", "config.yaml")
	rawConfig, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("read config: %v", readErr)
	}
	if !strings.Contains(string(rawConfig), "my-project: "+verifyProjectID) {
		t.Fatalf("expected stored project mapping in config, got:\n%s", string(rawConfig))
	}
}

func TestVerifyTagsModeUsesTagsAndIgnoresSuite(t *testing.T) {
	localApp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer localApp.Close()

	var ciPayload map[string]any
	var processListCalled atomic.Bool

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/overview":
			_, _ = io.WriteString(w, `{"defaultProjectId":"`+verifyProjectID+`","projects":[{"id":"`+verifyProjectID+`","slug":"my-project","name":"My Project"}]}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+verifyProjectID+"/processes":
			processListCalled.Store(true)
			_, _ = io.WriteString(w, `[]`)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/projects/"+verifyProjectID+"/environments":
			_, _ = io.WriteString(w, `{"id":"env-1","projectId":"`+verifyProjectID+`","key":"dev-key","label":"label","baseUrl":"`+localApp.URL+`","isDefault":false,"version":"verify-version","anthropicModel":"","executionTarget":"cloud_managed","runnerPoolId":""}`)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/ci/runs":
			var err error
			ciPayload, err = decodeJSONMap(r.Body)
			if err != nil {
				t.Fatalf("decode ci body: %v", err)
			}
			_, _ = io.WriteString(w, `{"runId":"run-2","testCaseCount":2,"statusPath":"/api/ci/runs/run-2","cancelPath":"/api/ci/runs/run-2/cancel","statusUrl":"`+apiServerURL(r)+`/api/ci/runs/run-2","appUrl":"","message":"Run created"}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/ci/runs/run-2":
			_, _ = io.WriteString(w, `{"runId":"run-2","state":"completed","conclusion":"success","total":1,"passed":1,"failed":0,"blocked":0,"pending":0,"aborted":0,"retryAfterSeconds":1}`)
			return
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/environments/env-1"):
			w.WriteHeader(http.StatusNoContent)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer apiServer.Close()

	stdout, _, err := executeRootCommand(t, []string{
		"--json", "verify",
		"--project", "my-project",
		"--url", localApp.URL,
		"--suite", "regression",
		"--tag", "checkout",
		"--tag", "payments",
	}, verifyBaseEnv(t.TempDir(), apiServer.URL))
	if err != nil {
		t.Fatalf("verify tags mode should succeed: %v", err)
	}

	if _, hasProcess := ciPayload["processSlug"]; hasProcess {
		t.Fatalf("expected no processSlug when tags provided, got %#v", ciPayload["processSlug"])
	}
	rawTags, ok := ciPayload["tags"].([]any)
	if !ok {
		t.Fatalf("expected tags array, got %#v", ciPayload["tags"])
	}
	if len(rawTags) != 2 || rawTags[0] != "checkout" || rawTags[1] != "payments" {
		t.Fatalf("unexpected tags payload: %#v", rawTags)
	}
	if processListCalled.Load() {
		t.Fatal("did not expect process list API call in tags mode")
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("parse verify output: %v", err)
	}
	if payload["mode"] != "tags" {
		t.Fatalf("expected mode=tags, got %#v", payload["mode"])
	}
	if _, hasSuite := payload["suite"]; hasSuite {
		t.Fatalf("expected suite omitted in tags mode, got %#v", payload["suite"])
	}
	if _, hasProcessSlug := payload["process_slug"]; hasProcessSlug {
		t.Fatalf("expected process_slug omitted in tags mode, got %#v", payload["process_slug"])
	}
}

func TestVerifyCustomSuiteResolvesFromAPI(t *testing.T) {
	localApp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer localApp.Close()

	var ciPayload map[string]any

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/overview":
			_, _ = io.WriteString(w, `{"defaultProjectId":"`+verifyProjectID+`","projects":[{"id":"`+verifyProjectID+`","slug":"my-project","name":"My Project"}]}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+verifyProjectID+"/processes":
			_, _ = io.WriteString(w, `[
				{
					"id":"proc-custom",
					"projectId":"`+verifyProjectID+`",
					"name":"Checkout",
					"slug":"checkout-suite",
					"isExploratory":false,
					"isActive":true,
					"configuration":{"ticketLabels":[],"testCaseTags":["checkout"]}
				}
			]`)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/projects/"+verifyProjectID+"/environments":
			_, _ = io.WriteString(w, `{"id":"env-1","projectId":"`+verifyProjectID+`","key":"dev-key","label":"label","baseUrl":"`+localApp.URL+`","isDefault":false,"version":"verify-version","anthropicModel":"","executionTarget":"cloud_managed","runnerPoolId":""}`)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/ci/runs":
			var err error
			ciPayload, err = decodeJSONMap(r.Body)
			if err != nil {
				t.Fatalf("decode ci body: %v", err)
			}
			_, _ = io.WriteString(w, `{"runId":"run-custom","testCaseCount":1,"statusPath":"/api/ci/runs/run-custom","cancelPath":"/api/ci/runs/run-custom/cancel","statusUrl":"`+apiServerURL(r)+`/api/ci/runs/run-custom","appUrl":"","message":"Run created"}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/ci/runs/run-custom":
			_, _ = io.WriteString(w, `{"runId":"run-custom","state":"completed","conclusion":"success","total":1,"passed":1,"failed":0,"blocked":0,"pending":0,"aborted":0,"retryAfterSeconds":1}`)
			return
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/environments/env-1"):
			w.WriteHeader(http.StatusNoContent)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer apiServer.Close()

	stdout, _, err := executeRootCommand(t, []string{
		"--json", "verify",
		"--project", "my-project",
		"--url", localApp.URL,
		"--suite", "checkout-suite",
	}, verifyBaseEnv(t.TempDir(), apiServer.URL))
	if err != nil {
		t.Fatalf("verify should succeed for custom suite: %v", err)
	}

	if ciPayload["processSlug"] != "checkout-suite" {
		t.Fatalf("expected processSlug=checkout-suite, got %#v", ciPayload["processSlug"])
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("parse verify output: %v", err)
	}
	if payload["suite"] != "checkout-suite" {
		t.Fatalf("expected suite=checkout-suite, got %#v", payload["suite"])
	}
	if payload["process_slug"] != "checkout-suite" {
		t.Fatalf("expected process_slug=checkout-suite, got %#v", payload["process_slug"])
	}
}

func TestVerifySuiteModeFallsBackWhenProcessDiscoveryUnauthorized(t *testing.T) {
	var ciPayload map[string]any

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/overview":
			_, _ = io.WriteString(w, `{"defaultProjectId":"`+verifyProjectID+`","projects":[{"id":"`+verifyProjectID+`","slug":"my-project","name":"My Project"}]}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+verifyProjectID+"/processes":
			w.WriteHeader(http.StatusUnauthorized)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+verifyProjectID+"/environments":
			_, _ = io.WriteString(w, `{"items":[{"id":"env-1","projectId":"`+verifyProjectID+`","key":"staging","label":"Staging","baseUrl":"https://staging.example.com","isDefault":false,"version":"","anthropicModel":"","executionTarget":"cloud_managed","runnerPoolId":""}],"totalCount":1,"page":1,"pageSize":100,"totalPages":1,"hasNextPage":false,"hasPreviousPage":false}`)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/ci/runs":
			var err error
			ciPayload, err = decodeJSONMap(r.Body)
			if err != nil {
				t.Fatalf("decode ci body: %v", err)
			}
			_, _ = io.WriteString(w, `{"runId":"run-fallback","testCaseCount":1,"statusPath":"/api/ci/runs/run-fallback","cancelPath":"/api/ci/runs/run-fallback/cancel","statusUrl":"`+apiServerURL(r)+`/api/ci/runs/run-fallback","appUrl":"","message":"Run created"}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/ci/runs/run-fallback":
			_, _ = io.WriteString(w, `{"runId":"run-fallback","state":"completed","conclusion":"success","total":1,"passed":1,"failed":0,"blocked":0,"pending":0,"aborted":0,"retryAfterSeconds":1}`)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer apiServer.Close()

	stdout, _, err := executeRootCommand(t, []string{
		"--json",
		"--environment", "staging",
		"verify",
		"--project", "my-project",
		"--suite", "smoke",
	}, verifyBaseEnv(t.TempDir(), apiServer.URL))
	if err != nil {
		t.Fatalf("verify should succeed when process discovery is unauthorized: %v", err)
	}

	if ciPayload["processSlug"] != "smoke-suite" {
		t.Fatalf("expected processSlug smoke-suite fallback, got %#v", ciPayload["processSlug"])
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("parse verify output: %v", err)
	}
	if payload["process_slug"] != "smoke-suite" {
		t.Fatalf("expected process_slug smoke-suite, got %#v", payload["process_slug"])
	}
}

func TestVerifyExistingEnvironmentModeUsesProvidedEnvAndSkipsEnvLifecycle(t *testing.T) {
	var ciPayload map[string]any
	var createEnvCalled atomic.Bool
	var deleteEnvCalled atomic.Bool

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/overview":
			_, _ = io.WriteString(w, `{"defaultProjectId":"`+verifyProjectID+`","projects":[{"id":"`+verifyProjectID+`","slug":"my-project","name":"My Project"}]}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+verifyProjectID+"/processes":
			_, _ = io.WriteString(w, `[
				{
					"id":"proc-smoke",
					"projectId":"`+verifyProjectID+`",
					"name":"Smoke",
					"slug":"smoke-suite",
					"isExploratory":false,
					"isActive":true,
					"configuration":{"ticketLabels":[],"testCaseTags":["smoke"]}
				}
			]`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+verifyProjectID+"/environments":
			_, _ = io.WriteString(w, `{"items":[{"id":"env-1","projectId":"`+verifyProjectID+`","key":"staging","label":"Staging","baseUrl":"https://staging.example.com","isDefault":true,"version":"2026.02.24","anthropicModel":"","executionTarget":"cloud_managed","runnerPoolId":""}],"totalCount":1,"page":1,"pageSize":100,"totalPages":1,"hasNextPage":false,"hasPreviousPage":false}`)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/projects/"+verifyProjectID+"/environments":
			createEnvCalled.Store(true)
			http.Error(w, "unexpected create", http.StatusInternalServerError)
			return
		case r.Method == http.MethodDelete && strings.Contains(r.URL.Path, "/api/projects/"+verifyProjectID+"/environments/"):
			deleteEnvCalled.Store(true)
			http.Error(w, "unexpected delete", http.StatusInternalServerError)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/ci/runs":
			var err error
			ciPayload, err = decodeJSONMap(r.Body)
			if err != nil {
				t.Fatalf("decode ci body: %v", err)
			}
			_, _ = io.WriteString(w, `{"runId":"run-existing","testCaseCount":1,"statusPath":"/api/ci/runs/run-existing","cancelPath":"/api/ci/runs/run-existing/cancel","statusUrl":"`+apiServerURL(r)+`/api/ci/runs/run-existing","appUrl":"","message":"Run created"}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/ci/runs/run-existing":
			_, _ = io.WriteString(w, `{"runId":"run-existing","state":"completed","conclusion":"success","total":1,"passed":1,"failed":0,"blocked":0,"pending":0,"aborted":0,"retryAfterSeconds":1}`)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer apiServer.Close()

	stdout, _, err := executeRootCommand(t, []string{
		"--json",
		"--environment", "staging",
		"verify",
		"--project", "my-project",
	}, verifyBaseEnv(t.TempDir(), apiServer.URL))
	if err != nil {
		t.Fatalf("verify existing environment mode should succeed: %v", err)
	}

	if createEnvCalled.Load() {
		t.Fatal("expected no environment creation in existing mode")
	}
	if deleteEnvCalled.Load() {
		t.Fatal("expected no environment deletion in existing mode")
	}
	if ciPayload["environmentKey"] != "staging" {
		t.Fatalf("expected CI environmentKey=staging, got %#v", ciPayload["environmentKey"])
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("parse verify output: %v", err)
	}
	if payload["environment_mode"] != "existing" {
		t.Fatalf("expected environment_mode=existing, got %#v", payload["environment_mode"])
	}
	if payload["environment_key"] != "staging" {
		t.Fatalf("expected environment_key=staging, got %#v", payload["environment_key"])
	}
	if deleted, _ := payload["environment_deleted"].(bool); deleted {
		t.Fatalf("expected environment_deleted=false in existing mode, got %#v", payload["environment_deleted"])
	}
}

func TestVerifyRequiresTargetURLOrEnvironment(t *testing.T) {
	_, _, err := executeRootCommand(t, []string{"verify", "--project", "my-project"}, map[string]string{
		"CERTYN_API_URL":  "https://api.example.com",
		"CERTYN_API_KEY":  "test-key",
		"XDG_CONFIG_HOME": t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected missing target error")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitUsage {
		t.Fatalf("expected usage exit code %d, got %d", ExitUsage, cmdErr.Code)
	}
	if !strings.Contains(cmdErr.Error(), "provide --url") || !strings.Contains(cmdErr.Error(), "--environment") {
		t.Fatalf("expected target hint, got %q", cmdErr.Error())
	}
}

func TestVerifyRejectsConflictingURLAndEnvironment(t *testing.T) {
	localApp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer localApp.Close()

	_, _, err := executeRootCommand(t, []string{
		"verify",
		"--project", "my-project",
		"--url", localApp.URL,
		"--environment", "staging",
	}, map[string]string{
		"CERTYN_API_URL":  "https://api.example.com",
		"CERTYN_API_KEY":  "test-key",
		"XDG_CONFIG_HOME": t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected conflicting targets error")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitUsage {
		t.Fatalf("expected usage exit code %d, got %d", ExitUsage, cmdErr.Code)
	}
	if !strings.Contains(cmdErr.Error(), "either --url or --environment") {
		t.Fatalf("unexpected error message: %q", cmdErr.Error())
	}
}

func TestVerifyURLOverridesImplicitEnvironmentDefaults(t *testing.T) {
	localApp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer localApp.Close()

	var createEnvCalled atomic.Bool
	var listEnvCalled atomic.Bool

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/overview":
			_, _ = io.WriteString(w, `{"defaultProjectId":"`+verifyProjectID+`","projects":[{"id":"`+verifyProjectID+`","slug":"my-project","name":"My Project"}]}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+verifyProjectID+"/processes":
			_, _ = io.WriteString(w, `[
				{
					"id":"proc-smoke",
					"projectId":"`+verifyProjectID+`",
					"name":"Smoke",
					"slug":"smoke-suite",
					"isExploratory":false,
					"isActive":true,
					"configuration":{"ticketLabels":[],"testCaseTags":["smoke"]}
				}
			]`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+verifyProjectID+"/environments":
			listEnvCalled.Store(true)
			http.Error(w, "unexpected list environments call", http.StatusInternalServerError)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/projects/"+verifyProjectID+"/environments":
			createEnvCalled.Store(true)
			_, _ = io.WriteString(w, `{"id":"env-override","projectId":"`+verifyProjectID+`","key":"dev-key","label":"label","baseUrl":"`+localApp.URL+`","isDefault":false,"version":"verify-version","anthropicModel":"","executionTarget":"cloud_managed","runnerPoolId":""}`)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/ci/runs":
			_, _ = io.WriteString(w, `{"runId":"run-override","testCaseCount":1,"statusPath":"/api/ci/runs/run-override","cancelPath":"/api/ci/runs/run-override/cancel","statusUrl":"`+apiServerURL(r)+`/api/ci/runs/run-override","appUrl":"","message":"Run created"}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/ci/runs/run-override":
			_, _ = io.WriteString(w, `{"runId":"run-override","state":"completed","conclusion":"success","total":1,"passed":1,"failed":0,"blocked":0,"pending":0,"aborted":0,"retryAfterSeconds":1}`)
			return
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/environments/env-override"):
			w.WriteHeader(http.StatusNoContent)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer apiServer.Close()

	stdout, _, err := executeRootCommand(t, []string{
		"--json", "verify",
		"--project", "my-project",
		"--url", localApp.URL,
	}, map[string]string{
		"CERTYN_API_URL":     apiServer.URL,
		"CERTYN_API_KEY":     "test-key",
		"CERTYN_ENVIRONMENT": "staging",
		"XDG_CONFIG_HOME":    t.TempDir(),
	})
	if err != nil {
		t.Fatalf("verify should succeed when URL is set and environment is implicit: %v", err)
	}

	if listEnvCalled.Load() {
		t.Fatal("expected URL mode to avoid existing environment resolution")
	}
	if !createEnvCalled.Load() {
		t.Fatal("expected URL mode to create ephemeral environment")
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("parse verify output: %v", err)
	}
	if payload["environment_mode"] != "ephemeral" {
		t.Fatalf("expected environment_mode=ephemeral, got %#v", payload["environment_mode"])
	}
}

func TestVerifyRejectsInvalidURL(t *testing.T) {
	_, _, err := executeRootCommand(t, []string{
		"verify",
		"--project", "my-project",
		"--url", "ftp://localhost",
	}, map[string]string{
		"CERTYN_API_URL":  "https://api.example.com",
		"CERTYN_API_KEY":  "test-key",
		"XDG_CONFIG_HOME": t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected invalid url error")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitUsage {
		t.Fatalf("expected usage exit code %d, got %d", ExitUsage, cmdErr.Code)
	}
}

func TestVerifyFailsWhenURLIsUnreachable(t *testing.T) {
	_, _, err := executeRootCommand(t, []string{
		"verify",
		"--project", "my-project",
		"--url", "http://127.0.0.1:1",
	}, map[string]string{
		"CERTYN_API_URL":  "https://api.example.com",
		"CERTYN_API_KEY":  "test-key",
		"XDG_CONFIG_HOME": t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected unreachable url error")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitUsage {
		t.Fatalf("expected usage exit code %d, got %d", ExitUsage, cmdErr.Code)
	}
	if !strings.Contains(cmdErr.Error(), "url is not reachable") {
		t.Fatalf("unexpected error message: %q", cmdErr.Error())
	}
}

func TestVerifyRejectsInvalidDiagnosticsMaxEvents(t *testing.T) {
	localApp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer localApp.Close()

	_, _, err := executeRootCommand(t, []string{
		"verify",
		"--project", "my-project",
		"--url", localApp.URL,
		"--diagnostics-max-events", "0",
	}, map[string]string{
		"CERTYN_API_URL":  "https://api.example.com",
		"CERTYN_API_KEY":  "test-key",
		"XDG_CONFIG_HOME": t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected invalid diagnostics max events error")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitUsage {
		t.Fatalf("expected usage exit code %d, got %d", ExitUsage, cmdErr.Code)
	}
	if !strings.Contains(cmdErr.Error(), "--diagnostics-max-events") {
		t.Fatalf("unexpected error message: %q", cmdErr.Error())
	}
}

func TestVerifyRejectsInvalidDiagnosticsSampleSize(t *testing.T) {
	localApp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer localApp.Close()

	_, _, err := executeRootCommand(t, []string{
		"verify",
		"--project", "my-project",
		"--url", localApp.URL,
		"--diagnostics-sample-size", "-1",
	}, map[string]string{
		"CERTYN_API_URL":  "https://api.example.com",
		"CERTYN_API_KEY":  "test-key",
		"XDG_CONFIG_HOME": t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected invalid diagnostics sample size error")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitUsage {
		t.Fatalf("expected usage exit code %d, got %d", ExitUsage, cmdErr.Code)
	}
	if !strings.Contains(cmdErr.Error(), "--diagnostics-sample-size") {
		t.Fatalf("unexpected error message: %q", cmdErr.Error())
	}
}

func TestVerifyMissingProjectFails(t *testing.T) {
	localApp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer localApp.Close()

	_, _, err := executeRootCommand(t, []string{
		"verify",
		"--url", localApp.URL,
	}, map[string]string{
		"CERTYN_API_URL":  "https://api.example.com",
		"CERTYN_API_KEY":  "test-key",
		"XDG_CONFIG_HOME": t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected missing project error")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitUsage {
		t.Fatalf("expected usage exit code %d, got %d", ExitUsage, cmdErr.Code)
	}
}

func TestVerifyFailsHardOnEnvironmentPermissionError(t *testing.T) {
	localApp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer localApp.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/overview":
			_, _ = io.WriteString(w, `{"defaultProjectId":"`+verifyProjectID+`","projects":[{"id":"`+verifyProjectID+`","slug":"my-project","name":"My Project"}]}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+verifyProjectID+"/processes":
			_, _ = io.WriteString(w, `[
				{
					"id":"proc-smoke",
					"projectId":"`+verifyProjectID+`",
					"name":"Smoke",
					"slug":"smoke-suite",
					"isExploratory":false,
					"isActive":true,
					"configuration":{"ticketLabels":[],"testCaseTags":["smoke"]}
				}
			]`)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/projects/"+verifyProjectID+"/environments":
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"error":"forbidden"}`)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer apiServer.Close()

	_, _, err := executeRootCommand(t, []string{
		"verify",
		"--project", "my-project",
		"--url", localApp.URL,
	}, verifyBaseEnv(t.TempDir(), apiServer.URL))
	if err == nil {
		t.Fatal("expected permission error")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitAuth {
		t.Fatalf("expected auth exit code %d, got %d", ExitAuth, cmdErr.Code)
	}
	if !strings.Contains(cmdErr.Error(), "missing permission for environment management") {
		t.Fatalf("unexpected error message: %q", cmdErr.Error())
	}
}

func TestVerifyKeepEnvSkipsDeletion(t *testing.T) {
	localApp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer localApp.Close()

	var deleteCalled atomic.Bool

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/overview":
			_, _ = io.WriteString(w, `{"defaultProjectId":"`+verifyProjectID+`","projects":[{"id":"`+verifyProjectID+`","slug":"my-project","name":"My Project"}]}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+verifyProjectID+"/processes":
			_, _ = io.WriteString(w, `[
				{
					"id":"proc-smoke",
					"projectId":"`+verifyProjectID+`",
					"name":"Smoke",
					"slug":"smoke-suite",
					"isExploratory":false,
					"isActive":true,
					"configuration":{"ticketLabels":[],"testCaseTags":["smoke"]}
				}
			]`)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/projects/"+verifyProjectID+"/environments":
			_, _ = io.WriteString(w, `{"id":"env-1","projectId":"`+verifyProjectID+`","key":"dev-key","label":"label","baseUrl":"`+localApp.URL+`","isDefault":false,"version":"verify-version","anthropicModel":"","executionTarget":"cloud_managed","runnerPoolId":""}`)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/ci/runs":
			_, _ = io.WriteString(w, `{"runId":"run-3","testCaseCount":1,"statusPath":"/api/ci/runs/run-3","cancelPath":"/api/ci/runs/run-3/cancel","statusUrl":"`+apiServerURL(r)+`/api/ci/runs/run-3","appUrl":"","message":"Run created"}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/ci/runs/run-3":
			_, _ = io.WriteString(w, `{"runId":"run-3","state":"completed","conclusion":"success","total":1,"passed":1,"failed":0,"blocked":0,"pending":0,"aborted":0,"retryAfterSeconds":1}`)
			return
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/environments/env-1"):
			deleteCalled.Store(true)
			w.WriteHeader(http.StatusNoContent)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer apiServer.Close()

	stdout, _, err := executeRootCommand(t, []string{
		"--json", "verify",
		"--project", "my-project",
		"--url", localApp.URL,
		"--keep-env",
	}, verifyBaseEnv(t.TempDir(), apiServer.URL))
	if err != nil {
		t.Fatalf("verify should succeed: %v", err)
	}
	if deleteCalled.Load() {
		t.Fatal("expected no environment deletion when --keep-env is used")
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("parse verify output: %v", err)
	}
	if deleted, _ := payload["environment_deleted"].(bool); deleted {
		t.Fatalf("expected environment_deleted=false, got %#v", payload["environment_deleted"])
	}
}

func TestVerifyDeletesEnvironmentOnGateFailure(t *testing.T) {
	localApp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer localApp.Close()

	var deleteCalled atomic.Bool

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/overview":
			_, _ = io.WriteString(w, `{"defaultProjectId":"`+verifyProjectID+`","projects":[{"id":"`+verifyProjectID+`","slug":"my-project","name":"My Project"}]}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+verifyProjectID+"/processes":
			_, _ = io.WriteString(w, `[
				{
					"id":"proc-smoke",
					"projectId":"`+verifyProjectID+`",
					"name":"Smoke",
					"slug":"smoke-suite",
					"isExploratory":false,
					"isActive":true,
					"configuration":{"ticketLabels":[],"testCaseTags":["smoke"]}
				}
			]`)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/projects/"+verifyProjectID+"/environments":
			_, _ = io.WriteString(w, `{"id":"env-1","projectId":"`+verifyProjectID+`","key":"dev-key","label":"label","baseUrl":"`+localApp.URL+`","isDefault":false,"version":"verify-version","anthropicModel":"","executionTarget":"cloud_managed","runnerPoolId":""}`)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/ci/runs":
			_, _ = io.WriteString(w, `{"runId":"run-4","testCaseCount":1,"statusPath":"/api/ci/runs/run-4","cancelPath":"/api/ci/runs/run-4/cancel","statusUrl":"`+apiServerURL(r)+`/api/ci/runs/run-4","appUrl":"","message":"Run created"}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/ci/runs/run-4":
			_, _ = io.WriteString(w, `{"runId":"run-4","state":"completed","conclusion":"failed","total":1,"passed":0,"failed":1,"blocked":0,"pending":0,"aborted":0,"retryAfterSeconds":1}`)
			return
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/environments/env-1"):
			deleteCalled.Store(true)
			w.WriteHeader(http.StatusNoContent)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer apiServer.Close()

	stdout, _, err := executeRootCommand(t, []string{
		"--json", "verify",
		"--project", "my-project",
		"--url", localApp.URL,
	}, verifyBaseEnv(t.TempDir(), apiServer.URL))
	if err == nil {
		t.Fatal("expected quality gate failure")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitGateFailed {
		t.Fatalf("expected gate failed exit code %d, got %d", ExitGateFailed, cmdErr.Code)
	}
	if !deleteCalled.Load() {
		t.Fatal("expected cleanup deletion on gate failure")
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("parse verify output: %v\noutput: %s", err, stdout)
	}
	if code := int(payload["exit_code"].(float64)); code != ExitGateFailed {
		t.Fatalf("expected exit_code=%d, got %d", ExitGateFailed, code)
	}
	if deleted, _ := payload["environment_deleted"].(bool); !deleted {
		t.Fatalf("expected environment_deleted=true, got %#v", payload["environment_deleted"])
	}
}

func TestVerifyGateFailureCollectsDiagnosticsForFailedExecutionsOnly(t *testing.T) {
	localApp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer localApp.Close()

	var passedConversationCalled atomic.Bool

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/overview":
			_, _ = io.WriteString(w, `{"defaultProjectId":"`+verifyProjectID+`","projects":[{"id":"`+verifyProjectID+`","slug":"my-project","name":"My Project"}]}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+verifyProjectID+"/processes":
			_, _ = io.WriteString(w, `[
				{
					"id":"proc-smoke",
					"projectId":"`+verifyProjectID+`",
					"name":"Smoke",
					"slug":"smoke-suite",
					"isExploratory":false,
					"isActive":true,
					"configuration":{"ticketLabels":[],"testCaseTags":["smoke"]}
				}
			]`)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/projects/"+verifyProjectID+"/environments":
			_, _ = io.WriteString(w, `{"id":"env-1","projectId":"`+verifyProjectID+`","key":"dev-key","label":"label","baseUrl":"`+localApp.URL+`","isDefault":false,"version":"verify-version","anthropicModel":"","executionTarget":"cloud_managed","runnerPoolId":""}`)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/ci/runs":
			_, _ = io.WriteString(w, `{"runId":"run-5","testCaseCount":2,"statusPath":"/api/ci/runs/run-5","cancelPath":"/api/ci/runs/run-5/cancel","statusUrl":"`+apiServerURL(r)+`/api/ci/runs/run-5","appUrl":"","message":"Run created"}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/ci/runs/run-5":
			_, _ = io.WriteString(w, `{"runId":"run-5","state":"completed","conclusion":"failed","total":2,"passed":1,"failed":1,"blocked":0,"pending":0,"aborted":0,"retryAfterSeconds":1}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+verifyProjectID+"/process-runs/run-5":
			_, _ = io.WriteString(w, `{
				"id":"run-5",
				"number":15,
				"processId":"proc-smoke",
				"processName":"Smoke",
				"processSlug":"smoke-suite",
				"status":"completed",
				"triggerType":"ci",
				"totalItems":2,
				"passedItems":1,
				"failedItems":1,
				"blockedItems":0,
				"runningItems":0,
				"pendingItems":0,
				"items":[
					{
						"id":"pri-failed",
						"testCaseId":"tc-failed",
						"testCaseName":"Failed case",
						"testCaseNumber":11,
						"executionId":"exe-failed",
						"agentSessionId":"session-failed",
						"status":"completed",
						"testOutcome":"failed"
					},
					{
						"id":"pri-passed",
						"testCaseId":"tc-passed",
						"testCaseName":"Passed case",
						"testCaseNumber":12,
						"executionId":"exe-passed",
						"agentSessionId":"session-passed",
						"status":"completed",
						"testOutcome":"passed"
					}
				]
			}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+verifyProjectID+"/agentsessions/session-failed/conversation":
			_, _ = io.WriteString(w, `{
				"items":[
					{"type":"tool_use_requested","timestampUtc":"2026-02-23T08:01:00Z","payload":{"toolName":"computer","toolCallId":"call-1"}},
					{"type":"policy_decision","timestampUtc":"2026-02-23T08:01:01Z","payload":{"allowed":false,"toolName":"computer","toolCallId":"call-1"}},
					{"type":"tool_result","timestampUtc":"2026-02-23T08:01:02Z","payload":{"status":"failed","output":"click failed","metadata":{"toolName":"computer","toolCallId":"call-1"}}},
					{"type":"browser_network_request","timestampUtc":"2026-02-23T08:01:03Z","payload":{"requestId":"req-1","method":"POST","url":"https://api.example.com/do?token=ABC123"}},
					{"type":"browser_network_response","timestampUtc":"2026-02-23T08:01:04Z","payload":{"requestId":"req-1","status":500,"statusText":"Server Error","url":"https://api.example.com/do?token=ABC123"}},
					{"type":"conversation_completed","timestampUtc":"2026-02-23T08:01:05Z","payload":{"status":"failed"}}
				],
				"totalCount":6,
				"page":1,
				"pageSize":500,
				"totalPages":1,
				"hasNextPage":false,
				"hasPreviousPage":false
			}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+verifyProjectID+"/agentsessions/session-passed/conversation":
			passedConversationCalled.Store(true)
			http.Error(w, "should not be called for passed executions", http.StatusInternalServerError)
			return
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/environments/env-1"):
			w.WriteHeader(http.StatusNoContent)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer apiServer.Close()

	stdout, _, err := executeRootCommand(t, []string{
		"--json", "verify",
		"--project", "my-project",
		"--url", localApp.URL,
	}, verifyBaseEnv(t.TempDir(), apiServer.URL))
	if err == nil {
		t.Fatal("expected quality gate failure")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitGateFailed {
		t.Fatalf("expected gate failed exit code %d, got %d", ExitGateFailed, cmdErr.Code)
	}
	if passedConversationCalled.Load() {
		t.Fatal("expected diagnostics to skip passed executions")
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("parse verify output: %v\noutput: %s", err, stdout)
	}
	if collected, ok := payload["diagnostics_collected"].(bool); !ok || !collected {
		t.Fatalf("expected diagnostics_collected=true, got %#v", payload["diagnostics_collected"])
	}

	rawDiagnostics, ok := payload["diagnostics"].([]any)
	if !ok || len(rawDiagnostics) != 1 {
		t.Fatalf("expected one diagnostic entry, got %#v", payload["diagnostics"])
	}
	diagnostic, ok := rawDiagnostics[0].(map[string]any)
	if !ok {
		t.Fatalf("expected diagnostic object, got %#v", rawDiagnostics[0])
	}
	if diagnostic["execution_id"] != "exe-failed" {
		t.Fatalf("expected execution_id=exe-failed, got %#v", diagnostic["execution_id"])
	}
	if diagnostic["primary_failure_reason"] != "conversation_failed" {
		t.Fatalf("expected primary_failure_reason=conversation_failed, got %#v", diagnostic["primary_failure_reason"])
	}
	counts, ok := diagnostic["counts"].(map[string]any)
	if !ok {
		t.Fatalf("expected counts object, got %#v", diagnostic["counts"])
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
	rawNetworkFailures, ok := diagnostic["network_failures"].([]any)
	if !ok || len(rawNetworkFailures) != 1 {
		t.Fatalf("expected one network failure sample, got %#v", diagnostic["network_failures"])
	}
	firstNetworkFailure, ok := rawNetworkFailures[0].(map[string]any)
	if !ok {
		t.Fatalf("expected network failure object, got %#v", rawNetworkFailures[0])
	}
	if !strings.Contains(firstNetworkFailure["url"].(string), "token=ABC123") {
		t.Fatalf("expected unredacted url, got %#v", firstNetworkFailure["url"])
	}
	if !strings.Contains(diagnostic["diagnose_command"].(string), "executions diagnose") {
		t.Fatalf("expected diagnose command hint, got %#v", diagnostic["diagnose_command"])
	}
	if !strings.Contains(diagnostic["conversation_command"].(string), "executions conversation") {
		t.Fatalf("expected conversation command hint, got %#v", diagnostic["conversation_command"])
	}
}

func TestVerifyGateFailureCanSkipDiagnosticsCollection(t *testing.T) {
	localApp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer localApp.Close()

	var conversationCalled atomic.Bool

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/overview":
			_, _ = io.WriteString(w, `{"defaultProjectId":"`+verifyProjectID+`","projects":[{"id":"`+verifyProjectID+`","slug":"my-project","name":"My Project"}]}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+verifyProjectID+"/processes":
			_, _ = io.WriteString(w, `[{"id":"proc-smoke","projectId":"`+verifyProjectID+`","name":"Smoke","slug":"smoke-suite","isExploratory":false,"isActive":true,"configuration":{"ticketLabels":[],"testCaseTags":["smoke"]}}]`)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/projects/"+verifyProjectID+"/environments":
			_, _ = io.WriteString(w, `{"id":"env-1","projectId":"`+verifyProjectID+`","key":"dev-key","label":"label","baseUrl":"`+localApp.URL+`","isDefault":false,"version":"verify-version","anthropicModel":"","executionTarget":"cloud_managed","runnerPoolId":""}`)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/ci/runs":
			_, _ = io.WriteString(w, `{"runId":"run-6","testCaseCount":1,"statusPath":"/api/ci/runs/run-6","cancelPath":"/api/ci/runs/run-6/cancel","statusUrl":"`+apiServerURL(r)+`/api/ci/runs/run-6","appUrl":"","message":"Run created"}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/ci/runs/run-6":
			_, _ = io.WriteString(w, `{"runId":"run-6","state":"completed","conclusion":"failed","total":1,"passed":0,"failed":1,"blocked":0,"pending":0,"aborted":0,"retryAfterSeconds":1}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+verifyProjectID+"/process-runs/run-6":
			_, _ = io.WriteString(w, `{
				"id":"run-6",
				"status":"completed",
				"totalItems":1,
				"passedItems":0,
				"failedItems":1,
				"blockedItems":0,
				"runningItems":0,
				"pendingItems":0,
				"items":[{"id":"pri-1","executionId":"exe-1","agentSessionId":"session-1","status":"completed","testOutcome":"failed"}]
			}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+verifyProjectID+"/agentsessions/session-1/conversation":
			conversationCalled.Store(true)
			http.Error(w, "should not be called when diagnostics are disabled", http.StatusInternalServerError)
			return
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/environments/env-1"):
			w.WriteHeader(http.StatusNoContent)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer apiServer.Close()

	stdout, _, err := executeRootCommand(t, []string{
		"--json", "verify",
		"--project", "my-project",
		"--url", localApp.URL,
		"--diagnose-failed=false",
	}, verifyBaseEnv(t.TempDir(), apiServer.URL))
	if err == nil {
		t.Fatal("expected gate failure")
	}
	if conversationCalled.Load() {
		t.Fatal("expected no diagnostics conversation fetch when --diagnose-failed=false")
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("parse verify output: %v", err)
	}
	if collected, ok := payload["diagnostics_collected"].(bool); !ok || collected {
		t.Fatalf("expected diagnostics_collected=false, got %#v", payload["diagnostics_collected"])
	}
	if _, exists := payload["diagnostics"]; exists {
		t.Fatalf("expected diagnostics to be omitted, got %#v", payload["diagnostics"])
	}
}

func TestVerifyDiagnosticsErrorsDoNotOverrideGateFailure(t *testing.T) {
	localApp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer localApp.Close()

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/overview":
			_, _ = io.WriteString(w, `{"defaultProjectId":"`+verifyProjectID+`","projects":[{"id":"`+verifyProjectID+`","slug":"my-project","name":"My Project"}]}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+verifyProjectID+"/processes":
			_, _ = io.WriteString(w, `[{"id":"proc-smoke","projectId":"`+verifyProjectID+`","name":"Smoke","slug":"smoke-suite","isExploratory":false,"isActive":true,"configuration":{"ticketLabels":[],"testCaseTags":["smoke"]}}]`)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/projects/"+verifyProjectID+"/environments":
			_, _ = io.WriteString(w, `{"id":"env-1","projectId":"`+verifyProjectID+`","key":"dev-key","label":"label","baseUrl":"`+localApp.URL+`","isDefault":false,"version":"verify-version","anthropicModel":"","executionTarget":"cloud_managed","runnerPoolId":""}`)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/ci/runs":
			_, _ = io.WriteString(w, `{"runId":"run-7","testCaseCount":1,"statusPath":"/api/ci/runs/run-7","cancelPath":"/api/ci/runs/run-7/cancel","statusUrl":"`+apiServerURL(r)+`/api/ci/runs/run-7","appUrl":"","message":"Run created"}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/ci/runs/run-7":
			_, _ = io.WriteString(w, `{"runId":"run-7","state":"completed","conclusion":"failed","total":1,"passed":0,"failed":1,"blocked":0,"pending":0,"aborted":0,"retryAfterSeconds":1}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+verifyProjectID+"/process-runs/run-7":
			_, _ = io.WriteString(w, `{
				"id":"run-7",
				"status":"completed",
				"totalItems":1,
				"passedItems":0,
				"failedItems":1,
				"blockedItems":0,
				"runningItems":0,
				"pendingItems":0,
				"items":[{"id":"pri-1","executionId":"exe-1","agentSessionId":"session-1","status":"completed","testOutcome":"failed"}]
			}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+verifyProjectID+"/agentsessions/session-1/conversation":
			http.Error(w, "conversation unavailable", http.StatusInternalServerError)
			return
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/environments/env-1"):
			w.WriteHeader(http.StatusNoContent)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer apiServer.Close()

	stdout, _, err := executeRootCommand(t, []string{
		"--json", "verify",
		"--project", "my-project",
		"--url", localApp.URL,
	}, verifyBaseEnv(t.TempDir(), apiServer.URL))
	if err == nil {
		t.Fatal("expected gate failure")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitGateFailed {
		t.Fatalf("expected gate failed exit code %d, got %d", ExitGateFailed, cmdErr.Code)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(stdout), &payload); err != nil {
		t.Fatalf("parse verify output: %v", err)
	}
	if code := int(payload["exit_code"].(float64)); code != ExitGateFailed {
		t.Fatalf("expected exit_code=%d, got %d", ExitGateFailed, code)
	}
	if collected, ok := payload["diagnostics_collected"].(bool); !ok || !collected {
		t.Fatalf("expected diagnostics_collected=true, got %#v", payload["diagnostics_collected"])
	}
	if rawDiagnostics, ok := payload["diagnostics"].([]any); ok && len(rawDiagnostics) > 0 {
		t.Fatalf("expected no successful diagnostics, got %#v", rawDiagnostics)
	}
	rawErrors, ok := payload["diagnostics_errors"].([]any)
	if !ok || len(rawErrors) != 1 {
		t.Fatalf("expected one diagnostics error, got %#v", payload["diagnostics_errors"])
	}
	errorEntry, ok := rawErrors[0].(map[string]any)
	if !ok {
		t.Fatalf("expected diagnostics error object, got %#v", rawErrors[0])
	}
	if errorEntry["execution_id"] != "exe-1" {
		t.Fatalf("expected execution_id=exe-1, got %#v", errorEntry["execution_id"])
	}
	if !strings.Contains(errorEntry["error"].(string), "failed to list execution conversation") {
		t.Fatalf("unexpected diagnostics error message: %#v", errorEntry["error"])
	}
}

func TestVerifyDeletesEnvironmentOnTimeout(t *testing.T) {
	localApp := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer localApp.Close()

	var deleteCalled atomic.Bool

	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/overview":
			_, _ = io.WriteString(w, `{"defaultProjectId":"`+verifyProjectID+`","projects":[{"id":"`+verifyProjectID+`","slug":"my-project","name":"My Project"}]}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/projects/"+verifyProjectID+"/processes":
			_, _ = io.WriteString(w, `[
				{
					"id":"proc-smoke",
					"projectId":"`+verifyProjectID+`",
					"name":"Smoke",
					"slug":"smoke-suite",
					"isExploratory":false,
					"isActive":true,
					"configuration":{"ticketLabels":[],"testCaseTags":["smoke"]}
				}
			]`)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/projects/"+verifyProjectID+"/environments":
			_, _ = io.WriteString(w, `{"id":"env-1","projectId":"`+verifyProjectID+`","key":"dev-key","label":"label","baseUrl":"`+localApp.URL+`","isDefault":false,"version":"verify-version","anthropicModel":"","executionTarget":"cloud_managed","runnerPoolId":""}`)
			return
		case r.Method == http.MethodPost && r.URL.Path == "/api/ci/runs":
			_, _ = io.WriteString(w, `{"runId":"run-timeout","testCaseCount":1,"statusPath":"/api/ci/runs/run-timeout","cancelPath":"/api/ci/runs/run-timeout/cancel","statusUrl":"`+apiServerURL(r)+`/api/ci/runs/run-timeout","appUrl":"","message":"Run created"}`)
			return
		case r.Method == http.MethodGet && r.URL.Path == "/api/ci/runs/run-timeout":
			_, _ = io.WriteString(w, `{"runId":"run-timeout","state":"running","conclusion":"","total":0,"passed":0,"failed":0,"blocked":0,"pending":0,"aborted":0,"retryAfterSeconds":60}`)
			return
		case r.Method == http.MethodDelete && strings.HasSuffix(r.URL.Path, "/environments/env-1"):
			deleteCalled.Store(true)
			w.WriteHeader(http.StatusNoContent)
			return
		default:
			http.NotFound(w, r)
			return
		}
	}))
	defer apiServer.Close()

	_, _, err := executeRootCommand(t, []string{
		"verify",
		"--project", "my-project",
		"--url", localApp.URL,
		"--timeout", "50ms",
	}, verifyBaseEnv(t.TempDir(), apiServer.URL))
	if err == nil {
		t.Fatal("expected timeout")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitTimeout {
		t.Fatalf("expected timeout exit code %d, got %d", ExitTimeout, cmdErr.Code)
	}
	if !deleteCalled.Load() {
		t.Fatal("expected cleanup deletion on timeout")
	}
}

func verifyBaseEnv(configHome, apiURL string) map[string]string {
	return map[string]string{
		"CERTYN_API_URL":  apiURL,
		"CERTYN_API_KEY":  "test-key",
		"XDG_CONFIG_HOME": configHome,
	}
}

func apiServerURL(r *http.Request) string {
	return "http://" + r.Host
}

func TestNormalizeVerifySuiteInput(t *testing.T) {
	suite, processID := normalizeVerifySuiteInput("smoke")
	if suite != "smoke" || processID != "smoke-suite" {
		t.Fatalf("expected smoke/smoke-suite, got %q/%q", suite, processID)
	}

	suite, processID = normalizeVerifySuiteInput("")
	if suite != "smoke" || processID != "smoke-suite" {
		t.Fatalf("expected default smoke/smoke-suite, got %q/%q", suite, processID)
	}

	suite, processID = normalizeVerifySuiteInput("checkout-suite")
	if suite != "checkout-suite" || processID != "checkout-suite" {
		t.Fatalf("expected custom suite passthrough, got %q/%q", suite, processID)
	}
}

func TestResolveVerifyProcessSlugNotFoundIncludesAvailable(t *testing.T) {
	server := processListServer(t, `[{
		"id":"proc-smoke","projectId":"project-1","name":"Smoke","slug":"smoke-suite",
		"isExploratory":false,"isActive":true,"configuration":{"ticketLabels":[],"testCaseTags":["smoke"]}
	}]`)
	defer server.Close()

	client := api.NewClient(server.URL, "test-key")
	_, err := resolveVerifyProcessSlug(context.Background(), client, "project-1", "missing-suite")
	if err == nil {
		t.Fatal("expected not found error")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitUsage {
		t.Fatalf("expected usage exit code %d, got %d", ExitUsage, cmdErr.Code)
	}
	if !strings.Contains(cmdErr.Error(), "available CI-compatible suites: smoke-suite") {
		t.Fatalf("unexpected error message: %q", cmdErr.Error())
	}
}

func TestResolveVerifyProcessSlugRejectsInactiveExploratoryAndTicketLabelSuites(t *testing.T) {
	server := processListServer(t, `[
		{
			"id":"proc-inactive","projectId":"project-1","name":"Inactive","slug":"inactive-suite",
			"isExploratory":false,"isActive":false,"configuration":{"ticketLabels":[],"testCaseTags":[]}
		},
		{
			"id":"proc-explore","projectId":"project-1","name":"Explore","slug":"explore-suite",
			"isExploratory":true,"isActive":true,"configuration":{"ticketLabels":[],"testCaseTags":[]}
		},
		{
			"id":"proc-ticket","projectId":"project-1","name":"Ticket","slug":"ticket-suite",
			"isExploratory":false,"isActive":true,"configuration":{"ticketLabels":["bug"],"testCaseTags":[]}
		},
		{
			"id":"proc-smoke","projectId":"project-1","name":"Smoke","slug":"smoke-suite",
			"isExploratory":false,"isActive":true,"configuration":{"ticketLabels":[],"testCaseTags":[]}
		}
	]`)
	defer server.Close()

	client := api.NewClient(server.URL, "test-key")

	tests := []struct {
		identifier string
		substring  string
	}{
		{identifier: "inactive-suite", substring: "inactive"},
		{identifier: "explore-suite", substring: "exploratory"},
		{identifier: "ticket-suite", substring: "ticketLabels"},
	}

	for _, tt := range tests {
		_, err := resolveVerifyProcessSlug(context.Background(), client, "project-1", tt.identifier)
		if err == nil {
			t.Fatalf("expected error for %s", tt.identifier)
		}
		var cmdErr *CommandError
		if !errors.As(err, &cmdErr) {
			t.Fatalf("expected CommandError for %s, got %T (%v)", tt.identifier, err, err)
		}
		if cmdErr.Code != ExitUsage {
			t.Fatalf("expected usage exit code %d for %s, got %d", ExitUsage, tt.identifier, cmdErr.Code)
		}
		if !strings.Contains(cmdErr.Error(), tt.substring) {
			t.Fatalf("expected %q in error for %s, got %q", tt.substring, tt.identifier, cmdErr.Error())
		}
	}
}

func TestResolveVerifyProcessSlugAllowsCustomActiveCiCompatibleProcess(t *testing.T) {
	server := processListServer(t, `[{
		"id":"proc-custom","projectId":"project-1","name":"Checkout","slug":"checkout-suite",
		"isExploratory":false,"isActive":true,"configuration":{"ticketLabels":[],"testCaseTags":["checkout"]}
	}]`)
	defer server.Close()

	client := api.NewClient(server.URL, "test-key")
	slug, err := resolveVerifyProcessSlug(context.Background(), client, "project-1", "checkout-suite")
	if err != nil {
		t.Fatalf("expected custom process resolution success, got %v", err)
	}
	if slug != "checkout-suite" {
		t.Fatalf("expected checkout-suite, got %q", slug)
	}
}

func TestResolveVerifyProcessSlugFallsBackWhenDiscoveryUnauthorized(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/projects/project-1/processes" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client := api.NewClient(server.URL, "test-key")

	slug, err := resolveVerifyProcessSlug(context.Background(), client, "project-1", "smoke-suite")
	if err != nil {
		t.Fatalf("expected fallback success for smoke-suite, got %v", err)
	}
	if slug != "smoke-suite" {
		t.Fatalf("expected smoke-suite fallback, got %q", slug)
	}

	customSlug, err := resolveVerifyProcessSlug(context.Background(), client, "project-1", "checkout-suite")
	if err != nil {
		t.Fatalf("expected fallback success for custom suite, got %v", err)
	}
	if customSlug != "checkout-suite" {
		t.Fatalf("expected checkout-suite fallback, got %q", customSlug)
	}
}

func TestGenerateVerifyEnvironmentKey(t *testing.T) {
	key := generateVerifyEnvironmentKey("dev")
	if !strings.HasPrefix(key, "dev-") {
		t.Fatalf("expected dev- prefix, got %q", key)
	}
	if strings.Contains(key, "_") {
		t.Fatalf("expected hyphenated key, got %q", key)
	}
	if len(key) > 63 {
		t.Fatalf("expected key length <= 63, got %d", len(key))
	}
}

func TestBuildVerifyVersion(t *testing.T) {
	version := buildVerifyVersion("abcdef1234567890")
	if version != "verify-abcdef123456" {
		t.Fatalf("unexpected version with sha: %q", version)
	}

	version = buildVerifyVersion("")
	if !strings.HasPrefix(version, "verify-") {
		t.Fatalf("expected verify- prefix, got %q", version)
	}
	if len(version) < len("verify-")+8 {
		t.Fatalf("expected timestamp suffix, got %q", version)
	}
}

func TestPreflightVerifyURLAcceptsAnyStatusCode(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	err := preflightVerifyURL(contextWithTimeout(t, time.Second), server.URL)
	if err != nil {
		t.Fatalf("expected 404 to pass preflight, got %v", err)
	}
}

func contextWithTimeout(t *testing.T, d time.Duration) context.Context {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), d)
	t.Cleanup(cancel)
	return ctx
}

func processListServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet && r.URL.Path == "/projects/project-1/processes" {
			_, _ = io.WriteString(w, body)
			return
		}
		http.NotFound(w, r)
	}))
}
