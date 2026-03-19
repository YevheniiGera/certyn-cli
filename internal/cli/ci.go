package cli

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"time"

	"github.com/certyn/certyn-cli/internal/api"
	"github.com/certyn/certyn-cli/internal/config"
	"github.com/certyn/certyn-cli/internal/output"
	"github.com/spf13/cobra"
)

var processAliases = map[string]string{
	"smoke":      "smoke-suite",
	"regression": "regression-suite",
	"explore":    "app-explorer",
}

func newCICommand(app *App) *cobra.Command {
	ciCmd := &cobra.Command{
		Use:   "ci",
		Short: "CI run operations",
	}

	ciCmd.AddCommand(newCIRunCommand(app))
	ciCmd.AddCommand(newCIStatusCommand(app))
	ciCmd.AddCommand(newCIWaitCommand(app))
	ciCmd.AddCommand(newCICancelCommand(app))
	ciCmd.AddCommand(newCIListCommand(app))

	return ciCmd
}

func newCIRunCommand(app *App) *cobra.Command {
	var project string
	var environment string
	var wait bool
	var timeout time.Duration
	var idempotencyKey string
	var repository string
	var ref string
	var sha string
	var eventName string
	var externalURL string

	cmd := &cobra.Command{
		Use:   "run <process-or-alias>",
		Short: "Trigger a CI run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, client, printer, err := app.ResolveRuntime(config.ResolveInput{Project: project, Environment: environment}, true)
			if err != nil {
				return err
			}

			if err := requireValue("project", resolved.Project); err != nil {
				return err
			}
			if err := requireValue("environment", resolved.Environment); err != nil {
				return err
			}

			// CI run API accepts project slug and environment key directly.
			// Avoid extra project/environment resolution calls so CI-scoped keys work.
			projectSlug := strings.TrimSpace(resolved.Project)
			envKey := strings.TrimSpace(resolved.Environment)

			processSlug := resolveProcessSlug(args[0])
			keyValue := strings.TrimSpace(idempotencyKey)
			if keyValue == "" || strings.EqualFold(keyValue, "auto") {
				idempotencyKey = generateID()
			} else {
				idempotencyKey = keyValue
			}

			resp, err := client.CreateCiRun(cmd.Context(), api.CreateCiRunRequest{
				ProjectSlug:    projectSlug,
				EnvironmentKey: envKey,
				ProcessSlug:    processSlug,
				Repository:     repository,
				Ref:            ref,
				CommitSHA:      sha,
				Event:          eventName,
				ExternalURL:    externalURL,
			}, idempotencyKey)
			if err != nil {
				return classifyAPIError(err, "failed to create CI run")
			}

			if err := output.WriteGitHubOutputs(map[string]string{
				"run_id":     resp.RunID,
				"status_url": resp.StatusURL,
				"app_url":    resp.AppURL,
			}); err != nil {
				return &CommandError{Code: ExitGateFailed, Message: "failed to write GitHub outputs", Err: err}
			}

			if wait {
				status, code, err := executeWait(cmd.Context(), client, resp.RunID, timeout, 0, true)
				if status != nil {
					emitStatusOutputs(*status, resp.StatusURL)
				}
				if printer.JSON {
					jsonErr := printer.EmitJSON(map[string]any{
						"run":    resp,
						"status": status,
					})
					if jsonErr != nil {
						return jsonErr
					}
				}
				if err != nil {
					return err
				}
				if code != ExitOK {
					return &CommandError{Code: code, Message: "CI run failed quality gate"}
				}
				if printer.JSON {
					return nil
				}
				printLegacyCIStatus(status, resp.StatusURL)
				return nil
			}

			if printer.JSON {
				return printer.EmitJSON(resp)
			}

			st := output.NewStyler()
			printHumanHeader(st, "ok", "CI run created")
			printHumanField(st, "run id", resp.RunID)
			printHumanField(st, "status url", resp.StatusURL)
			if resp.AppURL != "" {
				printHumanField(st, "app url", resp.AppURL)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug")
	cmd.Flags().StringVar(&environment, "environment", "", "Environment key")
	cmd.Flags().BoolVar(&wait, "wait", false, "Wait for run completion")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "Wait timeout")
	cmd.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "Idempotency key (or 'auto')")
	cmd.Flags().StringVar(&repository, "repo", "", "Repository name")
	cmd.Flags().StringVar(&ref, "ref", "", "Git ref")
	cmd.Flags().StringVar(&sha, "sha", "", "Commit SHA")
	cmd.Flags().StringVar(&eventName, "event", "", "Trigger event")
	cmd.Flags().StringVar(&externalURL, "external-url", "", "External CI URL")

	return cmd
}

