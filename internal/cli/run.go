package cli

import (
	"strings"
	"time"

	"github.com/certyn/certyn-cli/internal/config"
	"github.com/spf13/cobra"
)

func newRunCommand(app *App) *cobra.Command {
	var project string
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
		Use:   "run [process-or-alias]",
		Short: "Run Certyn validation against a preview URL or existing environment",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 0 && len(nonEmptyValues(tags)) > 0 {
				return usageError("conflicting selectors: use either a process argument or --tag, not both", nil)
			}

			process := "smoke"
			if len(args) > 0 {
				process = strings.TrimSpace(args[0])
			}

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
				effectiveEnvironment = ""
			}

			if strings.TrimSpace(targetURL) == "" && effectiveEnvironment == "" {
				return usageError("missing target: provide --url for preview mode or --environment for existing-environment mode", nil)
			}

			output, runErr := runVerify(cmd.Context(), app, client, resolved, verifyInput{
				Suite:                 process,
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
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Feature tag to run (repeatable)")
	cmd.Flags().StringVar(&targetURL, "url", "", "Public application URL for preview mode")
	cmd.Flags().DurationVar(&timeout, "timeout", 30*time.Minute, "Wait timeout")
	cmd.Flags().DurationVar(&pollInterval, "poll-interval", 0, "Fixed polling interval (default uses retryAfterSeconds)")
	cmd.Flags().BoolVar(&keepEnv, "keep-env", false, "Keep the temporary environment after a preview run")
	cmd.Flags().BoolVar(&diagnoseFailed, "diagnose-failed", true, "Collect failure diagnostics for failed executions when the run gate fails")
	cmd.Flags().IntVar(&diagnosticsMaxEvents, "diagnostics-max-events", 1000, "Maximum conversation events to scan per failed execution")
	cmd.Flags().IntVar(&diagnosticsSampleSize, "diagnostics-sample-size", 10, "Maximum network/tool failure samples per failed execution")
	cmd.Flags().StringVar(&repository, "repo", "", "Repository name")
	cmd.Flags().StringVar(&ref, "ref", "", "Git ref")
	cmd.Flags().StringVar(&sha, "sha", "", "Commit SHA")
	cmd.Flags().StringVar(&eventName, "event", "", "Trigger event")
	cmd.Flags().StringVar(&externalURL, "external-url", "", "External CI URL")

	statusCmd := newCIStatusCommand(app)
	statusCmd.Short = "Get run status"
	waitCmd := newCIWaitCommand(app)
	waitCmd.Short = "Wait for run completion"
	cancelCmd := newCICancelCommand(app)
	cancelCmd.Short = "Cancel a run"
	listCmd := newCIListCommand(app)
	listCmd.Short = "List runs for a project"

	cmd.AddCommand(statusCmd)
	cmd.AddCommand(waitCmd)
	cmd.AddCommand(cancelCmd)
	cmd.AddCommand(listCmd)

	return cmd
}
