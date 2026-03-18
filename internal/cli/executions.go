package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/certyn/certyn-cli/internal/api"
	"github.com/certyn/certyn-cli/internal/config"
	"github.com/spf13/cobra"
)

var allowedExecutionStatuses = []string{
	"draft",
	"queued",
	"dispatching",
	"starting",
	"running",
	"completed",
	"failed",
	"aborted",
	"blocked",
}

func newExecutionsCommand(app *App) *cobra.Command {
	executionsCmd := &cobra.Command{
		Use:   "executions",
		Short: "Execution triage operations",
	}

	executionsCmd.AddCommand(newExecutionsListCommand(app))
	executionsCmd.AddCommand(newExecutionsGetCommand(app))
	executionsCmd.AddCommand(newExecutionsForIssueCommand(app))
	executionsCmd.AddCommand(newExecutionsArtifactsCommand(app))
	executionsCmd.AddCommand(newExecutionsNotesCommand(app))
	executionsCmd.AddCommand(newExecutionsTestcasesCommand(app))
	executionsCmd.AddCommand(newExecutionsConversationCommand(app))
	executionsCmd.AddCommand(newExecutionsDiagnoseCommand(app))
	executionsCmd.AddCommand(newExecutionsRetryCommand(app))
	executionsCmd.AddCommand(newExecutionsStopCommand(app))

	return executionsCmd
}

func newExecutionsListCommand(app *App) *cobra.Command {
	var project string
	var status string
	var agentID string
	var page int
	var pageSize int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List executions for a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, client, printer, err := app.ResolveRuntime(config.ResolveInput{Project: project}, true)
			if err != nil {
				return err
			}
			if err := requireValue("project", resolved.Project); err != nil {
				return err
			}
			projectID, err := resolveProjectRouteIDFromConfig(app, resolved.Project, resolved.Profile)
			if err != nil {
				return err
			}

			status, err = normalizeCSVEnum("status", status, allowedExecutionStatuses)
			if err != nil {
				return err
			}
			environmentKey := explicitFlagString(cmd, "environment")

			resp, err := client.ListExecutions(cmd.Context(), projectID, api.ListExecutionsParams{
				Status:         status,
				AgentID:        strings.TrimSpace(agentID),
				EnvironmentKey: environmentKey,
				Page:           page,
				PageSize:       pageSize,
			})
			if err != nil {
				return classifyProjectRouteError(err, resolved.Project, resolved.Profile, "failed to list executions")
			}
			if printer.JSON {
				return printer.EmitJSON(resp)
			}

			fmt.Printf("Executions: %d\n", resp.TotalCount)
			fmt.Printf("%-36s %-11s %-10s %-10s %-36s %s\n",
				"ID", "STATUS", "OUTCOME", "DURATION", "TICKET", "TITLE")
			for _, execution := range resp.Items {
				ticketID := valueOrDash(execution.TicketID)
				ticketTitle := "-"
				if execution.Ticket != nil {
					ticketID = valueOrDash(execution.Ticket.ID)
					ticketTitle = valueOrDash(execution.Ticket.Title)
				}
				fmt.Printf("%-36s %-11s %-10s %-10ds %-36s %s\n",
					execution.ID,
					execution.Status,
					valueOrDash(execution.EffectiveTestOutcome),
					execution.Duration,
					ticketID,
					ticketTitle,
				)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&status, "status", "", "Execution status filter, comma-separated")
	cmd.Flags().StringVar(&agentID, "agent-id", "", "Filter by agent id")
	cmd.Flags().String("environment", "", "Filter by environment key")
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&pageSize, "page-size", 20, "Page size")

	return cmd
}

