package cli

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/certyn/certyn-cli/internal/api"
	"github.com/certyn/certyn-cli/internal/config"
	"github.com/certyn/certyn-cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	allowedIssueActivities           = []string{"active", "attention", "idle"}
	allowedIssueTypes                = []string{"bug", "improvement", "task"}
	allowedIssueSeverities           = []string{"critical", "major", "minor", "trivial"}
	allowedIssueStatuses             = []string{"open", "closed", "canceled"}
	allowedIssuePriorities           = []string{"critical", "major", "minor", "trivial"}
	allowedIssueVerificationDecision = []string{"passed", "failed", "blocked", "needs_review", "cannot_reproduce"}
)

func newIssuesCommand(app *App) *cobra.Command {
	issuesCmd := &cobra.Command{
		Use:   "issues",
		Short: "Manage issue triage and verification",
	}

	issuesCmd.AddCommand(newIssuesListCommand(app))
	issuesCmd.AddCommand(newIssuesGetCommand(app))
	issuesCmd.AddCommand(newIssuesOverviewCommand(app))
	issuesCmd.AddCommand(newIssuesCreateCommand(app))
	issuesCmd.AddCommand(newIssuesUpdateCommand(app))
	issuesCmd.AddCommand(newIssuesDeleteCommand(app))
	issuesCmd.AddCommand(newIssuesLinkTestcaseCommand(app))
	issuesCmd.AddCommand(newRemovedCommand("link-testcase", "issues link-test"))
	issuesCmd.AddCommand(newIssuesCommentAddCommand(app))
	issuesCmd.AddCommand(newIssuesAttachmentAddCommand(app))
	issuesCmd.AddCommand(newIssuesVerificationAddCommand(app))
	issuesCmd.AddCommand(newIssuesRetestCommand(app))

	labelsCmd := &cobra.Command{Use: "labels", Short: "Issue label operations"}
	labelsCmd.AddCommand(newIssuesLabelsSetCommand(app))
	issuesCmd.AddCommand(labelsCmd)

	return issuesCmd
}

func newIssuesListCommand(app *App) *cobra.Command {
	var project string
	var activity string
	var issueType string
	var severity string
	var status string
	var agentID string
	var environment string
	var environmentVersion string
	var page int
	var pageSize int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List issues for a project",
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

			activity, err = normalizeEnum("activity", activity, allowedIssueActivities)
			if err != nil {
				return err
			}
			issueType, err = normalizeEnum("type", issueType, allowedIssueTypes)
			if err != nil {
				return err
			}
			severity, err = normalizeEnum("severity", severity, allowedIssueSeverities)
			if err != nil {
				return err
			}
			status, err = normalizeEnum("status", status, allowedIssueStatuses)
			if err != nil {
				return err
			}

			resp, err := client.ListIssues(cmd.Context(), projectID, api.ListIssuesParams{
				Activity:           activity,
				Type:               issueType,
				Severity:           severity,
				Status:             status,
				AgentID:            strings.TrimSpace(agentID),
				EnvironmentKey:     strings.TrimSpace(environment),
				EnvironmentVersion: strings.TrimSpace(environmentVersion),
				Page:               page,
				PageSize:           pageSize,
			})
			if err != nil {
				return classifyProjectRouteError(err, resolved.Project, resolved.Profile, "failed to list issues")
			}
			if printer.JSON {
				return printer.EmitJSON(resp)
			}

			st := output.NewStyler()
			printHumanHeader(st, "info", fmt.Sprintf("Issues (%d)", resp.TotalCount))
			for _, issue := range resp.Items {
				printHumanItem(st, humanKVSummary(
					fmt.Sprintf("#%d", issue.Number),
					st.Status(issue.Severity),
					st.Status(issue.Status),
					ptrStringOrDash(issue.RunActivity),
					issue.Type,
					issue.Title,
				))
				printHumanField(st, "id", issue.ID)
				printHumanField(st, "env version", ptrStringOrDash(issue.EnvironmentVersion))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&activity, "activity", "", "Activity filter: active, attention, idle")
	cmd.Flags().StringVar(&issueType, "type", "", "Issue type: bug, improvement, task")
	cmd.Flags().StringVar(&severity, "severity", "", "Severity: critical, major, minor, trivial")
	cmd.Flags().StringVar(&status, "status", "", "Status: open, closed, canceled")
	cmd.Flags().StringVar(&agentID, "agent-id", "", "Filter by agent id")
	cmd.Flags().StringVar(&environment, "environment", "", "Filter by environment key")
	cmd.Flags().StringVar(&environmentVersion, "environment-version", "", "Filter by environment version")
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&pageSize, "page-size", 20, "Page size")

	return cmd
}

