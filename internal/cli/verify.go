package cli

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/certyn/certyn-cli/internal/api"
	"github.com/certyn/certyn-cli/internal/config"
	"github.com/certyn/certyn-cli/internal/output"
	"github.com/spf13/cobra"
)

var nonAlphaNumericPattern = regexp.MustCompile(`[^a-z0-9]+`)

type verifyOutput struct {
	SchemaVersion         string                   `json:"schema_version"`
	Mode                  string                   `json:"mode"`
	Suite                 string                   `json:"suite,omitempty"`
	ProcessSlug           string                   `json:"process_slug,omitempty"`
	Tags                  []string                 `json:"tags,omitempty"`
	URL                   string                   `json:"url"`
	EnvironmentMode       string                   `json:"environment_mode,omitempty"`
	EnvironmentKey        string                   `json:"environment_key,omitempty"`
	EnvironmentDeleted    bool                     `json:"environment_deleted"`
	RunID                 string                   `json:"run_id,omitempty"`
	StatusURL             string                   `json:"status_url,omitempty"`
	AppURL                string                   `json:"app_url,omitempty"`
	State                 string                   `json:"state,omitempty"`
	Conclusion            string                   `json:"conclusion,omitempty"`
	Total                 int                      `json:"total"`
	Passed                int                      `json:"passed"`
	Failed                int                      `json:"failed"`
	Blocked               int                      `json:"blocked"`
	Pending               int                      `json:"pending"`
	Aborted               int                      `json:"aborted"`
	ExecutionTotal        int                      `json:"execution_total,omitempty"`
	ExecutionPassed       int                      `json:"execution_passed,omitempty"`
	ExecutionFailed       int                      `json:"execution_failed,omitempty"`
	ExecutionBlocked      int                      `json:"execution_blocked,omitempty"`
	ExecutionRunning      int                      `json:"execution_running,omitempty"`
	ExecutionPending      int                      `json:"execution_pending,omitempty"`
	Executions            []verifyExecutionSummary `json:"executions,omitempty"`
	ExecutionDetailsError string                   `json:"execution_details_error,omitempty"`
	DiagnosticsCollected  bool                     `json:"diagnostics_collected"`
	Diagnostics           []executionDiagnostic    `json:"diagnostics,omitempty"`
	DiagnosticsErrors     []verifyDiagnosticError  `json:"diagnostics_errors,omitempty"`
	ExitCode              int                      `json:"exit_code"`
	Error                 string                   `json:"error,omitempty"`
}

type verifyExecutionSummary struct {
	ExecutionID         string     `json:"execution_id,omitempty"`
	ProcessRunItemID    string     `json:"process_run_item_id,omitempty"`
	TestCaseID          string     `json:"test_case_id,omitempty"`
	TestCaseName        string     `json:"test_case_name,omitempty"`
	TestCaseNumber      int        `json:"test_case_number,omitempty"`
	Status              string     `json:"status,omitempty"`
	Outcome             string     `json:"outcome,omitempty"`
	AgentSessionID      string     `json:"agent_session_id,omitempty"`
	StartedAt           *time.Time `json:"started_at,omitempty"`
	CompletedAt         *time.Time `json:"completed_at,omitempty"`
	ConversationCommand string     `json:"conversation_command,omitempty"`
}