func newExecutionsGetCommand(app *App) *cobra.Command {
	var project string

	cmd := &cobra.Command{
		Use:   "get <execution-id>",
		Short: "Get execution details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, client, printer, err := app.ResolveRuntime(config.ResolveInput{Project: project}, true)
			if err != nil {
				return err
			}
			if err := requireValue("project", resolved.Project); err != nil {
				return err
			}
			projectID, err := resolveProjectRouteIDFromConfig(app, resolved.Project, resolved.Profile)
			if err != nil {
				return err
			}

			execution, err := client.GetExecution(cmd.Context(), projectID, strings.TrimSpace(args[0]))
			if err != nil {
				return classifyAPIError(err, "failed to get execution")
			}
			if printer.JSON {
				return printer.EmitJSON(execution)
			}

			fmt.Printf("Execution %s\n", execution.ID)
			fmt.Printf("Ticket: %s\n", valueOrDash(execution.TicketID))
			fmt.Printf("Status: %s\n", execution.Status)
			fmt.Printf("Trigger type: %s\n", execution.TriggerType)
			fmt.Printf("Started at: %s\n", timeStringOrDash(execution.StartedAt))
			fmt.Printf("Completed at: %s\n", timeStringOrDash(execution.CompletedAt))
			fmt.Printf("Summary available: %t\n", execution.SummaryMarkdown != nil && strings.TrimSpace(*execution.SummaryMarkdown) != "")
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	return cmd
}

func newExecutionsForIssueCommand(app *App) *cobra.Command {
	var project string
	var page int
	var pageSize int

	cmd := &cobra.Command{
		Use:   "for-issue <issue-id>",
		Short: "List executions associated with an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, client, printer, err := app.ResolveRuntime(config.ResolveInput{Project: project}, true)
			if err != nil {
				return err
			}
			if err := requireValue("project", resolved.Project); err != nil {
				return err
			}
			projectID, err := resolveProjectRouteIDFromConfig(app, resolved.Project, resolved.Profile)
			if err != nil {
				return err
			}

			resp, err := client.ListExecutionsForIssue(cmd.Context(), projectID, strings.TrimSpace(args[0]), page, pageSize)
			if err != nil {
				return classifyAPIError(err, "failed to list issue executions")
			}
			if printer.JSON {
				return printer.EmitJSON(resp)
			}

			fmt.Printf("Executions for issue %s: %d\n", strings.TrimSpace(args[0]), resp.TotalCount)
			for _, execution := range resp.Items {
				fmt.Printf("- %s status=%s startedAt=%s completedAt=%s\n",
					execution.ID,
					execution.Status,
					timeStringOrDash(execution.StartedAt),
					timeStringOrDash(execution.CompletedAt),
				)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&pageSize, "page-size", 20, "Page size")
	return cmd
}

func newExecutionsArtifactsCommand(app *App) *cobra.Command {
	var project string
	var page int
	var pageSize int

	cmd := &cobra.Command{
		Use:   "artifacts <execution-id>",
		Short: "List execution artifacts",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, client, printer, err := app.ResolveRuntime(config.ResolveInput{Project: project}, true)
			if err != nil {
				return err
			}
			if err := requireValue("project", resolved.Project); err != nil {
				return err
			}
			projectID, err := resolveProjectRouteIDFromConfig(app, resolved.Project, resolved.Profile)
			if err != nil {
				return err
			}

			resp, err := client.ListExecutionArtifacts(cmd.Context(), projectID, strings.TrimSpace(args[0]), page, pageSize)
			if err != nil {
				return classifyAPIError(err, "failed to list execution artifacts")
			}
			if printer.JSON {
				return printer.EmitJSON(resp)
			}

			fmt.Printf("Artifacts: %d\n", resp.TotalCount)
			for _, artifact := range resp.Items {
				fmt.Printf("- %s type=%s uri=%s createdAt=%s\n",
					artifact.ID,
					artifact.Type,
					ptrStringOrDash(artifact.URI),
					artifact.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
				)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&pageSize, "page-size", 20, "Page size")
	return cmd
}

func newExecutionsNotesCommand(app *App) *cobra.Command {
	var project string
	var page int
	var pageSize int

	cmd := &cobra.Command{
		Use:   "notes <execution-id>",
		Short: "List execution notes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, client, printer, err := app.ResolveRuntime(config.ResolveInput{Project: project}, true)
			if err != nil {
				return err
			}
			if err := requireValue("project", resolved.Project); err != nil {
				return err
			}
			projectID, err := resolveProjectRouteIDFromConfig(app, resolved.Project, resolved.Profile)
			if err != nil {
				return err
			}

			resp, err := client.ListExecutionNotes(cmd.Context(), projectID, strings.TrimSpace(args[0]), page, pageSize)
			if err != nil {
				return classifyAPIError(err, "failed to list execution notes")
			}
			if printer.JSON {
				return printer.EmitJSON(resp)
			}

			fmt.Printf("Notes: %d\n", resp.TotalCount)
			for _, note := range resp.Items {
				fmt.Printf("- %s createdAt=%s\n", note.ID, note.CreatedAt.Format("2006-01-02T15:04:05Z07:00"))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&pageSize, "page-size", 20, "Page size")
	return cmd
}

func newExecutionsTestcasesCommand(app *App) *cobra.Command {
	var project string
	var page int
	var pageSize int

	cmd := &cobra.Command{
		Use:   "testcases <execution-id>",
		Short: "List test cases associated with an execution",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, client, printer, err := app.ResolveRuntime(config.ResolveInput{Project: project}, true)
			if err != nil {
				return err
			}
			if err := requireValue("project", resolved.Project); err != nil {
				return err
			}
			projectID, err := resolveProjectRouteIDFromConfig(app, resolved.Project, resolved.Profile)
			if err != nil {
				return err
			}

			resp, err := client.ListExecutionTestCases(cmd.Context(), projectID, strings.TrimSpace(args[0]), page, pageSize)
			if err != nil {
				return classifyAPIError(err, "failed to list execution test cases")
			}
			if printer.JSON {
				return printer.EmitJSON(resp)
			}

			fmt.Printf("Execution test cases: %d\n", resp.TotalCount)
			for _, testCase := range resp.Items {
				fmt.Printf("- %s testcaseId=%s status=%s severity=%s\n",
					testCase.ID,
					testCase.TestCaseID,
					testCase.Status,
					testCase.Severity,
				)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&pageSize, "page-size", 20, "Page size")
	return cmd
}

func newExecutionsConversationCommand(app *App) *cobra.Command {
	var project string
	var before string
	var page int
	var pageSize int

	cmd := &cobra.Command{
		Use:   "conversation <execution-id>",
		Short: "List conversation events for an execution's agent session",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, client, printer, err := app.ResolveRuntime(config.ResolveInput{Project: project}, true)
			if err != nil {
				return err
			}
			if err := requireValue("project", resolved.Project); err != nil {
				return err
			}
			projectID, err := resolveProjectRouteIDFromConfig(app, resolved.Project, resolved.Profile)
			if err != nil {
				return err
			}

			executionID := strings.TrimSpace(args[0])
			execution, err := client.GetExecution(cmd.Context(), projectID, executionID)
			if err != nil {
				return classifyAPIError(err, "failed to get execution")
			}

			sessionID := strings.TrimSpace(ptrString(execution.AgentSessionID))
			if sessionID == "" {
				return notFoundError(
					fmt.Sprintf("execution '%s' has no agent session conversation", executionID),
					nil,
				)
			}

			var beforeTime *time.Time
			if strings.TrimSpace(before) != "" {
				parsed, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(before))
				if parseErr != nil {
					return usageError("invalid --before: expected RFC3339 timestamp", parseErr)
				}
				beforeTime = &parsed
			}

			resp, err := client.ListAgentSessionConversation(cmd.Context(), projectID, sessionID, api.ListConversationEventsParams{
				Before:   beforeTime,
				Page:     page,
				PageSize: pageSize,
			})
			if err != nil {
				return classifyAPIError(err, "failed to list execution conversation")
			}

			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"execution_id":     executionID,
					"agent_session_id": sessionID,
					"items":            resp.Items,
					"totalCount":       resp.TotalCount,
					"page":             resp.Page,
					"pageSize":         resp.PageSize,
					"totalPages":       resp.TotalPages,
					"hasNextPage":      resp.HasNextPage,
					"hasPreviousPage":  resp.HasPrevPage,
				})
			}

			fmt.Printf("Execution: %s\n", executionID)
			fmt.Printf("Agent session: %s\n", sessionID)
			fmt.Printf("Conversation events: %d\n", resp.TotalCount)
			for _, event := range resp.Items {
				fmt.Printf("- %s %-24s %s\n",
					event.TimestampUTC.Format(time.RFC3339),
					event.Type,
					compactJSONForDisplay(event.Payload),
				)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&before, "before", "", "Return events before this RFC3339 timestamp")
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&pageSize, "page-size", 500, "Page size (max 1000)")

	return cmd
}

func newDiagnoseCommand(app *App) *cobra.Command {
	return newDiagnoseExecutionCommand(app)
}

func newExecutionsDiagnoseCommand(app *App) *cobra.Command {
	return newDiagnoseExecutionCommand(app)
}

func newDiagnoseExecutionCommand(app *App) *cobra.Command {
	var project string
	var before string
	var page int
	var pageSize int
	var maxEvents int
	var sampleSize int

	cmd := &cobra.Command{
		Use:   "diagnose <execution-id>",
		Short: "Produce compact failure diagnostics for an execution conversation",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, client, printer, err := app.ResolveRuntime(config.ResolveInput{Project: project}, true)
			if err != nil {
				return err
			}
			if err := requireValue("project", resolved.Project); err != nil {
				return err
			}
			projectID, err := resolveProjectRouteIDFromConfig(app, resolved.Project, resolved.Profile)
			if err != nil {
				return err
			}

			if maxEvents <= 0 {
				return usageError("invalid --max-events: must be greater than zero", nil)
			}
			if sampleSize < 0 {
				return usageError("invalid --sample-size: must be zero or greater", nil)
			}
			if pageSize <= 0 {
				return usageError("invalid --page-size: must be greater than zero", nil)
			}

			executionID := strings.TrimSpace(args[0])
			execution, err := client.GetExecution(cmd.Context(), projectID, executionID)
			if err != nil {
				return classifyAPIError(err, "failed to get execution")
			}

			var beforeTime *time.Time
			if strings.TrimSpace(before) != "" {
				parsed, parseErr := time.Parse(time.RFC3339, strings.TrimSpace(before))
				if parseErr != nil {
					return usageError("invalid --before: expected RFC3339 timestamp", parseErr)
				}
				beforeTime = &parsed
			}

			diagnosticResult, err := collectExecutionDiagnostics(
				cmd.Context(),
				client,
				projectID,
				strings.TrimSpace(resolved.Project),
				executionDiagnosticSubject{
					ExecutionID:    executionID,
					AgentSessionID: strings.TrimSpace(ptrString(execution.AgentSessionID)),
					Status:         strings.TrimSpace(execution.Status),
				},
				executionDiagnosticsOptions{
					Before:     beforeTime,
					Page:       page,
					PageSize:   pageSize,
					MaxEvents:  maxEvents,
					SampleSize: sampleSize,
				},
			)
			if err != nil {
				return err
			}

			payload := executionDiagnoseOutput{
				SchemaVersion:               executionDiagnoseSchemaVersion,
				ExecutionID:                 diagnosticResult.Diagnostic.ExecutionID,
				AgentSessionID:              diagnosticResult.Diagnostic.AgentSessionID,
				EventWindow:                 diagnosticResult.EventWindow,
				PrimaryFailureReason:        diagnosticResult.Diagnostic.PrimaryFailureReason,
				FailureSummary:              diagnosticResult.Diagnostic.FailureSummary,
				ConversationCompletedStatus: diagnosticResult.Diagnostic.ConversationCompletedStatus,
				Counts:                      diagnosticResult.Diagnostic.Counts,
				NetworkFailures:             diagnosticResult.Diagnostic.NetworkFailures,
				ToolFailures:                diagnosticResult.Diagnostic.ToolFailures,
				ConversationCommand:         diagnosticResult.Diagnostic.ConversationCommand,
			}

			if printer.JSON {
				return printer.EmitJSON(payload)
			}

			fmt.Printf("Execution: %s\n", payload.ExecutionID)
			fmt.Printf("Agent session: %s\n", valueOrDash(payload.AgentSessionID))
			fmt.Printf("Reason: %s\n", valueOrDash(payload.PrimaryFailureReason))
			fmt.Printf("Summary: %s\n", valueOrDash(payload.FailureSummary))
			if payload.ConversationCompletedStatus != "" {
				fmt.Printf("Conversation status: %s\n", payload.ConversationCompletedStatus)
			}
			fmt.Printf(
				"Counts: tool_errors=%d policy_denied=%d network_4xx=%d network_5xx=%d network_total=%d\n",
				payload.Counts.ToolErrors,
				payload.Counts.PolicyDenied,
				payload.Counts.Network4xx,
				payload.Counts.Network5xx,
				payload.Counts.NetworkTotal,
			)
			fmt.Printf(
				"Event window: scanned=%d truncated=%t has_more=%t",
				payload.EventWindow.Scanned,
				payload.EventWindow.Truncated,
				payload.EventWindow.HasMore,
			)
			if payload.EventWindow.NextPage > 0 {
				fmt.Printf(" next_page=%d", payload.EventWindow.NextPage)
			}
			fmt.Println()
			if len(payload.NetworkFailures) > 0 {
				fmt.Println("Network failures:")
				for _, failure := range payload.NetworkFailures {
					fmt.Printf(
						"- status=%d method=%s url=%s requestId=%s\n",
						failure.Status,
						valueOrDash(failure.Method),
						valueOrDash(failure.URL),
						valueOrDash(failure.RequestID),
					)
				}
			}
			if len(payload.ToolFailures) > 0 {
				fmt.Println("Tool failures:")
				for _, failure := range payload.ToolFailures {
					fmt.Printf(
						"- tool=%s status=%s call=%s error=%s\n",
						valueOrDash(failure.ToolName),
						valueOrDash(failure.Status),
						valueOrDash(failure.ToolCallID),
						valueOrDash(firstNonEmpty(failure.Error, failure.Output)),
					)
				}
			}
			if payload.ConversationCommand != "" {
				fmt.Printf("Conversation: %s\n", payload.ConversationCommand)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&before, "before", "", "Return events before this RFC3339 timestamp")
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&pageSize, "page-size", 500, "Page size (max 1000)")
	cmd.Flags().IntVar(&maxEvents, "max-events", 1000, "Maximum events to scan")
	cmd.Flags().IntVar(&sampleSize, "sample-size", 10, "Maximum failure samples to include")

	return cmd
}

func compactJSONForDisplay(raw []byte) string {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "-"
	}

	var compact bytes.Buffer
	if err := json.Compact(&compact, []byte(trimmed)); err != nil {
		return trimmed
	}
	return compact.String()
}