func newIssuesGetCommand(app *App) *cobra.Command {
	var project string

	cmd := &cobra.Command{
		Use:   "get <issue-id>",
		Short: "Get issue details",
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

			issue, err := client.GetIssue(cmd.Context(), projectID, strings.TrimSpace(args[0]))
			if err != nil {
				return classifyAPIError(err, "failed to get issue")
			}
			if printer.JSON {
				return printer.EmitJSON(issue)
			}

			st := output.NewStyler()
			printHumanHeader(st, "info", fmt.Sprintf("Issue #%d", issue.Number))
			printHumanField(st, "id", issue.ID)
			printHumanField(st, "title", issue.Title)
			printHumanField(st, "status", st.Status(issue.Status))
			printHumanField(st, "severity", st.Status(issue.Severity))
			printHumanField(st, "type", issue.Type)
			printHumanField(st, "activity", ptrStringOrDash(issue.RunActivity))
			printHumanField(st, "env version", ptrStringOrDash(issue.EnvironmentVersion))
			printHumanField(st, "description", ptrStringOrDash(issue.Description))
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	return cmd
}

func newIssuesOverviewCommand(app *App) *cobra.Command {
	var project string
	var environment string
	var page int
	var pageSize int

	cmd := &cobra.Command{
		Use:   "overview",
		Short: "List issue overview data",
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

			resp, err := client.ListIssuesOverview(cmd.Context(), projectID, strings.TrimSpace(environment), page, pageSize)
			if err != nil {
				return classifyProjectRouteError(err, resolved.Project, resolved.Profile, "failed to list issues overview")
			}
			if printer.JSON {
				return printer.EmitJSON(resp)
			}

			st := output.NewStyler()
			printHumanHeader(st, "info", fmt.Sprintf("Issue overview (%d)", resp.TotalCount))
			for _, issue := range resp.Items {
				printHumanItem(st, humanKVSummary(
					fmt.Sprintf("#%d", issue.Number),
					st.Status(issue.Severity),
					st.Status(issue.Status),
					valueOrDash(issue.RunActivity),
					issue.Title,
				))
				printHumanField(st, "id", issue.ID)
				printHumanField(st, "env", ptrStringOrDash(issue.EnvironmentKey))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&environment, "environment", "", "Filter by environment key")
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&pageSize, "page-size", 20, "Page size")
	return cmd
}

func newIssuesCreateCommand(app *App) *cobra.Command {
	var project string
	var title string
	var description string
	var issueType string
	var severity string
	var status string
	var labels []string
	var needsReview bool
	var createdByAgent string
	var createdDuringExecution string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an issue",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireValue("title", title); err != nil {
				return err
			}

			issueType, err := normalizeEnum("type", issueType, allowedIssueTypes)
			if err != nil {
				return err
			}
			severity, err = normalizeEnum("severity", severity, allowedIssueSeverities)
			if err != nil {
				return err
			}
			status, err = normalizeEnum("status", status, allowedIssueStatuses)
			if err != nil {
				return err
			}

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

			input := api.IssueInput{
				Title:                    title,
				Description:              description,
				Type:                     issueType,
				Severity:                 severity,
				Status:                   status,
				Labels:                   nonEmptyValues(labels),
				CreatedByAgentTemplateID: strings.TrimSpace(createdByAgent),
				CreatedDuringExecutionID: strings.TrimSpace(createdDuringExecution),
			}
			if cmd.Flags().Changed("needs-review") {
				input.NeedsReview = &needsReview
			}

			issue, err := client.CreateIssue(cmd.Context(), projectID, input)
			if err != nil {
				return classifyProjectRouteError(err, resolved.Project, resolved.Profile, "failed to create issue")
			}
			if printer.JSON {
				return printer.EmitJSON(issue)
			}

			fmt.Printf("Created issue %s\n", issue.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&title, "title", "", "Issue title")
	cmd.Flags().StringVar(&description, "description", "", "Issue description")
	cmd.Flags().StringVar(&issueType, "type", "", "Issue type: bug, improvement, task")
	cmd.Flags().StringVar(&severity, "severity", "", "Severity: critical, major, minor, trivial")
	cmd.Flags().StringVar(&status, "status", "", "Status: open, closed, canceled")
	cmd.Flags().StringArrayVar(&labels, "label", nil, "Label (repeatable)")
	cmd.Flags().BoolVar(&needsReview, "needs-review", false, "Set needs review state")
	cmd.Flags().StringVar(&createdByAgent, "created-by-agent", "", "Created by agent template id")
	cmd.Flags().StringVar(&createdDuringExecution, "created-during-execution", "", "Created during execution id")
	return cmd
}