func newVerifyCommand(app *App) *cobra.Command {
	var project string
	var suite string
	var tags []string
	var targetURL string
	var timeout time.Duration
	var pollInterval time.Duration
	var keepEnv bool
	var repository string
	var ref string
	var sha string
	var eventName string
	var externalURL string
	var diagnoseFailed bool
	var diagnosticsMaxEvents int
	var diagnosticsSampleSize int

	cmd := &cobra.Command{
		Use:   "verify",
		Short: "Run local pre-PR verification against an ephemeral URL or existing environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, client, printer, err := app.ResolveRuntime(config.ResolveInput{Project: project}, true)
			if err != nil {
				return err
			}
			if err := requireValue("project", resolved.Project); err != nil {
				return err
			}
			effectiveEnvironment := strings.TrimSpace(resolved.Environment)
			explicitEnvironment := cmd.Flags().Changed("environment")
			if strings.TrimSpace(targetURL) != "" {
				if explicitEnvironment && effectiveEnvironment != "" {
					return usageError("conflicting targets: use either --url or --environment, not both", nil)
				}
				// URL mode overrides implicit environment defaults from profile/env.
				effectiveEnvironment = ""
			}

			if strings.TrimSpace(targetURL) == "" && effectiveEnvironment == "" {
				return usageError("missing target: provide --url for preview mode or --environment for existing-environment mode", nil)
			}

			output, runErr := runVerify(cmd.Context(), app, client, resolved, verifyInput{
				Suite:                 suite,
				Tags:                  tags,
				URL:                   targetURL,
				Environment:           effectiveEnvironment,
				Timeout:               timeout,
				PollInterval:          pollInterval,
				KeepEnv:               keepEnv,
				Repository:            repository,
				Ref:                   ref,
				SHA:                   sha,
				Event:                 eventName,
				ExternalURL:           externalURL,
				DiagnoseFailed:        diagnoseFailed,
				DiagnosticsMaxEvents:  diagnosticsMaxEvents,
				DiagnosticsSampleSize: diagnosticsSampleSize,
			})

			if printer.JSON {
				if emitErr := printer.EmitJSON(output); emitErr != nil {
					return emitErr
				}
				return runErr
			}

			printVerifyHumanSummary(output, keepEnv)
			return runErr
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&suite, "suite", "smoke", "Suite/process to run when no --tag is provided: smoke, regression, or a process slug")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Feature tag to run (repeatable)")
	cmd.Flags().StringVar(&targetURL, "url", "", "Public application URL (ephemeral mode; optional when --environment is provided)")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "Wait timeout")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", 0, "Fixed polling interval (default uses retryAfterSeconds)")
	cmd.Flags().BoolVar(&keepEnv, "keep-env", false, "Keep ephemeral environment after verification")
	cmd.Flags().BoolVar(&diagnoseFailed, "diagnose-failed", true, "Collect failure diagnostics for failed executions when verify gate fails")
	cmd.Flags().IntVar(&diagnosticsMaxEvents, "diagnostics-max-events", 1000, "Maximum conversation events to scan per failed execution")
	cmd.Flags().IntVar(&diagnosticsSampleSize, "diagnostics-sample-size", 10, "Maximum network/tool failure samples per failed execution")
	cmd.Flags().StringVar(&repository, "repo", "", "Repository name")
	cmd.Flags().StringVar(&ref, "ref", "", "Git ref")
	cmd.Flags().StringVar(&sha, "sha", "", "Commit SHA")
	cmd.Flags().StringVar(&eventName, "event", "", "Trigger event")
	cmd.Flags().StringVar(&externalURL, "external-url", "", "External CI URL")

	return cmd
}

type verifyInput struct {
	Suite                 string
	Tags                  []string
	URL                   string
	Environment           string
	Timeout               time.Duration
	PollInterval          time.Duration
	KeepEnv               bool
	Repository            string
	Ref                   string
	SHA                   string
	Event                 string
	ExternalURL           string
	DiagnoseFailed        bool
	DiagnosticsMaxEvents  int
	DiagnosticsSampleSize int
}