func newCIStatusCommand(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status <run-id>",
		Short: "Get CI run status",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, client, printer, err := app.ResolveRuntime(config.ResolveInput{}, true)
			if err != nil {
				return err
			}

			status, err := client.GetCiRunStatus(cmd.Context(), args[0])
			if err != nil {
				return classifyAPIError(err, "failed to fetch CI run status")
			}

			emitStatusOutputs(*status, client.StatusURL(status.RunID))

			if printer.JSON {
				return printer.EmitJSON(status)
			}

			printLegacyCIStatus(status, client.StatusURL(status.RunID))
			return nil
		},
	}
	return cmd
}

func newCIWaitCommand(app *App) *cobra.Command {
	var timeout time.Duration
	var pollInterval time.Duration
	var cancelOnInterrupt bool

	cmd := &cobra.Command{
		Use:   "wait <run-id>",
		Short: "Wait for CI run completion",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, client, printer, err := app.ResolveRuntime(config.ResolveInput{}, true)
			if err != nil {
				return err
			}

			status, code, err := executeWait(cmd.Context(), client, args[0], timeout, pollInterval, cancelOnInterrupt)
			if status != nil {
				emitStatusOutputs(*status, client.StatusURL(status.RunID))
			}

			if printer.JSON {
				if jsonErr := emitCIWaitJSON(printer, args[0], status, code, err); jsonErr != nil {
					return jsonErr
				}
			}

			if err != nil {
				return err
			}

			if status != nil && !printer.JSON {
				printLegacyCIStatus(status, client.StatusURL(status.RunID))
			}

			if code != ExitOK {
				return &CommandError{Code: code, Message: "CI run failed quality gate"}
			}
			return nil
		},
	}

	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "Wait timeout")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", 0, "Fixed polling interval (default uses retryAfterSeconds)")
	cmd.Flags().BoolVar(&cancelOnInterrupt, "cancel-on-interrupt", true, "Cancel run when interrupted")

	return cmd
}

func newCICancelCommand(app *App) *cobra.Command {
	var reason string

	cmd := &cobra.Command{
		Use:   "cancel <run-id>",
		Short: "Cancel a CI run",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, client, printer, err := app.ResolveRuntime(config.ResolveInput{}, true)
			if err != nil {
				return err
			}

			if err := client.CancelCiRun(cmd.Context(), args[0], reason); err != nil {
				return classifyAPIError(err, "failed to cancel run")
			}

			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"run_id":    args[0],
					"cancelled": true,
					"reason":    reason,
				})
			}

			st := output.NewStyler()
			printHumanHeader(st, "ok", "CI run cancelled")
			printHumanField(st, "run id", args[0])
			printHumanField(st, "reason", valueOrDash(reason))
			return nil
		},
	}

	cmd.Flags().StringVar(&reason, "reason", "", "Cancellation reason")
	return cmd
}