func newIssuesUpdateCommand(app *App) *cobra.Command {
	var project string
	var title string
	var description string
	var issueType string
	var severity string
	var status string
	var labels []string
	var needsReview bool
	var createdByAgent string
	var createdDuringExecution string

	cmd := &cobra.Command{
		Use:   "update <issue-id>",
		Short: "Update an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAnyFlagChanged(
				cmd,
				"title",
				"description",
				"type",
				"severity",
				"status",
				"label",
				"needs-review",
				"created-by-agent",
				"created-during-execution",
			); err != nil {
				return err
			}

			issueType, err := normalizeEnum("type", issueType, allowedIssueTypes)
			if err != nil {
				return err
			}
			severity, err = normalizeEnum("severity", severity, allowedIssueSeverities)
			if err != nil {
				return err
			}
			status, err = normalizeEnum("status", status, allowedIssueStatuses)
			if err != nil {
				return err
			}

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
			issueID := strings.TrimSpace(args[0])

			input := api.IssueInput{}
			if cmd.Flags().Changed("title") {
				input.Title = title
			}
			if cmd.Flags().Changed("description") {
				input.Description = description
			}
			if cmd.Flags().Changed("type") {
				input.Type = issueType
			}
			if cmd.Flags().Changed("severity") {
				input.Severity = severity
			}
			if cmd.Flags().Changed("status") {
				input.Status = status
			}
			if cmd.Flags().Changed("label") {
				input.Labels = nonEmptyValues(labels)
			}
			if cmd.Flags().Changed("needs-review") {
				input.NeedsReview = &needsReview
			}
			if cmd.Flags().Changed("created-by-agent") {
				input.CreatedByAgentTemplateID = strings.TrimSpace(createdByAgent)
			}
			if cmd.Flags().Changed("created-during-execution") {
				input.CreatedDuringExecutionID = strings.TrimSpace(createdDuringExecution)
			}

			if err := client.UpdateIssue(cmd.Context(), projectID, issueID, input); err != nil {
				return classifyAPIError(err, "failed to update issue")
			}
			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"project_id": projectID,
					"issue_id":   issueID,
					"updated":    true,
				})
			}

			fmt.Printf("Updated issue %s\n", issueID)
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&title, "title", "", "Issue title")
	cmd.Flags().StringVar(&description, "description", "", "Issue description")
	cmd.Flags().StringVar(&issueType, "type", "", "Issue type: bug, improvement, task")
	cmd.Flags().StringVar(&severity, "severity", "", "Severity: critical, major, minor, trivial")
	cmd.Flags().StringVar(&status, "status", "", "Status: open, closed, canceled")
	cmd.Flags().StringArrayVar(&labels, "label", nil, "Label (repeatable)")
	cmd.Flags().BoolVar(&needsReview, "needs-review", false, "Set needs review state")
	cmd.Flags().StringVar(&createdByAgent, "created-by-agent", "", "Created by agent template id")
	cmd.Flags().StringVar(&createdDuringExecution, "created-during-execution", "", "Created during execution id")
	return cmd
}

