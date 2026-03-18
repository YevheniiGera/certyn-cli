package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestCreateEnvironmentSendsFlatPayload(t *testing.T) {
	var captured map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/projects/project-1/environments" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_, _ = w.Write([]byte(`{"id":"env-1","projectId":"project-1","key":"staging","label":"Staging","baseUrl":"https://staging.example.com","isDefault":false,"version":"1.0.0","anthropicModel":"","executionTarget":"cloud_managed","runnerPoolId":""}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	_, err := client.CreateEnvironment(context.Background(), "project-1", EnvironmentInput{
		Key:     "staging",
		Label:   "Staging",
		BaseURL: "https://staging.example.com",
		Version: "1.0.0",
	})
	if err != nil {
		t.Fatalf("CreateEnvironment failed: %v", err)
	}

	if _, hasWrapper := captured["environment"]; hasWrapper {
		t.Fatalf("expected flat payload, got wrapped payload: %#v", captured)
	}
	if captured["key"] != "staging" {
		t.Fatalf("expected key=staging, got %#v", captured["key"])
	}
}

func TestUpdateEnvironmentSendsFlatPayload(t *testing.T) {
	var captured map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/projects/project-1/environments/env-1" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	err := client.UpdateEnvironment(context.Background(), "project-1", "env-1", EnvironmentInput{
		BaseURL: "https://updated.example.com",
		Version: "2.0.0",
	})
	if err != nil {
		t.Fatalf("UpdateEnvironment failed: %v", err)
	}

	if _, hasWrapper := captured["environment"]; hasWrapper {
		t.Fatalf("expected flat payload, got wrapped payload: %#v", captured)
	}
	if captured["baseUrl"] != "https://updated.example.com" {
		t.Fatalf("expected baseUrl update, got %#v", captured["baseUrl"])
	}
}

func TestCreateEnvironmentVariableSendsFlatPayload(t *testing.T) {
	var captured map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/projects/project-1/environments/env-1/variables" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		_, _ = w.Write([]byte(`{"id":"var-1","environmentId":"env-1","name":"API_KEY","value":"secret","description":"test"}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	_, err := client.CreateEnvironmentVariable(context.Background(), "project-1", "env-1", EnvironmentVariableInput{
		Name:        "API_KEY",
		Value:       "secret",
		Description: "test",
	})
	if err != nil {
		t.Fatalf("CreateEnvironmentVariable failed: %v", err)
	}

	if _, hasWrapper := captured["variable"]; hasWrapper {
		t.Fatalf("expected flat payload, got wrapped payload: %#v", captured)
	}
	if captured["name"] != "API_KEY" {
		t.Fatalf("expected name=API_KEY, got %#v", captured["name"])
	}
}

func TestUpdateEnvironmentVariableSendsFlatPayload(t *testing.T) {
	var captured map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPut || r.URL.Path != "/projects/project-1/environments/env-1/variables/var-1" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request body: %v", err)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	err := client.UpdateEnvironmentVariable(context.Background(), "project-1", "env-1", "var-1", EnvironmentVariableInput{
		Name:  "API_KEY",
		Value: "updated",
	})
	if err != nil {
		t.Fatalf("UpdateEnvironmentVariable failed: %v", err)
	}

	if _, hasWrapper := captured["variable"]; hasWrapper {
		t.Fatalf("expected flat payload, got wrapped payload: %#v", captured)
	}
	if captured["value"] != "updated" {
		t.Fatalf("expected value=updated, got %#v", captured["value"])
	}
}

func TestListProcessesDecodesConfiguration(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/projects/project-1/processes" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`[
			{
				"id":"proc-1",
				"projectId":"project-1",
				"name":"Smoke Suite",
				"slug":"smoke-suite",
				"isExploratory":false,
				"isActive":true,
				"configuration":{
					"ticketLabels":[],
					"testCaseTags":["smoke"]
				}
			}
		]`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	processes, err := client.ListProcesses(context.Background(), "project-1")
	if err != nil {
		t.Fatalf("ListProcesses failed: %v", err)
	}

	if len(processes) != 1 {
		t.Fatalf("expected 1 process, got %d", len(processes))
	}
	if processes[0].Slug != "smoke-suite" {
		t.Fatalf("expected slug smoke-suite, got %q", processes[0].Slug)
	}
	if len(processes[0].Configuration.TestCaseTags) != 1 || processes[0].Configuration.TestCaseTags[0] != "smoke" {
		t.Fatalf("unexpected testCaseTags: %#v", processes[0].Configuration.TestCaseTags)
	}
}

func TestListAgentSessionConversationMapsQueryAndDecodesPayload(t *testing.T) {
	before := time.Date(2026, 2, 23, 8, 0, 0, 0, time.UTC)
	var capturedQuery string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/projects/project-1/agentsessions/session-1/conversation" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		capturedQuery = r.URL.RawQuery
		_, _ = w.Write([]byte(`{
			"items":[
				{
					"type":"tool_result",
					"timestampUtc":"2026-02-23T08:01:00Z",
					"payload":{"tool":"open_url","ok":true}
				}
			],
			"totalCount":1,
			"page":2,
			"pageSize":50,
			"totalPages":1,
			"hasNextPage":false,
			"hasPreviousPage":true
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	resp, err := client.ListAgentSessionConversation(context.Background(), "project-1", "session-1", ListConversationEventsParams{
		Before:   &before,
		Page:     2,
		PageSize: 50,
	})
	if err != nil {
		t.Fatalf("ListAgentSessionConversation failed: %v", err)
	}

	if !strings.Contains(capturedQuery, "page=2") || !strings.Contains(capturedQuery, "pageSize=50") || !strings.Contains(capturedQuery, "before=2026-02-23T08%3A00%3A00Z") {
		t.Fatalf("unexpected query: %s", capturedQuery)
	}
	if len(resp.Items) != 1 {
		t.Fatalf("expected 1 conversation event, got %d", len(resp.Items))
	}
	if resp.Items[0].Type != "tool_result" {
		t.Fatalf("expected type=tool_result, got %q", resp.Items[0].Type)
	}
	if string(resp.Items[0].Payload) != `{"tool":"open_url","ok":true}` {
		t.Fatalf("unexpected payload: %s", string(resp.Items[0].Payload))
	}
}

