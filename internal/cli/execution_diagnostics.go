package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/certyn/certyn-cli/internal/api"
)

const (
	verifySchemaVersion            = "certyn.verify.v1"
	executionDiagnoseSchemaVersion = "certyn.execution_diagnose.v1"
)

type executionDiagnosticCounts struct {
	ToolErrors   int `json:"tool_errors"`
	PolicyDenied int `json:"policy_denied"`
	Network4xx   int `json:"network_4xx"`
	Network5xx   int `json:"network_5xx"`
	NetworkTotal int `json:"network_total"`
}

type executionDiagnosticNetworkFailure struct {
	RequestID    string     `json:"request_id,omitempty"`
	Method       string     `json:"method,omitempty"`
	URL          string     `json:"url,omitempty"`
	Status       int        `json:"status,omitempty"`
	StatusText   string     `json:"status_text,omitempty"`
	TimestampUTC *time.Time `json:"timestamp_utc,omitempty"`
}

type executionDiagnosticToolFailure struct {
	ToolName     string     `json:"tool_name,omitempty"`
	ToolCallID   string     `json:"tool_call_id,omitempty"`
	Status       string     `json:"status,omitempty"`
	Error        string     `json:"error,omitempty"`
	Output       string     `json:"output,omitempty"`
	TimestampUTC *time.Time `json:"timestamp_utc,omitempty"`
}

type executionDiagnostic struct {
	ExecutionID                 string                              `json:"execution_id,omitempty"`
	AgentSessionID              string                              `json:"agent_session_id,omitempty"`
	PrimaryFailureReason        string                              `json:"primary_failure_reason,omitempty"`
	FailureSummary              string                              `json:"failure_summary,omitempty"`
	ConversationCompletedStatus string                              `json:"conversation_completed_status,omitempty"`
	Counts                      executionDiagnosticCounts           `json:"counts"`
	NetworkFailures             []executionDiagnosticNetworkFailure `json:"network_failures,omitempty"`
	ToolFailures                []executionDiagnosticToolFailure    `json:"tool_failures,omitempty"`
	DiagnoseCommand             string                              `json:"diagnose_command,omitempty"`
	ConversationCommand         string                              `json:"conversation_command,omitempty"`
}

type verifyDiagnosticError struct {
	ExecutionID string `json:"execution_id,omitempty"`
	Error       string `json:"error"`
}

type executionDiagnosticEventWindow struct {
	Scanned   int    `json:"scanned"`
	Truncated bool   `json:"truncated"`
	HasMore   bool   `json:"has_more"`
	NextPage  int    `json:"next_page,omitempty"`
	Before    string `json:"before,omitempty"`
}

type executionDiagnosticSubject struct {
	ExecutionID    string
	AgentSessionID string
	Status         string
	Outcome        string
}

type executionDiagnosticsOptions struct {
	Before     *time.Time
	Page       int
	PageSize   int
	MaxEvents  int
	SampleSize int
}

type executionDiagnosticResult struct {
	Diagnostic  executionDiagnostic
	EventWindow executionDiagnosticEventWindow
}

type executionDiagnoseOutput struct {
	SchemaVersion               string                              `json:"schema_version"`
	ExecutionID                 string                              `json:"execution_id"`
	AgentSessionID              string                              `json:"agent_session_id,omitempty"`
	EventWindow                 executionDiagnosticEventWindow      `json:"event_window"`
	PrimaryFailureReason        string                              `json:"primary_failure_reason,omitempty"`
	FailureSummary              string                              `json:"failure_summary,omitempty"`
	ConversationCompletedStatus string                              `json:"conversation_completed_status,omitempty"`
	Counts                      executionDiagnosticCounts           `json:"counts"`
	NetworkFailures             []executionDiagnosticNetworkFailure `json:"network_failures,omitempty"`
	ToolFailures                []executionDiagnosticToolFailure    `json:"tool_failures,omitempty"`
	ConversationCommand         string                              `json:"conversation_command,omitempty"`
}