func newCIListCommand(app *App) *cobra.Command {
	var project string
	var take int
	var skip int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List CI runs for a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, client, printer, err := app.ResolveRuntime(config.ResolveInput{Project: project}, true)
			if err != nil {
				return err
			}
			if err := requireValue("project", resolved.Project); err != nil {
				return err
			}

			projectIdentifier := strings.TrimSpace(resolved.Project)
			resp, err := client.ListCiRuns(cmd.Context(), projectIdentifier, take, skip)

			// Backend list endpoint expects project id. If a slug is passed and there are no runs,
			// endpoint can return 200 + empty list instead of 404. Attempt one slug->id resolution
			// in that case to avoid silent false-empty responses.
			if err == nil && resp != nil && resp.TotalCount == 0 && !looksLikeProjectID(projectIdentifier) {
				projectID, resolveErr := resolveProjectID(cmd.Context(), client, projectIdentifier)
				if resolveErr != nil {
					if !isAuthAPIError(resolveErr) {
						return resolveErr
					}
					// Keep original empty result when CI-scoped keys cannot resolve slug->id.
					// Initial list call already succeeded.
				}

				if projectID != "" && !strings.EqualFold(projectID, projectIdentifier) {
					resp, err = client.ListCiRuns(cmd.Context(), projectID, take, skip)
				}
			}

			if err != nil {
				var apiErr *api.APIError
				shouldResolve := errors.As(err, &apiErr) && (apiErr.StatusCode == 400 || apiErr.StatusCode == 404)
				if shouldResolve {
					projectID, resolveErr := resolveProjectID(cmd.Context(), client, projectIdentifier)
					if resolveErr != nil {
						var resolveAPIError *api.APIError
						if errors.As(resolveErr, &resolveAPIError) && (resolveAPIError.StatusCode == 401 || resolveAPIError.StatusCode == 403) {
							return authError(
								"unable to resolve project slug with this API key; pass --project as project id or use a key with project read access",
								resolveErr,
							)
						}
						return resolveErr
					}
					resp, err = client.ListCiRuns(cmd.Context(), projectID, take, skip)
				}
			}
			if err != nil {
				return classifyAPIError(err, "failed to list CI runs")
			}

			if printer.JSON {
				return printer.EmitJSON(resp)
			}

			st := output.NewStyler()
			printHumanHeader(st, "info", fmt.Sprintf("CI runs (%d)", resp.TotalCount))
			for _, item := range resp.Items {
				printHumanItem(st, humanKVSummary(
					item.RunID,
					st.Status(item.State),
					st.Status(item.Conclusion),
					fmt.Sprintf("failed %d", item.Failed),
					fmt.Sprintf("blocked %d", item.Blocked),
				))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project id or slug")
	cmd.Flags().IntVar(&take, "take", 50, "Number of runs to fetch")
	cmd.Flags().IntVar(&skip, "skip", 0, "Number of runs to skip")

	return cmd
}

func executeWait(baseCtx context.Context, client *api.Client, runID string, timeout, pollInterval time.Duration, cancelOnInterrupt bool) (*api.CiRunStatusResponse, int, error) {
	ctx := baseCtx
	cancel := func() {}
	if timeout > 0 {
		ctx, cancel = context.WithTimeout(baseCtx, timeout)
	}
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)
	defer signal.Stop(sigCh)

	for {
		status, err := client.GetCiRunStatus(ctx, runID)
		if err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, ExitTimeout, timeoutError("timed out waiting for run completion", ctx.Err())
			}
			return nil, ExitGateFailed, classifyAPIError(err, "failed while polling CI run")
		}

		if status.State == "completed" || status.State == "cancelled" {
			if status.State == "cancelled" {
				return status, ExitGateFailed, nil
			}
			if status.Failed > 0 || status.Blocked > 0 {
				return status, ExitGateFailed, nil
			}
			return status, ExitOK, nil
		}

		interval := pollInterval
		if interval <= 0 {
			retrySeconds := status.RetryAfterSeconds
			if retrySeconds <= 0 {
				retrySeconds = 15
			}
			interval = time.Duration(retrySeconds) * time.Second
		}

		timer := time.NewTimer(interval)
		select {
		case <-ctx.Done():
			timer.Stop()
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return nil, ExitTimeout, timeoutError("timed out waiting for run completion", ctx.Err())
			}
			return nil, ExitInterrupted, &CommandError{Code: ExitInterrupted, Message: "interrupted"}
		case <-sigCh:
			timer.Stop()
			if cancelOnInterrupt {
				cancelCtx, cancelFunc := context.WithTimeout(context.Background(), 5*time.Second)
				_ = client.CancelCiRun(cancelCtx, runID, "Canceled by interrupt")
				cancelFunc()
			}
			return nil, ExitInterrupted, &CommandError{Code: ExitInterrupted, Message: "interrupted"}
		case <-timer.C:
		}
	}
}