func runVerify(
	ctx context.Context,
	app *App,
	client *api.Client,
	resolved config.Runtime,
	input verifyInput,
) (result verifyOutput, runErr error) {
	result = verifyOutput{SchemaVersion: runSchemaVersion, ExitCode: ExitOK}

	modeURL := strings.TrimSpace(input.URL)
	modeEnvironment := strings.TrimSpace(input.Environment)
	if modeURL == "" && modeEnvironment == "" {
		runErr = usageError("missing target: provide --url for preview mode or --environment for existing-environment mode", nil)
		applyVerifyError(&result, runErr, ExitUsage)
		return result, runErr
	}
	if modeURL != "" && modeEnvironment != "" {
		runErr = usageError("conflicting targets: use either --url or --environment, not both", nil)
		applyVerifyError(&result, runErr, ExitUsage)
		return result, runErr
	}

	if modeURL != "" {
		normalizedURL, err := normalizeVerifyURL(modeURL)
		if err != nil {
			result.URL = modeURL
			runErr = err
			applyVerifyError(&result, runErr, ExitUsage)
			return result, runErr
		}
		result.URL = normalizedURL
		result.EnvironmentMode = "ephemeral"
		if err := preflightVerifyURL(ctx, normalizedURL); err != nil {
			runErr = err
			applyVerifyError(&result, runErr, ExitUsage)
			return result, runErr
		}
	} else {
		result.EnvironmentMode = "existing"
		result.EnvironmentKey = modeEnvironment
	}
	if input.DiagnosticsMaxEvents <= 0 {
		runErr = usageError("invalid --diagnostics-max-events: must be greater than zero", nil)
		applyVerifyError(&result, runErr, ExitUsage)
		return result, runErr
	}
	if input.DiagnosticsSampleSize < 0 {
		runErr = usageError("invalid --diagnostics-sample-size: must be zero or greater", nil)
		applyVerifyError(&result, runErr, ExitUsage)
		return result, runErr
	}

	tags := nonEmptyValues(input.Tags)
	processIdentifier := ""
	if len(tags) > 0 {
		result.Mode = "tags"
		result.Tags = tags
	} else {
		normalizedSuite, identifier := normalizeVerifySuiteInput(input.Suite)
		result.Mode = "suite"
		result.Suite = normalizedSuite
		processIdentifier = identifier
	}

	cfg, err := app.ConfigManager()
	if err != nil {
		runErr = err
		applyVerifyError(&result, runErr, ExitUsage)
		return result, runErr
	}

	projectID, projectSlug, err := resolveProjectIDAndSlug(ctx, client, resolved.Project)
	if err != nil {
		runErr = err
		applyVerifyError(&result, runErr, ExitGateFailed)
		return result, runErr
	}
	if err := storeProjectMapping(cfg, resolved.Profile, projectSlug, projectID); err != nil {
		runErr = err
		applyVerifyError(&result, runErr, ExitUsage)
		return result, runErr
	}

	if result.Mode == "suite" {
		resolvedProcessSlug, resolveErr := resolveVerifyProcessSlug(ctx, client, projectID, processIdentifier)
		if resolveErr != nil {
			runErr = resolveErr
			applyVerifyError(&result, runErr, ExitUsage)
			return result, runErr
		}
		result.ProcessSlug = resolvedProcessSlug
	}

	if result.EnvironmentMode == "ephemeral" {
		envKey := generateVerifyEnvironmentKey(resolved.Profile)
		result.EnvironmentKey = envKey

		envInput := api.EnvironmentInput{
			Key:     envKey,
			Label:   fmt.Sprintf("Local Run %s", time.Now().UTC().Format("2006-01-02 15:04:05 UTC")),
			BaseURL: result.URL,
			Version: buildVerifyVersion(input.SHA),
		}
		env, err := client.CreateEnvironment(ctx, projectID, envInput)
		if err != nil {
			runErr = classifyVerifyEnvironmentError(err, "failed to create verification environment")
			applyVerifyError(&result, runErr, ExitGateFailed)
			return result, runErr
		}

		createdEnvID := strings.TrimSpace(env.ID)
		defer func() {
			if createdEnvID == "" || input.KeepEnv {
				return
			}

			deleteErr := client.DeleteEnvironment(context.Background(), projectID, createdEnvID)
			if deleteErr != nil {
				if runErr == nil {
					runErr = classifyVerifyEnvironmentError(deleteErr, "failed to delete verification environment")
					applyVerifyError(&result, runErr, ExitGateFailed)
				} else if result.Error == "" {
					result.Error = runErr.Error()
				}
				return
			}

			result.EnvironmentDeleted = true
		}()
	} else {
		_, envKey, resolveErr := resolveEnvironmentIDAndKey(
			ctx,
			client,
			projectID,
			modeEnvironment,
			projectSlug,
			resolved.Profile,
		)
		if resolveErr != nil {
			runErr = resolveErr
			applyVerifyError(&result, runErr, ExitUsage)
			return result, runErr
		}
		result.EnvironmentKey = envKey
	}

	ciRequest := api.CreateCiRunRequest{
		ProjectSlug:    projectSlug,
		EnvironmentKey: result.EnvironmentKey,
		Repository:     strings.TrimSpace(input.Repository),
		Ref:            strings.TrimSpace(input.Ref),
		CommitSHA:      strings.TrimSpace(input.SHA),
		Event:          strings.TrimSpace(input.Event),
		ExternalURL:    strings.TrimSpace(input.ExternalURL),
	}
	if result.Mode == "tags" {
		ciRequest.Tags = tags
	} else {
		ciRequest.ProcessSlug = result.ProcessSlug
	}

	createResp, err := client.CreateCiRun(ctx, ciRequest, generateID())
	if err != nil {
		runErr = classifyAPIError(err, "failed to create run")
		applyVerifyError(&result, runErr, ExitGateFailed)
		return result, runErr
	}

	result.RunID = createResp.RunID
	result.StatusURL = createResp.StatusURL
	result.AppURL = createResp.AppURL

	status, code, waitErr := executeWait(ctx, client, createResp.RunID, input.Timeout, input.PollInterval, true)
	if status != nil {
		result.State = status.State
		result.Conclusion = status.Conclusion
		result.Total = status.Total
		result.Passed = status.Passed
		result.Failed = status.Failed
		result.Blocked = status.Blocked
		result.Pending = status.Pending
		result.Aborted = status.Aborted
		if status.AppURL != "" {
			result.AppURL = status.AppURL
		}
		if result.StatusURL == "" {
			result.StatusURL = client.StatusURL(status.RunID)
		}
	}

	if detailsErr := attachVerifyExecutionDetails(ctx, client, projectID, projectSlug, createResp.RunID, &result); detailsErr != nil {
		result.ExecutionDetailsError = detailsErr.Error()
	}

	if waitErr != nil {
		runErr = waitErr
		result.ExitCode = code
		if result.ExitCode == ExitOK {
			result.ExitCode = exitCodeFromError(runErr, ExitGateFailed)
		}
		if result.Error == "" {
			result.Error = runErr.Error()
		}
		return result, runErr
	}

	if code != ExitOK {
		if input.DiagnoseFailed {
			attachVerifyFailureDiagnostics(ctx, client, projectID, projectSlug, &result, input)
		}
		runErr = &CommandError{Code: code, Message: "CI run failed quality gate"}
		applyVerifyError(&result, runErr, code)
		return result, runErr
	}

	result.ExitCode = ExitOK
	return result, nil
}