type diagnosticRequestContext struct {
	Method string
	URL    string
}

type conversationDiagnosticsAccumulator struct {
	sampleSize          int
	toolNamesByCallID   map[string]string
	requestsByRequestID map[string]diagnosticRequestContext
	counts              executionDiagnosticCounts
	networkFailures     []executionDiagnosticNetworkFailure
	toolFailures        []executionDiagnosticToolFailure
	conversationStatus  string
}

func collectExecutionDiagnostics(
	ctx context.Context,
	client *api.Client,
	projectID,
	projectSlug string,
	subject executionDiagnosticSubject,
	options executionDiagnosticsOptions,
) (executionDiagnosticResult, error) {
	if strings.TrimSpace(subject.ExecutionID) == "" {
		return executionDiagnosticResult{}, usageError("missing execution id for diagnostics", nil)
	}

	normalizedOptions, err := normalizeExecutionDiagnosticsOptions(options)
	if err != nil {
		return executionDiagnosticResult{}, err
	}

	sessionID := strings.TrimSpace(subject.AgentSessionID)
	if sessionID == "" {
		execution, getErr := client.GetExecution(ctx, projectID, strings.TrimSpace(subject.ExecutionID))
		if getErr != nil {
			return executionDiagnosticResult{}, classifyAPIError(getErr, "failed to get execution")
		}
		sessionID = strings.TrimSpace(ptrString(execution.AgentSessionID))
	}
	if sessionID == "" {
		return executionDiagnosticResult{}, notFoundError(
			fmt.Sprintf("execution '%s' has no agent session conversation", strings.TrimSpace(subject.ExecutionID)),
			nil,
		)
	}

	acc := conversationDiagnosticsAccumulator{
		sampleSize:          normalizedOptions.SampleSize,
		toolNamesByCallID:   map[string]string{},
		requestsByRequestID: map[string]diagnosticRequestContext{},
	}

	page := normalizedOptions.Page
	window := executionDiagnosticEventWindow{}
	if normalizedOptions.Before != nil {
		window.Before = normalizedOptions.Before.UTC().Format(time.RFC3339Nano)
	}

	for {
		if window.Scanned >= normalizedOptions.MaxEvents {
			window.Truncated = true
			window.HasMore = true
			window.NextPage = page
			break
		}

		resp, listErr := client.ListAgentSessionConversation(
			ctx,
			projectID,
			sessionID,
			api.ListConversationEventsParams{
				Before:   normalizedOptions.Before,
				Page:     page,
				PageSize: normalizedOptions.PageSize,
			},
		)
		if listErr != nil {
			return executionDiagnosticResult{}, classifyAPIError(listErr, "failed to list execution conversation")
		}

		for _, event := range resp.Items {
			if window.Scanned >= normalizedOptions.MaxEvents {
				window.Truncated = true
				window.HasMore = true
				window.NextPage = page
				break
			}
			window.Scanned++
			acc.ingestEvent(event)
		}

		if window.Truncated {
			break
		}
		if !resp.HasNextPage {
			break
		}

		window.HasMore = true
		page++
		window.NextPage = page
	}

	diagnostic := executionDiagnostic{
		ExecutionID:                 strings.TrimSpace(subject.ExecutionID),
		AgentSessionID:              sessionID,
		ConversationCompletedStatus: acc.conversationStatus,
		Counts:                      acc.counts,
		NetworkFailures:             acc.networkFailures,
		ToolFailures:                acc.toolFailures,
	}
	diagnostic.PrimaryFailureReason = classifyExecutionFailureReason(
		subject.Status,
		subject.Outcome,
		diagnostic.ConversationCompletedStatus,
		diagnostic.Counts,
	)
	diagnostic.FailureSummary = buildExecutionFailureSummary(diagnostic, subject.Status, subject.Outcome)
	projectHint := strings.TrimSpace(projectSlug)
	if projectHint != "" {
		diagnostic.DiagnoseCommand = fmt.Sprintf(
			"certyn executions diagnose --project %s %s",
			projectHint,
			diagnostic.ExecutionID,
		)
		diagnostic.ConversationCommand = fmt.Sprintf(
			"certyn executions conversation --project %s %s",
			projectHint,
			diagnostic.ExecutionID,
		)
	}

	return executionDiagnosticResult{
		Diagnostic:  diagnostic,
		EventWindow: window,
	}, nil
}

