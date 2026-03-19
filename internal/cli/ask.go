package cli

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/certyn/certyn-cli/internal/api"
	"github.com/certyn/certyn-cli/internal/config"
	"github.com/certyn/certyn-cli/internal/output"
	"github.com/spf13/cobra"
)

const askSchemaVersion = "certyn.ask.v1"

type askOutput struct {
	SchemaVersion      string               `json:"schema_version"`
	Mode               string               `json:"mode"`
	Question           string               `json:"question"`
	ProjectInput       string               `json:"project_input,omitempty"`
	ProjectID          string               `json:"project_id,omitempty"`
	UsedProjectContext bool                 `json:"used_project_context"`
	Warnings           []string             `json:"warnings,omitempty"`
	ConversationID     string               `json:"conversation_id,omitempty"`
	MessageID          string               `json:"message_id,omitempty"`
	Role               string               `json:"role,omitempty"`
	Content            string               `json:"content,omitempty"`
	ToolCalls          []api.ToolCallResult `json:"tool_calls,omitempty"`
	CreatedAt          *time.Time           `json:"created_at,omitempty"`
	ExitCode           int                  `json:"exit_code"`
	Error              string               `json:"error,omitempty"`
}

type askInput struct {
	QuestionParts      []string
	Context            string
	MaxToolIterations  int
	MaxOutputTokens    int
	ExplicitProjectRef bool
}

func newAskCommand(app *App) *cobra.Command {
	var project string
	var extraContext string
	var maxToolIterations int
	var maxOutputTokens int

	cmd := &cobra.Command{
		Use:   "ask <question...>",
		Short: "Ask Certyn advisor mode for project-aware guidance",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, client, printer, err := app.ResolveRuntime(config.ResolveInput{Project: project}, true)
			if err != nil {
				return err
			}

			explicitProjectRef := cmd.Flags().Changed("project") || cmd.InheritedFlags().Changed("project")
			output, runErr := runAsk(
				cmd.Context(),
				client,
				resolved,
				askInput{
					QuestionParts:      args,
					Context:            extraContext,
					MaxToolIterations:  maxToolIterations,
					MaxOutputTokens:    maxOutputTokens,
					ExplicitProjectRef: explicitProjectRef,
				},
			)

			if printer.JSON {
				if emitErr := printer.EmitJSON(output); emitErr != nil {
					return emitErr
				}
				return runErr
			}

			printAskHumanOutput(output)
			return runErr
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&extraContext, "context", "", "Additional context appended to the question")
	cmd.Flags().IntVar(&maxToolIterations, "max-tool-iterations", 10, "Maximum tool iterations for advisor mode")
	cmd.Flags().IntVar(&maxOutputTokens, "max-output-tokens", 4096, "Maximum output token count for advisor mode")

	return cmd
}

func runAsk(
	ctx context.Context,
	client *api.Client,
	resolved config.Runtime,
	input askInput,
) (result askOutput, runErr error) {
	result = askOutput{
		SchemaVersion: askSchemaVersion,
		Mode:          "advisor",
		ExitCode:      ExitOK,
	}

	question := strings.TrimSpace(strings.Join(input.QuestionParts, " "))
	if question == "" {
		runErr = usageError("missing question: usage `certyn ask <question...>`", nil)
		applyAskError(&result, runErr, ExitUsage)
		return result, runErr
	}
	result.Question = question

	if input.MaxToolIterations <= 0 {
		runErr = usageError("invalid --max-tool-iterations: must be greater than zero", nil)
		applyAskError(&result, runErr, ExitUsage)
		return result, runErr
	}
	if input.MaxOutputTokens <= 0 {
		runErr = usageError("invalid --max-output-tokens: must be greater than zero", nil)
		applyAskError(&result, runErr, ExitUsage)
		return result, runErr
	}

	projectInput := strings.TrimSpace(resolved.Project)
	if projectInput != "" {
		result.ProjectInput = projectInput
	}

	projectID := ""
	if projectInput != "" {
		if looksLikeProjectID(projectInput) {
			projectID = projectInput
		} else {
			resolvedProjectID, err := resolveProjectID(ctx, client, projectInput)
			if err != nil {
				if input.ExplicitProjectRef {
					runErr = err
					applyAskError(&result, runErr, ExitUsage)
					return result, runErr
				}
				runErr = usageError(
					fmt.Sprintf(
						"project context is required for advisor mode; could not resolve project '%s'. update your config or pass --project",
						projectInput,
					),
					err,
				)
				applyAskError(&result, runErr, ExitUsage)
				return result, runErr
			} else {
				projectID = resolvedProjectID
			}
		}
	} else {
		defaultProjectID, defaultProjectSlug, err := resolveDefaultProjectID(ctx, client)
		if err != nil {
			runErr = err
			applyAskError(&result, runErr, ExitUsage)
			return result, runErr
		}
		projectID = defaultProjectID
		if defaultProjectSlug != "" {
			result.Warnings = append(
				result.Warnings,
				fmt.Sprintf("using default project '%s' for advisor mode", defaultProjectSlug),
			)
		} else {
			result.Warnings = append(result.Warnings, "using account default project for advisor mode")
		}
	}

	message := question
	if trimmedContext := strings.TrimSpace(input.Context); trimmedContext != "" {
		message = question + "\n\nAdditional context:\n" + trimmedContext
	}

	req := api.AskAdvisorRequest{
		Message:             message,
		ProjectID:           projectID,
		MaxToolIterations:   &input.MaxToolIterations,
		MaxOutputTokenCount: &input.MaxOutputTokens,
	}
	resp, err := client.AskAdvisor(ctx, req)
	if err != nil {
		runErr = classifyAPIError(err, "failed to ask advisor")
		applyAskError(&result, runErr, ExitGateFailed)
		return result, runErr
	}

	result.ProjectID = projectID
	result.UsedProjectContext = strings.TrimSpace(projectID) != ""
	result.ConversationID = strings.TrimSpace(resp.ConversationID)
	result.MessageID = strings.TrimSpace(resp.MessageID)
	result.Role = strings.TrimSpace(resp.Role)
	result.Content = resp.Content
	result.ToolCalls = resp.ToolCalls
	if !resp.CreatedAt.IsZero() {
		createdAt := resp.CreatedAt
		result.CreatedAt = &createdAt
	}

	return result, nil
}

func applyAskError(result *askOutput, err error, fallbackCode int) {
	if err == nil {
		result.ExitCode = ExitOK
		result.Error = ""
		return
	}

	result.ExitCode = exitCodeFromError(err, fallbackCode)
	result.Error = err.Error()
}

func printAskHumanOutput(result askOutput) {
	st := output.NewStyler()

	for _, warning := range result.Warnings {
		fmt.Printf("%s %s\n", st.Badge("warn"), warning)
	}
	if strings.TrimSpace(result.Content) != "" {
		if len(result.Warnings) > 0 {
			fmt.Println()
		}
		printHumanHeader(st, "info", "Advisor answer")
		fmt.Println()
		fmt.Println(result.Content)
	}
	if result.ProjectID != "" || result.ConversationID != "" || result.MessageID != "" || len(result.ToolCalls) > 0 {
		fmt.Println()
		printHumanHeader(st, "info", "Context")
		printHumanField(st, "project", valueOrDash(result.ProjectID))
		printHumanField(st, "conversation", valueOrDash(result.ConversationID))
		printHumanField(st, "message", valueOrDash(result.MessageID))
		printHumanField(st, "tool calls", fmt.Sprintf("%d", len(result.ToolCalls)))
	}
}