func attachVerifyFailureDiagnostics(
	ctx context.Context,
	client *api.Client,
	projectID,
	projectSlug string,
	result *verifyOutput,
	input verifyInput,
) {
	if result == nil || len(result.Executions) == 0 {
		return
	}

	targets := make([]verifyExecutionSummary, 0, len(result.Executions))
	for _, execution := range result.Executions {
		if shouldDiagnoseVerifyExecution(execution) {
			targets = append(targets, execution)
		}
	}
	if len(targets) == 0 {
		return
	}

	result.DiagnosticsCollected = true
	options := executionDiagnosticsOptions{
		Page:       1,
		PageSize:   500,
		MaxEvents:  input.DiagnosticsMaxEvents,
		SampleSize: input.DiagnosticsSampleSize,
	}
	for _, target := range targets {
		if strings.TrimSpace(target.ExecutionID) == "" {
			continue
		}

		diagnosticResult, err := collectExecutionDiagnostics(
			ctx,
			client,
			projectID,
			projectSlug,
			executionDiagnosticSubject{
				ExecutionID:    target.ExecutionID,
				AgentSessionID: target.AgentSessionID,
				Status:         target.Status,
				Outcome:        target.Outcome,
			},
			options,
		)
		if err != nil {
			result.DiagnosticsErrors = append(result.DiagnosticsErrors, verifyDiagnosticError{
				ExecutionID: target.ExecutionID,
				Error:       err.Error(),
			})
			continue
		}

		diagnostic := diagnosticResult.Diagnostic
		if strings.TrimSpace(diagnostic.ExecutionID) == "" {
			diagnostic.ExecutionID = target.ExecutionID
		}
		if strings.TrimSpace(diagnostic.AgentSessionID) == "" {
			diagnostic.AgentSessionID = target.AgentSessionID
		}
		if strings.TrimSpace(diagnostic.ConversationCommand) == "" {
			diagnostic.ConversationCommand = fmt.Sprintf(
				"certyn executions conversation --project %s %s",
				projectSlug,
				diagnostic.ExecutionID,
			)
		}
		if strings.TrimSpace(diagnostic.DiagnoseCommand) == "" {
			diagnostic.DiagnoseCommand = fmt.Sprintf(
				"certyn diagnose --project %s %s",
				projectSlug,
				diagnostic.ExecutionID,
			)
		}
		result.Diagnostics = append(result.Diagnostics, diagnostic)
	}
}