func normalizeExecutionDiagnosticsOptions(options executionDiagnosticsOptions) (executionDiagnosticsOptions, error) {
	if options.Page <= 0 {
		options.Page = 1
	}
	if options.PageSize <= 0 {
		options.PageSize = 500
	}
	if options.PageSize > 1000 {
		options.PageSize = 1000
	}
	if options.MaxEvents <= 0 {
		return executionDiagnosticsOptions{}, usageError("invalid --max-events: must be greater than zero", nil)
	}
	if options.SampleSize < 0 {
		return executionDiagnosticsOptions{}, usageError("invalid --sample-size: must be zero or greater", nil)
	}
	return options, nil
}

func (a *conversationDiagnosticsAccumulator) ingestEvent(event api.ConversationEvent) {
	switch strings.TrimSpace(event.Type) {
	case "tool_use_requested":
		payload := jsonPayloadMap(event.Payload)
		toolCallID := payloadString(payload, "toolCallId")
		toolName := payloadString(payload, "toolName")
		if toolCallID != "" && toolName != "" {
			a.toolNamesByCallID[toolCallID] = toolName
		}
	case "tool_result":
		payload := jsonPayloadMap(event.Payload)
		status := strings.ToLower(strings.TrimSpace(payloadString(payload, "status")))
		if !isToolFailureStatus(status) {
			return
		}

		toolCallID := firstNonEmpty(
			payloadNestedString(payload, "metadata", "toolCallId"),
			payloadString(payload, "toolCallId"),
			payloadString(payload, "commandId"),
		)
		toolName := firstNonEmpty(
			payloadNestedString(payload, "metadata", "toolName"),
			payloadString(payload, "toolName"),
		)
		if toolName == "" && toolCallID != "" {
			toolName = a.toolNamesByCallID[toolCallID]
		}

		a.counts.ToolErrors++
		a.appendToolFailure(executionDiagnosticToolFailure{
			ToolName:     strings.TrimSpace(toolName),
			ToolCallID:   strings.TrimSpace(toolCallID),
			Status:       status,
			Error:        summarizeFailureString(payloadString(payload, "error")),
			Output:       summarizeFailureString(payloadString(payload, "output")),
			TimestampUTC: timestampPointer(event.TimestampUTC),
		})
	case "policy_decision":
		payload := jsonPayloadMap(event.Payload)
		allowed, ok := payloadBool(payload, "allowed")
		if ok && !allowed {
			a.counts.PolicyDenied++
		}
	case "browser_network_request":
		payload := jsonPayloadMap(event.Payload)
		requestID := payloadString(payload, "requestId")
		if strings.TrimSpace(requestID) == "" {
			return
		}
		a.requestsByRequestID[requestID] = diagnosticRequestContext{
			Method: strings.TrimSpace(payloadString(payload, "method")),
			URL:    strings.TrimSpace(payloadString(payload, "url")),
		}
	case "browser_network_response":
		payload := jsonPayloadMap(event.Payload)
		status := payloadInt(payload, "status")
		if status > 0 {
			a.counts.NetworkTotal++
		}
		switch {
		case status >= 500 && status <= 599:
			a.counts.Network5xx++
		case status >= 400 && status <= 499:
			a.counts.Network4xx++
		default:
			return
		}

		requestID := strings.TrimSpace(payloadString(payload, "requestId"))
		method := strings.TrimSpace(payloadString(payload, "method"))
		urlValue := strings.TrimSpace(payloadString(payload, "url"))
		if ctx, ok := a.requestsByRequestID[requestID]; ok {
			if method == "" {
				method = strings.TrimSpace(ctx.Method)
			}
			if urlValue == "" {
				urlValue = strings.TrimSpace(ctx.URL)
			}
		}

		a.appendNetworkFailure(executionDiagnosticNetworkFailure{
			RequestID:    requestID,
			Method:       method,
			URL:          urlValue,
			Status:       status,
			StatusText:   strings.TrimSpace(payloadString(payload, "statusText")),
			TimestampUTC: timestampPointer(event.TimestampUTC),
		})
	case "conversation_completed":
		payload := jsonPayloadMap(event.Payload)
		status := strings.TrimSpace(payloadString(payload, "status"))
		if status != "" {
			a.conversationStatus = status
		}
	case "job_completed":
		payload := jsonPayloadMap(event.Payload)
		if a.conversationStatus == "" {
			status := strings.TrimSpace(payloadString(payload, "status"))
			if status != "" {
				a.conversationStatus = status
			}
		}
	case "job_state_changed", "job_running", "job_stopping", "job_stopped":
		// Parse JSON for known lifecycle events so diagnostics can be extended without
		// changing scan behavior.
		_ = jsonPayloadMap(event.Payload)
	default:
		return
	}
}