func TestGetProcessRunDecodesItems(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/projects/project-1/process-runs/run-1" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		_, _ = w.Write([]byte(`{
			"id":"run-1",
			"number":12,
			"processId":"proc-1",
			"processName":"Smoke",
			"processSlug":"smoke-suite",
			"status":"completed",
			"triggerType":"ci",
			"startedAt":"2026-02-23T08:00:00Z",
			"completedAt":"2026-02-23T08:02:00Z",
			"summaryMarkdown":"done",
			"totalItems":2,
			"passedItems":1,
			"failedItems":1,
			"blockedItems":0,
			"runningItems":0,
			"pendingItems":0,
			"items":[
				{
					"id":"pri-1",
					"testCaseId":"tc-1",
					"testCaseName":"Case 1",
					"testCaseNumber":1,
					"executionId":"exe-1",
					"agentSessionId":"session-1",
					"status":"completed",
					"testOutcome":"failed",
					"summaryMarkdown":"failed details",
					"startedAt":"2026-02-23T08:00:10Z",
					"completedAt":"2026-02-23T08:01:00Z"
				}
			]
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	resp, err := client.GetProcessRun(context.Background(), "project-1", "run-1")
	if err != nil {
		t.Fatalf("GetProcessRun failed: %v", err)
	}

	if resp.ID != "run-1" {
		t.Fatalf("expected run id run-1, got %q", resp.ID)
	}
	if resp.FailedItems != 1 {
		t.Fatalf("expected failedItems=1, got %d", resp.FailedItems)
	}
	if len(resp.Items) != 1 || resp.Items[0].ExecutionID == nil || *resp.Items[0].ExecutionID != "exe-1" {
		t.Fatalf("unexpected items: %#v", resp.Items)
	}
}

func TestClientUsesBearerTokenWhenApiKeyMissing(t *testing.T) {
	var authHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"defaultProjectId":"","projects":[]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "")
	client.SetAccessToken("token-123")

	if _, err := client.GetProjectsOverview(context.Background()); err != nil {
		t.Fatalf("GetProjectsOverview failed: %v", err)
	}

	if authHeader != "Bearer token-123" {
		t.Fatalf("expected bearer token header, got %q", authHeader)
	}
}

func TestClientPrefersBearerTokenOverAPIKey(t *testing.T) {
	var authHeader string
	var apiKeyHeader string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		apiKeyHeader = r.Header.Get("X-API-Key")
		_, _ = w.Write([]byte(`{"defaultProjectId":"","projects":[]}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	client.SetAccessToken("token-123")

	if _, err := client.GetProjectsOverview(context.Background()); err != nil {
		t.Fatalf("GetProjectsOverview failed: %v", err)
	}

	if authHeader != "Bearer token-123" {
		t.Fatalf("expected bearer token header, got %q", authHeader)
	}
	if apiKeyHeader != "" {
		t.Fatalf("expected X-API-Key to be omitted when bearer auth is present, got %q", apiKeyHeader)
	}
}

func TestAskAdvisorSendsPayloadAndDecodesResponse(t *testing.T) {
	var captured map[string]any

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/chat/advisor" {
			t.Fatalf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if err := json.NewDecoder(r.Body).Decode(&captured); err != nil {
			t.Fatalf("decode request body: %v", err)
		}

		_, _ = w.Write([]byte(`{
			"conversationId":"advisor-1",
			"messageId":"msg-1",
			"content":"Use smoke-suite first.",
			"role":"assistant",
			"toolCalls":[
				{"toolName":"list_processes","success":true,"result":{"count":2},"error":""}
			],
			"createdAt":"2026-02-23T08:15:00Z"
		}`))
	}))
	defer server.Close()

	client := NewClient(server.URL, "test-key")
	maxTools := 4
	maxTokens := 800
	resp, err := client.AskAdvisor(context.Background(), AskAdvisorRequest{
		Message:             "What should I run?",
		ProjectID:           "project-1",
		MaxToolIterations:   &maxTools,
		MaxOutputTokenCount: &maxTokens,
	})
	if err != nil {
		t.Fatalf("AskAdvisor failed: %v", err)
	}

	if captured["message"] != "What should I run?" {
		t.Fatalf("expected message field, got %#v", captured["message"])
	}
	if captured["projectId"] != "project-1" {
		t.Fatalf("expected projectId field, got %#v", captured["projectId"])
	}
	if int(captured["maxToolIterations"].(float64)) != 4 {
		t.Fatalf("expected maxToolIterations=4, got %#v", captured["maxToolIterations"])
	}
	if int(captured["maxOutputTokenCount"].(float64)) != 800 {
		t.Fatalf("expected maxOutputTokenCount=800, got %#v", captured["maxOutputTokenCount"])
	}

	if resp.ConversationID != "advisor-1" {
		t.Fatalf("expected conversationId=advisor-1, got %q", resp.ConversationID)
	}
	if resp.Content != "Use smoke-suite first." {
		t.Fatalf("expected content decode, got %q", resp.Content)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].ToolName != "list_processes" {
		t.Fatalf("unexpected toolCalls: %#v", resp.ToolCalls)
	}
}