func newIssuesDeleteCommand(app *App) *cobra.Command {
	var project string
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete <issue-id>",
		Short: "Delete an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return usageError("pass --yes to confirm issue deletion", nil)
			}
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

			issueID := strings.TrimSpace(args[0])
			if err := client.DeleteIssue(cmd.Context(), projectID, issueID); err != nil {
				return classifyAPIError(err, "failed to delete issue")
			}
			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"project_id": projectID,
					"issue_id":   issueID,
					"deleted":    true,
				})
			}

			fmt.Printf("Deleted issue %s\n", issueID)
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm deletion")
	return cmd
}

func newIssuesLabelsSetCommand(app *App) *cobra.Command {
	var project string
	var labels []string
	var clear bool

	cmd := &cobra.Command{
		Use:   "set <issue-id>",
		Short: "Set issue labels",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			normalizedLabels := nonEmptyValues(labels)
			if clear && len(normalizedLabels) > 0 {
				return usageError("use either --label or --clear, not both", nil)
			}
			if !clear && len(normalizedLabels) == 0 {
				return usageError("provide at least one --label or pass --clear", nil)
			}
			if clear {
				normalizedLabels = []string{}
			}

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

			issueID := strings.TrimSpace(args[0])
			if err := client.UpdateIssueLabels(cmd.Context(), projectID, issueID, normalizedLabels); err != nil {
				return classifyAPIError(err, "failed to update issue labels")
			}
			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"project_id": projectID,
					"issue_id":   issueID,
					"labels":     normalizedLabels,
					"updated":    true,
				})
			}

			fmt.Printf("Updated labels for issue %s\n", issueID)
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringArrayVar(&labels, "label", nil, "Label (repeatable)")
	cmd.Flags().BoolVar(&clear, "clear", false, "Clear all labels")
	return cmd
}

func newIssuesLinkTestcaseCommand(app *App) *cobra.Command {
	var project string
	var testCaseID string
	var priority string

	cmd := &cobra.Command{
		Use:   "link-test <issue-id>",
		Short: "Link a test to an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireValue("testcase", testCaseID); err != nil {
				return err
			}
			priority, err := normalizeEnum("priority", priority, allowedIssuePriorities)
			if err != nil {
				return err
			}

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

			issueID := strings.TrimSpace(args[0])
			resp, err := client.LinkIssueTestCase(cmd.Context(), projectID, issueID, api.LinkIssueTestCaseInput{
				TestCaseID: strings.TrimSpace(testCaseID),
				Priority:   priority,
			})
			if err != nil {
				return classifyAPIError(err, "failed to link testcase")
			}
			if printer.JSON {
				return printer.EmitJSON(resp)
			}

			fmt.Printf("Linked testcase to issue %s\n", resp.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&testCaseID, "testcase", "", "Test case id")
	cmd.Flags().StringVar(&priority, "priority", "", "Priority: critical, major, minor, trivial")
	return cmd
}

func newIssuesCommentAddCommand(app *App) *cobra.Command {
	var project string
	var body string

	cmd := &cobra.Command{
		Use:   "comment <issue-id>",
		Short: "Add a comment to an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireValue("body", body); err != nil {
				return err
			}
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

			issueID := strings.TrimSpace(args[0])
			comment, err := client.AddIssueComment(cmd.Context(), projectID, issueID, body)
			if err != nil {
				return classifyAPIError(err, "failed to add issue comment")
			}
			if printer.JSON {
				return printer.EmitJSON(comment)
			}

			fmt.Printf("Added comment %s to issue %s\n", comment.ID, issueID)
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&body, "body", "", "Comment markdown text")
	return cmd
}