func (a *conversationDiagnosticsAccumulator) appendNetworkFailure(failure executionDiagnosticNetworkFailure) {
	if a.sampleSize == 0 || len(a.networkFailures) >= a.sampleSize {
		return
	}
	a.networkFailures = append(a.networkFailures, failure)
}

func (a *conversationDiagnosticsAccumulator) appendToolFailure(failure executionDiagnosticToolFailure) {
	if a.sampleSize == 0 || len(a.toolFailures) >= a.sampleSize {
		return
	}
	a.toolFailures = append(a.toolFailures, failure)
}

func classifyExecutionFailureReason(status, outcome, conversationStatus string, counts executionDiagnosticCounts) string {
	normalizedStatus := strings.ToLower(strings.TrimSpace(status))
	normalizedOutcome := strings.ToLower(strings.TrimSpace(outcome))
	normalizedConversationStatus := strings.ToLower(strings.TrimSpace(conversationStatus))

	if normalizedOutcome == "blocked" || normalizedStatus == "blocked" {
		return "execution_blocked"
	}
	if normalizedOutcome == "aborted" || normalizedStatus == "aborted" {
		return "execution_aborted"
	}
	if isFailedConversationStatus(normalizedConversationStatus) {
		return "conversation_failed"
	}
	if counts.ToolErrors > 0 {
		return "tool_error"
	}
	if counts.Network5xx > 0 && counts.Network5xx >= counts.Network4xx {
		return "network_5xx"
	}
	if counts.Network4xx > 0 {
		return "network_4xx"
	}
	if normalizedOutcome == "failed" || normalizedStatus == "failed" {
		return "execution_failed"
	}
	return "unknown"
}