func shouldDiagnoseVerifyExecution(execution verifyExecutionSummary) bool {
	status := strings.ToLower(strings.TrimSpace(execution.Status))
	outcome := strings.ToLower(strings.TrimSpace(execution.Outcome))
	switch outcome {
	case "failed", "blocked", "aborted":
		return true
	}
	switch status {
	case "failed", "blocked", "aborted":
		return true
	default:
		return false
	}
}

func attachVerifyExecutionDetails(
	ctx context.Context,
	client *api.Client,
	projectID,
	projectSlug,
	runID string,
	result *verifyOutput,
) error {
	runID = strings.TrimSpace(runID)
	if runID == "" || result == nil {
		return nil
	}

	processRun, err := client.GetProcessRun(ctx, projectID, runID)
	if err != nil {
		return classifyAPIError(err, "failed to get process run details")
	}

	result.ExecutionTotal = processRun.TotalItems
	result.ExecutionPassed = processRun.PassedItems
	result.ExecutionFailed = processRun.FailedItems
	result.ExecutionBlocked = processRun.BlockedItems
	result.ExecutionRunning = processRun.RunningItems
	result.ExecutionPending = processRun.PendingItems

	if result.Total == 0 && processRun.TotalItems > 0 {
		result.Total = processRun.TotalItems
		result.Passed = processRun.PassedItems
		result.Failed = processRun.FailedItems
		result.Blocked = processRun.BlockedItems
		result.Pending = processRun.PendingItems
	}

	summaries := make([]verifyExecutionSummary, 0, len(processRun.Items))
	for _, item := range processRun.Items {
		summary := verifyExecutionSummary{
			ExecutionID:      strings.TrimSpace(ptrString(item.ExecutionID)),
			ProcessRunItemID: strings.TrimSpace(item.ID),
			TestCaseID:       strings.TrimSpace(ptrString(item.TestCaseID)),
			TestCaseName:     strings.TrimSpace(ptrString(item.TestCaseName)),
			Status:           strings.TrimSpace(ptrString(item.Status)),
			Outcome:          strings.TrimSpace(ptrString(item.TestOutcome)),
			AgentSessionID:   strings.TrimSpace(ptrString(item.AgentSessionID)),
			StartedAt:        item.StartedAt,
			CompletedAt:      item.CompletedAt,
		}
		if item.TestCaseNumber != nil {
			summary.TestCaseNumber = *item.TestCaseNumber
		}
		if summary.ExecutionID != "" {
			summary.ConversationCommand = fmt.Sprintf(
				"certyn executions conversation --project %s %s",
				projectSlug,
				summary.ExecutionID,
			)
		}
		summaries = append(summaries, summary)
	}
	result.Executions = summaries
	return nil
}

func normalizeVerifySuiteInput(raw string) (suite string, processIdentifier string) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	if normalized == "" {
		return "smoke", processAliases["smoke"]
	}
	if mapped, ok := processAliases[normalized]; ok {
		return normalized, mapped
	}
	return strings.TrimSpace(raw), strings.TrimSpace(raw)
}