func ptrString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func newExecutionsRetryCommand(app *App) *cobra.Command {
	var project string

	cmd := &cobra.Command{
		Use:   "retry <execution-id>",
		Short: "Retry an execution",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, client, printer, err := app.ResolveRuntime(config.ResolveInput{Project: project}, true)
			if err != nil {
				return err
			}
			if err := requireValue("project", resolved.Project); err != nil {
				return err
			}
			projectID, err := resolveProjectRouteIDFromConfig(app, resolved.Project, resolved.Profile)
			if err != nil {
				return err
			}

			previousExecutionID := strings.TrimSpace(args[0])
			execution, err := client.RetryExecution(cmd.Context(), projectID, previousExecutionID)
			if err != nil {
				return classifyAPIError(err, "failed to retry execution")
			}
			if printer.JSON {
				return printer.EmitJSON(execution)
			}

			fmt.Printf("Retried execution %s\n", previousExecutionID)
			fmt.Printf("New execution ID: %s\n", execution.ID)
			fmt.Printf("Status: %s\n", execution.Status)
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	return cmd
}

func newExecutionsStopCommand(app *App) *cobra.Command {
	var project string

	cmd := &cobra.Command{
		Use:   "stop <execution-id>",
		Short: "Stop an execution",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, client, printer, err := app.ResolveRuntime(config.ResolveInput{Project: project}, true)
			if err != nil {
				return err
			}
			if err := requireValue("project", resolved.Project); err != nil {
				return err
			}
			projectID, err := resolveProjectRouteIDFromConfig(app, resolved.Project, resolved.Profile)
			if err != nil {
				return err
			}

			executionID := strings.TrimSpace(args[0])
			if err := client.StopExecution(cmd.Context(), projectID, executionID); err != nil {
				return classifyAPIError(err, "failed to stop execution")
			}
			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"project_id":   projectID,
					"execution_id": executionID,
					"stopped":      true,
				})
			}

			fmt.Printf("Stopped execution %s\n", executionID)
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	return cmd
}