func buildExecutionFailureSummary(diagnostic executionDiagnostic, status, outcome string) string {
	suffix := ""
	if diagnostic.Counts.PolicyDenied > 0 {
		suffix = fmt.Sprintf("; policy_denied=%d", diagnostic.Counts.PolicyDenied)
	}

	switch diagnostic.PrimaryFailureReason {
	case "execution_blocked":
		return "execution reported blocked outcome" + suffix
	case "execution_aborted":
		return "execution reported aborted outcome" + suffix
	case "conversation_failed":
		statusText := strings.TrimSpace(diagnostic.ConversationCompletedStatus)
		if statusText == "" {
			statusText = "failed"
		}
		return fmt.Sprintf("conversation completed with status %s", statusText) + suffix
	case "tool_error":
		if len(diagnostic.ToolFailures) == 0 {
			return fmt.Sprintf("%d tool error(s) detected", diagnostic.Counts.ToolErrors) + suffix
		}
		first := diagnostic.ToolFailures[0]
		toolName := valueOrDash(strings.TrimSpace(first.ToolName))
		detail := first.Error
		if strings.TrimSpace(detail) == "" {
			detail = first.Status
		}
		if strings.TrimSpace(detail) == "" {
			detail = "failed"
		}
		return fmt.Sprintf("%d tool error(s); first=%s (%s)", diagnostic.Counts.ToolErrors, toolName, detail) + suffix
	case "network_5xx":
		return fmt.Sprintf("network failures detected: %d server error response(s)", diagnostic.Counts.Network5xx) + suffix
	case "network_4xx":
		return fmt.Sprintf("network failures detected: %d client error response(s)", diagnostic.Counts.Network4xx) + suffix
	case "execution_failed":
		if strings.TrimSpace(outcome) != "" {
			return fmt.Sprintf("execution outcome=%s", strings.TrimSpace(outcome)) + suffix
		}
		if strings.TrimSpace(status) != "" {
			return fmt.Sprintf("execution status=%s", strings.TrimSpace(status)) + suffix
		}
		return "execution reported failure" + suffix
	default:
		if diagnostic.Counts.PolicyDenied > 0 {
			return fmt.Sprintf("policy denied decisions detected: %d", diagnostic.Counts.PolicyDenied)
		}
		return "no deterministic failure signal found in scanned conversation events"
	}
}

func isFailedConversationStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "", "succeeded", "success", "completed", "ok", "passed":
		return false
	default:
		return true
	}
}

func isToolFailureStatus(status string) bool {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "failed", "error", "errored", "denied", "blocked", "aborted", "cancelled", "canceled", "timeout", "timed_out":
		return true
	default:
		return false
	}
}

func summarizeFailureString(value string) string {
	compact := strings.Join(strings.Fields(strings.TrimSpace(value)), " ")
	if len(compact) <= 240 {
		return compact
	}
	return compact[:240] + "..."
}

func jsonPayloadMap(raw json.RawMessage) map[string]any {
	if len(raw) == 0 {
		return nil
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	return payload
}

func payloadString(payload map[string]any, key string) string {
	if payload == nil {
		return ""
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return typed
	case json.Number:
		return typed.String()
	case float64:
		return fmt.Sprintf("%.0f", typed)
	default:
		return ""
	}
}

func payloadNestedString(payload map[string]any, parent, key string) string {
	if payload == nil {
		return ""
	}
	rawParent, ok := payload[parent]
	if !ok || rawParent == nil {
		return ""
	}
	parentMap, ok := rawParent.(map[string]any)
	if !ok {
		return ""
	}
	return payloadString(parentMap, key)
}

func payloadBool(payload map[string]any, key string) (bool, bool) {
	if payload == nil {
		return false, false
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return false, false
	}
	switch typed := value.(type) {
	case bool:
		return typed, true
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true":
			return true, true
		case "false":
			return false, true
		default:
			return false, false
		}
	default:
		return false, false
	}
}

func payloadInt(payload map[string]any, key string) int {
	if payload == nil {
		return 0
	}
	value, ok := payload[key]
	if !ok || value == nil {
		return 0
	}
	switch typed := value.(type) {
	case int:
		return typed
	case int32:
		return int(typed)
	case int64:
		return int(typed)
	case float64:
		return int(typed)
	case json.Number:
		if parsed, err := typed.Int64(); err == nil {
			return int(parsed)
		}
		return 0
	case string:
		var parsed int
		_, _ = fmt.Sscanf(strings.TrimSpace(typed), "%d", &parsed)
		return parsed
	default:
		return 0
	}
}

func timestampPointer(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	copied := value
	return &copied
}