func resolveVerifyProcessSlug(ctx context.Context, client *api.Client, projectID, identifier string) (string, error) {
	requested := strings.TrimSpace(identifier)
	if requested == "" {
		requested = processAliases["smoke"]
	}

	processes, err := client.ListProcesses(ctx, projectID)
	if err != nil {
		if shouldBypassVerifyProcessDiscovery(err) {
			// Some deployments expose CI endpoints for API keys while process discovery remains bearer-only.
			// In that case, defer validation to CI run creation and keep verify operational.
			return requested, nil
		}
		return "", classifyAPIError(err, "failed to list project processes")
	}

	available := ciCompatibleVerifyProcessSlugs(processes)
	needle := requested

	var matched *api.ProcessDefinition
	for i := range processes {
		process := &processes[i]
		if strings.EqualFold(strings.TrimSpace(process.Slug), needle) || strings.EqualFold(strings.TrimSpace(process.ID), needle) {
			matched = process
			break
		}
	}

	if matched == nil {
		return "", verifyProcessResolutionError(
			fmt.Sprintf("suite/process %q not found", identifier),
			available,
		)
	}

	if !matched.IsActive {
		return "", verifyProcessResolutionError(
			fmt.Sprintf("process '%s' is inactive", matched.Slug),
			available,
		)
	}
	if matched.IsExploratory {
		return "", verifyProcessResolutionError(
			fmt.Sprintf("process '%s' is exploratory and not CI-compatible", matched.Slug),
			available,
		)
	}
	if len(matched.Configuration.TicketLabels) > 0 {
		return "", verifyProcessResolutionError(
			fmt.Sprintf("process '%s' uses ticketLabels and is not CI-compatible", matched.Slug),
			available,
		)
	}

	resolvedSlug := strings.TrimSpace(matched.Slug)
	if resolvedSlug == "" {
		return "", usageError(fmt.Sprintf("process '%s' has an empty slug", matched.ID), nil)
	}
	return resolvedSlug, nil
}

func shouldBypassVerifyProcessDiscovery(err error) bool {
	var apiErr *api.APIError
	if !errors.As(err, &apiErr) {
		return false
	}
	switch apiErr.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound:
		return true
	default:
		return false
	}
}

func verifyProcessResolutionError(prefix string, available []string) error {
	if len(available) == 0 {
		return usageError(prefix+". no active CI-compatible suites are available for this project", nil)
	}
	return usageError(prefix+". available CI-compatible suites: "+strings.Join(available, ", "), nil)
}

func ciCompatibleVerifyProcessSlugs(processes []api.ProcessDefinition) []string {
	available := make([]string, 0, len(processes))
	for _, process := range processes {
		if !process.IsActive || process.IsExploratory || len(process.Configuration.TicketLabels) > 0 {
			continue
		}
		slug := strings.TrimSpace(process.Slug)
		if slug == "" {
			continue
		}
		available = append(available, slug)
	}
	sort.Strings(available)
	return available
}

func normalizeVerifyURL(raw string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", usageError("missing required flag --url", nil)
	}

	parsed, err := url.Parse(trimmed)
	if err != nil || !parsed.IsAbs() || strings.TrimSpace(parsed.Host) == "" {
		return "", usageError("invalid --url: expected absolute http(s) URL", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", usageError("invalid --url: only http and https are supported", nil)
	}

	return parsed.String(), nil
}

func preflightVerifyURL(ctx context.Context, target string) error {
	reqCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, target, nil)
	if err != nil {
		return usageError("failed to build url preflight request", err)
	}

	resp, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		return usageError(fmt.Sprintf("url is not reachable: %s", target), err)
	}
	_ = resp.Body.Close()
	return nil
}

func classifyVerifyEnvironmentError(err error, fallback string) error {
	var apiErr *api.APIError
	if errors.As(err, &apiErr) && (apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden) {
		return authError(
			"missing permission for environment management required by `certyn run --url` (need environment create/delete access)",
			apiErr,
		)
	}
	return classifyAPIError(err, fallback)
}

func storeProjectMapping(cfg *config.Manager, profile, slug, projectID string) error {
	normalizedSlug := strings.ToLower(strings.TrimSpace(slug))
	normalizedID := strings.TrimSpace(projectID)
	if normalizedSlug == "" || normalizedID == "" {
		return usageError("failed to store project mapping: empty slug or id", nil)
	}

	existing, ok := cfg.GetProjectMapping(profile, normalizedSlug)
	if ok && strings.EqualFold(strings.TrimSpace(existing), normalizedID) {
		return nil
	}

	cfg.SetProjectMapping(profile, normalizedSlug, normalizedID)
	if err := cfg.Save(); err != nil {
		return usageError("failed to save project mapping", err)
	}

	return nil
}