func emitCIWaitJSON(printer output.Printer, runID string, status *api.CiRunStatusResponse, code int, waitErr error) error {
	if status != nil {
		return printer.EmitJSON(status)
	}

	payload := map[string]any{
		"run_id":    strings.TrimSpace(runID),
		"exit_code": code,
	}
	if waitErr != nil {
		payload["error"] = waitErr.Error()
	}

	return printer.EmitJSON(payload)
}

func resolveProcessSlug(value string) string {
	normalized := strings.TrimSpace(strings.ToLower(value))
	if mapped, ok := processAliases[normalized]; ok {
		return mapped
	}
	return strings.TrimSpace(value)
}

func emitStatusOutputs(status api.CiRunStatusResponse, statusURL string) {
	_ = output.WriteGitHubOutputs(map[string]string{
		"run_id":     status.RunID,
		"status_url": statusURL,
		"state":      status.State,
		"conclusion": status.Conclusion,
		"total":      strconv.Itoa(status.Total),
		"passed":     strconv.Itoa(status.Passed),
		"failed":     strconv.Itoa(status.Failed),
		"blocked":    strconv.Itoa(status.Blocked),
		"pending":    strconv.Itoa(status.Pending),
		"aborted":    strconv.Itoa(status.Aborted),
		"app_url":    status.AppURL,
	})
}

func printLegacyCIStatus(status *api.CiRunStatusResponse, statusURL string) {
	if status == nil {
		return
	}
	st := output.NewStyler()
	kind := "info"
	title := "CI run status"
	switch strings.ToLower(strings.TrimSpace(status.Conclusion)) {
	case "passed":
		kind = "ok"
		title = "CI run passed"
	case "failed", "blocked", "aborted", "cancelled", "canceled":
		kind = "fail"
		title = "CI run needs attention"
	}
	printHumanHeader(st, kind, title)
	printHumanField(st, "run id", status.RunID)
	printHumanField(st, "state", st.Status(status.State))
	printHumanField(st, "conclusion", st.Status(status.Conclusion))
	printHumanField(st, "totals", fmt.Sprintf("%d passed, %d failed, %d blocked, %d pending, %d aborted", status.Passed, status.Failed, status.Blocked, status.Pending, status.Aborted))
	if statusURL != "" {
		printHumanField(st, "status url", statusURL)
	}
	if status.AppURL != "" {
		printHumanField(st, "app url", status.AppURL)
	}
}

func generateID() string {
	bytes := make([]byte, 16)
	if _, err := rand.Read(bytes); err != nil {
		return fmt.Sprintf("fallback-%d", time.Now().UnixNano())
	}
	// RFC 4122 v4 UUID variant.
	bytes[6] = (bytes[6] & 0x0f) | 0x40
	bytes[8] = (bytes[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", bytes[0:4], bytes[4:6], bytes[6:8], bytes[8:10], bytes[10:16])
}

func looksLikeProjectID(value string) bool {
	id := strings.TrimSpace(value)
	if id == "" {
		return false
	}
	return looksLikeUUID(id) || looksLikeULID(id)
}

func looksLikeUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, ch := range value {
		switch i {
		case 8, 13, 18, 23:
			if ch != '-' {
				return false
			}
		default:
			if !((ch >= '0' && ch <= '9') || (ch >= 'a' && ch <= 'f') || (ch >= 'A' && ch <= 'F')) {
				return false
			}
		}
	}
	return true
}

func looksLikeULID(value string) bool {
	if len(value) != 26 {
		return false
	}
	for _, ch := range value {
		if ch >= '0' && ch <= '9' {
			continue
		}
		if (ch >= 'A' && ch <= 'Z') || (ch >= 'a' && ch <= 'z') {
			continue
		}
		return false
	}
	return true
}

func isAuthAPIError(err error) bool {
	var apiErr *api.APIError
	return errors.As(err, &apiErr) && (apiErr.StatusCode == 401 || apiErr.StatusCode == 403)
}