func newIssuesAttachmentAddCommand(app *App) *cobra.Command {
	var project string
	var kind string
	var filePath string
	var attachmentURL string
	var description string

	cmd := &cobra.Command{
		Use:   "attach <issue-id>",
		Short: "Attach a file or URL to an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireValue("kind", kind); err != nil {
				return err
			}

			trimmedFile := strings.TrimSpace(filePath)
			trimmedURL := strings.TrimSpace(attachmentURL)
			hasFile := trimmedFile != ""
			hasURL := trimmedURL != ""
			if hasFile == hasURL {
				return usageError("provide either --file or --url", nil)
			}
			if hasURL {
				if _, err := url.ParseRequestURI(trimmedURL); err != nil {
					return usageError("invalid value for --url", err)
				}
			}

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

			payload := api.IssueAttachmentInput{
				Kind:        strings.TrimSpace(kind),
				URL:         trimmedURL,
				Description: description,
			}
			if hasFile {
				base64Data, err := readFileAsBase64(trimmedFile)
				if err != nil {
					return err
				}
				payload.Base64Data = base64Data
			}

			issueID := strings.TrimSpace(args[0])
			attachment, err := client.AddIssueAttachment(cmd.Context(), projectID, issueID, payload)
			if err != nil {
				return classifyAPIError(err, "failed to add issue attachment")
			}
			if printer.JSON {
				return printer.EmitJSON(attachment)
			}

			fmt.Printf("Added attachment %s to issue %s\n", attachment.ID, issueID)
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&kind, "kind", "", "Attachment kind")
	cmd.Flags().StringVar(&filePath, "file", "", "Attachment file path")
	cmd.Flags().StringVar(&attachmentURL, "url", "", "Attachment URL")
	cmd.Flags().StringVar(&description, "description", "", "Attachment description")
	return cmd
}

func newIssuesVerificationAddCommand(app *App) *cobra.Command {
	var project string
	var decision string
	var summary string
	var expected string
	var observed string
	var screenshotFile string

	cmd := &cobra.Command{
		Use:   "verify <issue-id>",
		Short: "Add a verification result to an issue",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireValue("decision", decision); err != nil {
				return err
			}
			if err := requireValue("summary", summary); err != nil {
				return err
			}

			decision, err := normalizeEnum("decision", decision, allowedIssueVerificationDecision)
			if err != nil {
				return err
			}

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

			payload := api.IssueVerificationInput{
				Decision: decision,
				Summary:  summary,
				Expected: expected,
				Observed: observed,
			}
			if cmd.Flags().Changed("screenshot-file") {
				base64Screenshot, err := readFileAsBase64(screenshotFile)
				if err != nil {
					return err
				}
				payload.Base64Screenshot = base64Screenshot
			}

			issueID := strings.TrimSpace(args[0])
			verification, err := client.AddIssueVerification(cmd.Context(), projectID, issueID, payload)
			if err != nil {
				return classifyAPIError(err, "failed to add issue verification")
			}
			if printer.JSON {
				return printer.EmitJSON(verification)
			}

			fmt.Printf("Added verification %s to issue %s\n", verification.ID, issueID)
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&decision, "decision", "", "Decision: passed, failed, blocked, needs_review, cannot_reproduce")
	cmd.Flags().StringVar(&summary, "summary", "", "Verification summary")
	cmd.Flags().StringVar(&expected, "expected", "", "Expected result")
	cmd.Flags().StringVar(&observed, "observed", "", "Observed result")
	cmd.Flags().StringVar(&screenshotFile, "screenshot-file", "", "Screenshot file path")
	return cmd
}

func newIssuesRetestCommand(app *App) *cobra.Command {
	var project string

	cmd := &cobra.Command{
		Use:   "retest <issue-id>",
		Short: "Request issue retest",
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

			resp, err := client.RetestIssue(cmd.Context(), projectID, strings.TrimSpace(args[0]))
			if err != nil {
				return classifyAPIError(err, "failed to retest issue")
			}
			if printer.JSON {
				if emitErr := printer.EmitJSON(resp); emitErr != nil {
					return emitErr
				}
			}

			message := ptrStringOrDash(resp.Message)
			if !resp.Success {
				return &CommandError{Code: ExitGateFailed, Message: message}
			}

			if !printer.JSON {
				fmt.Printf("Retest requested for issue %s\n", strings.TrimSpace(args[0]))
				fmt.Printf("Message: %s\n", message)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	return cmd
}