func generateVerifyEnvironmentKey(profile string) string {
	user := normalizeUserForEnvironment(firstNonEmpty(
		strings.TrimSpace(os.Getenv("USER")),
		strings.TrimSpace(os.Getenv("USERNAME")),
		strings.TrimSpace(profile),
		"user",
	))
	timestamp := time.Now().UTC().Format("20060102150405")
	random := strings.ReplaceAll(generateID(), "-", "")
	if len(random) > 8 {
		random = random[:8]
	}

	key := fmt.Sprintf("dev-%s-%s-%s", user, timestamp, random)
	if len(key) > 63 {
		key = key[:63]
		key = strings.Trim(key, "-")
	}
	if key == "" {
		return "dev-user"
	}
	return key
}

func normalizeUserForEnvironment(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	normalized = nonAlphaNumericPattern.ReplaceAllString(normalized, "-")
	normalized = strings.Trim(normalized, "-")
	if normalized == "" {
		normalized = "user"
	}
	if len(normalized) > 20 {
		normalized = normalized[:20]
		normalized = strings.Trim(normalized, "-")
		if normalized == "" {
			normalized = "user"
		}
	}
	return normalized
}

func buildVerifyVersion(sha string) string {
	normalizedSHA := strings.TrimSpace(sha)
	if normalizedSHA != "" {
		if len(normalizedSHA) > 12 {
			normalizedSHA = normalizedSHA[:12]
		}
		normalizedSHA = nonAlphaNumericPattern.ReplaceAllString(strings.ToLower(normalizedSHA), "")
		if normalizedSHA != "" {
			return "verify-" + normalizedSHA
		}
	}

	return "verify-" + time.Now().UTC().Format("20060102150405")
}

func applyVerifyError(result *verifyOutput, err error, fallbackCode int) {
	if err == nil {
		result.ExitCode = ExitOK
		return
	}
	result.ExitCode = exitCodeFromError(err, fallbackCode)
	result.Error = err.Error()
}

func exitCodeFromError(err error, fallback int) int {
	var cmdErr *CommandError
	if errors.As(err, &cmdErr) {
		return cmdErr.Code
	}
	if fallback == 0 {
		return ExitGateFailed
	}
	return fallback
}

func printVerifyHumanSummary(result verifyOutput, keepEnv bool) {
	st := output.NewStyler()

	printHumanHeader(st, verifySummaryKind(result), verifySummaryTitle(result))
	printHumanField(st, "mode", result.Mode)
	if result.EnvironmentMode != "" {
		printHumanField(st, "target", result.EnvironmentMode)
	}
	if result.Mode == "suite" {
		printHumanField(st, "process", firstNonEmpty(result.ProcessSlug, result.Suite))
	} else if len(result.Tags) > 0 {
		printHumanField(st, "tags", strings.Join(result.Tags, ", "))
	}
	if strings.TrimSpace(result.URL) != "" {
		printHumanField(st, "url", result.URL)
	}
	if result.EnvironmentKey != "" {
		printHumanField(st, "environment", result.EnvironmentKey)
	}
	if result.RunID != "" {
		printHumanField(st, "run id", result.RunID)
	}
	if result.State != "" || result.Conclusion != "" {
		printHumanField(st, "result", humanKVSummary(
			firstNonEmpty(result.State, ""),
			firstNonEmpty(result.Conclusion, ""),
		))
	}
	if result.RunID != "" {
		printHumanField(st, "totals", fmt.Sprintf(
			"%d passed, %d failed, %d blocked, %d pending, %d aborted",
			result.Passed,
			result.Failed,
			result.Blocked,
			result.Pending,
			result.Aborted,
		))
	}
	if result.ExecutionTotal > 0 || len(result.Executions) > 0 {
		printHumanField(st, "executions", fmt.Sprintf(
			"%d total, %d passed, %d failed, %d blocked, %d running, %d pending",
			result.ExecutionTotal,
			result.ExecutionPassed,
			result.ExecutionFailed,
			result.ExecutionBlocked,
			result.ExecutionRunning,
			result.ExecutionPending,
		))
	}
	switch {
	case len(result.Diagnostics) > 0:
		printHumanField(st, "diagnostics", fmt.Sprintf("%d failure summaries collected", len(result.Diagnostics)))
	case result.Conclusion == "passed":
		printHumanField(st, "diagnostics", "not needed")
	case result.DiagnosticsCollected:
		printHumanField(st, "diagnostics", "none collected")
	}
	if result.StatusURL != "" {
		printHumanField(st, "status url", result.StatusURL)
	}
	if result.AppURL != "" {
		printHumanField(st, "app url", result.AppURL)
	}

	if len(result.Executions) > 0 {
		fmt.Println()
		printHumanHeader(st, "info", "Executions")
		for _, execution := range result.Executions {
			testCase := valueOrDash(execution.TestCaseName)
			if execution.TestCaseNumber > 0 {
				testCase = fmt.Sprintf("TC-%d %s", execution.TestCaseNumber, valueOrDash(execution.TestCaseName))
			}
			fmt.Printf(
				"  - %s %s (%s)\n",
				st.Status(firstNonEmpty(execution.Outcome, execution.Status)),
				testCase,
				valueOrDash(execution.ExecutionID),
			)
			if execution.ConversationCommand != "" {
				printHumanField(st, "conversation", execution.ConversationCommand)
			}
		}
	}

	if len(result.Diagnostics) > 0 {
		fmt.Println()
		printHumanHeader(st, "warn", "Failure diagnostics")
		for _, diagnostic := range result.Diagnostics {
			fmt.Printf(
				"  - execution=%s reason=%s summary=%s\n",
				valueOrDash(diagnostic.ExecutionID),
				valueOrDash(diagnostic.PrimaryFailureReason),
				valueOrDash(diagnostic.FailureSummary),
			)
			printHumanField(st, "tool errors", fmt.Sprintf("%d", diagnostic.Counts.ToolErrors))
			printHumanField(st, "policy denied", fmt.Sprintf("%d", diagnostic.Counts.PolicyDenied))
			printHumanField(st, "network 4xx", fmt.Sprintf("%d", diagnostic.Counts.Network4xx))
			printHumanField(st, "network 5xx", fmt.Sprintf("%d", diagnostic.Counts.Network5xx))
			if diagnostic.DiagnoseCommand != "" {
				printHumanField(st, "diagnose", diagnostic.DiagnoseCommand)
			}
			if diagnostic.ConversationCommand != "" {
				printHumanField(st, "conversation", diagnostic.ConversationCommand)
			}
		}
	}
	if len(result.DiagnosticsErrors) > 0 {
		fmt.Println()
		printHumanHeader(st, "warn", "Diagnostics errors")
		for _, entry := range result.DiagnosticsErrors {
			fmt.Printf("  - execution=%s error=%s\n", valueOrDash(entry.ExecutionID), entry.Error)
		}
	}
	if result.ExecutionDetailsError != "" {
		fmt.Println()
		printHumanField(st, "execution details", "unavailable ("+result.ExecutionDetailsError+")")
	}

	switch {
	case result.EnvironmentMode != "ephemeral":
		return
	case keepEnv && result.EnvironmentKey != "":
		printHumanField(st, "cleanup", fmt.Sprintf("skipped (kept environment %s)", result.EnvironmentKey))
	case result.EnvironmentDeleted:
		printHumanField(st, "cleanup", "environment deleted")
	case result.EnvironmentKey != "":
		printHumanField(st, "cleanup", "environment not deleted")
	}
}

func verifySummaryKind(result verifyOutput) string {
	switch strings.ToLower(strings.TrimSpace(result.Conclusion)) {
	case "passed":
		return "ok"
	case "failed", "blocked", "aborted", "cancelled", "canceled":
		return "fail"
	}
	switch strings.ToLower(strings.TrimSpace(result.State)) {
	case "completed":
		return "info"
	case "running", "queued", "pending":
		return "warn"
	default:
		return "info"
	}
}

func verifySummaryTitle(result verifyOutput) string {
	switch strings.ToLower(strings.TrimSpace(result.Conclusion)) {
	case "passed":
		return "Run passed"
	case "failed", "blocked":
		return "Run needs attention"
	case "aborted", "cancelled", "canceled":
		return "Run aborted"
	default:
		return "Run summary"
	}
}
