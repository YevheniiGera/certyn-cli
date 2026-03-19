package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/certyn/certyn-cli/internal/api"
	"github.com/certyn/certyn-cli/internal/config"
	"github.com/certyn/certyn-cli/internal/output"
	"github.com/spf13/cobra"
)

func newTestsCommand(app *App) *cobra.Command {
	testcasesCmd := &cobra.Command{
		Use:   "tests",
		Short: "Manage test cases and execution triage",
	}

	testcasesCmd.AddCommand(newTestcasesListCommand(app))
	testcasesCmd.AddCommand(newTestcasesGetCommand(app))
	testcasesCmd.AddCommand(newTestcasesOverviewCommand(app))
	testcasesCmd.AddCommand(newTestcasesReportCommand(app))
	testcasesCmd.AddCommand(newTestcasesFlakinessCommand(app))
	testcasesCmd.AddCommand(newTestcasesFlakyCommand(app))
	testcasesCmd.AddCommand(newTestcasesCreateCommand(app))
	testcasesCmd.AddCommand(newTestcasesUpdateCommand(app))
	testcasesCmd.AddCommand(newTestcasesDeleteCommand(app))
	testcasesCmd.AddCommand(newTestcasesExecuteCommand(app))
	testcasesCmd.AddCommand(newTestcasesExecuteBulkCommand(app))
	testcasesCmd.AddCommand(newRemovedCommand("execute", "tests run"))
	testcasesCmd.AddCommand(newRemovedCommand("execute-bulk", "tests run-many"))

	tagsCmd := &cobra.Command{Use: "tags", Short: "Test case tag operations"}
	tagsCmd.AddCommand(newTestcasesTagsSetCommand(app))
	testcasesCmd.AddCommand(tagsCmd)

	return testcasesCmd
}

func newTestcasesListCommand(app *App) *cobra.Command {
	var project string
	var tag string
	var quarantined string
	var page int
	var pageSize int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List test cases for a project",
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
			quarantinedPtr, err := parseOptionalBool("quarantined", quarantined)
			if err != nil {
				return err
			}

			resp, err := client.ListTestCases(cmd.Context(), projectID, quarantinedPtr, tag, page, pageSize)
			if err != nil {
				return classifyProjectRouteError(err, resolved.Project, resolved.Profile, "failed to list test cases")
			}
			if printer.JSON {
				return printer.EmitJSON(resp)
			}

			st := output.NewStyler()
			printHumanHeader(st, "info", fmt.Sprintf("Tests (%d)", resp.TotalCount))
			for _, tc := range resp.Items {
				printHumanItem(st, humanKVSummary(
					fmt.Sprintf("#%d", tc.Number),
					tc.Name,
				))
				printHumanField(st, "id", tc.ID)
				printHumanField(st, "quarantined", humanBool(st, tc.IsQuarantined))
				printHumanField(st, "needs review", humanBool(st, tc.NeedsReview))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&tag, "tag", "", "Filter by tag")
	cmd.Flags().StringVar(&quarantined, "quarantined", "", "Filter by quarantine state: true or false")
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&pageSize, "page-size", 20, "Page size")

	return cmd
}

func newTestcasesGetCommand(app *App) *cobra.Command {
	var project string

	cmd := &cobra.Command{
		Use:   "get <testcase-id>",
		Short: "Get test case details",
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

			tc, err := client.GetTestCase(cmd.Context(), projectID, strings.TrimSpace(args[0]))
			if err != nil {
				return classifyAPIError(err, "failed to get test case")
			}
			if printer.JSON {
				return printer.EmitJSON(tc)
			}

			st := output.NewStyler()
			printHumanHeader(st, "info", fmt.Sprintf("Test #%d", tc.Number))
			printHumanField(st, "id", tc.ID)
			printHumanField(st, "name", tc.Name)
			printHumanField(st, "description", ptrStringOrDash(tc.Description))
			printHumanField(st, "quarantined", humanBool(st, tc.IsQuarantined))
			printHumanField(st, "needs review", humanBool(st, tc.NeedsReview))
			printHumanField(st, "min env", ptrStringOrDash(tc.MinimumSupportedEnvironmentVersion))
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	return cmd
}

func newTestcasesOverviewCommand(app *App) *cobra.Command {
	var project string
	var page int
	var pageSize int

	cmd := &cobra.Command{
		Use:   "overview",
		Short: "List test case quality overview",
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

			resp, err := client.ListTestCaseOverview(cmd.Context(), projectID, page, pageSize)
			if err != nil {
				return classifyProjectRouteError(err, resolved.Project, resolved.Profile, "failed to list test case overview")
			}
			if printer.JSON {
				return printer.EmitJSON(resp)
			}

			st := output.NewStyler()
			printHumanHeader(st, "info", fmt.Sprintf("Test overview (%d)", resp.TotalCount))
			for _, tc := range resp.Items {
				printHumanItem(st, humanKVSummary(
					fmt.Sprintf("#%d", tc.Number),
					tc.Name,
					fmt.Sprintf("pass %.2f", tc.PassRate),
					fmt.Sprintf("flaky %.2f", tc.FlakinessRate),
				))
				printHumanField(st, "id", tc.ID)
				printHumanField(st, "active tickets", fmt.Sprintf("%d", tc.ActiveTickets))
				printHumanField(st, "quarantined", humanBool(st, tc.IsQuarantined))
				printHumanField(st, "needs review", humanBool(st, tc.NeedsReview))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&pageSize, "page-size", 20, "Page size")

	return cmd
}

func newTestcasesReportCommand(app *App) *cobra.Command {
	var project string

	cmd := &cobra.Command{
		Use:   "report <testcase-id>",
		Short: "Get test case run report",
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

			report, err := client.GetTestCaseReport(cmd.Context(), projectID, strings.TrimSpace(args[0]))
			if err != nil {
				return classifyAPIError(err, "failed to get test case report")
			}
			if printer.JSON {
				return printer.EmitJSON(report)
			}

			st := output.NewStyler()
			printHumanHeader(st, "info", "Test report")
			printHumanField(st, "id", report.TestCase.ID)
			printHumanField(st, "name", report.TestCase.Name)
			printHumanField(st, "pass rate", fmt.Sprintf("%.2f", report.PassRate))
			printHumanField(st, "flakiness", fmt.Sprintf("%.2f", report.FlakinessRate))
			printHumanField(st, "runs", fmt.Sprintf("%d total, %d passed, %d failed", report.TotalRuns, report.PassedRuns, report.FailedRuns))
			printHumanField(st, "active tickets", fmt.Sprintf("%d", report.ActiveTickets))
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	return cmd
}

func newTestcasesFlakinessCommand(app *App) *cobra.Command {
	var project string

	cmd := &cobra.Command{
		Use:   "flakiness <testcase-id>",
		Short: "Get flakiness details for a test case",
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

			flakiness, err := client.GetTestCaseFlakiness(cmd.Context(), projectID, strings.TrimSpace(args[0]))
			if err != nil {
				return classifyAPIError(err, "failed to get test case flakiness")
			}
			if printer.JSON {
				return printer.EmitJSON(flakiness)
			}

			st := output.NewStyler()
			printHumanHeader(st, "info", "Flakiness")
			printHumanField(st, "test", flakiness.TestCaseID)
			printHumanField(st, "score", fmt.Sprintf("%.2f", flakiness.FlakinessScore))
			printHumanField(st, "runs", fmt.Sprintf("%d total, %d passed, %d failed, %d flips",
				flakiness.TotalRuns,
				flakiness.PassedRuns,
				flakiness.FailedRuns,
				flakiness.FlipCount,
			))
			printHumanField(st, "quarantined", humanBool(st, flakiness.IsQuarantined))
			printHumanField(st, "should quar", humanBool(st, flakiness.ShouldQuarantine))
			printHumanField(st, "last run", timeStringOrDash(flakiness.LastRunAt))
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	return cmd
}

func newTestcasesFlakyCommand(app *App) *cobra.Command {
	var project string
	var threshold float64

	cmd := &cobra.Command{
		Use:   "flaky",
		Short: "List flaky test cases",
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

			items, err := client.ListFlakyTestCases(cmd.Context(), projectID, threshold)
			if err != nil {
				return classifyProjectRouteError(err, resolved.Project, resolved.Profile, "failed to list flaky test cases")
			}
			if printer.JSON {
				return printer.EmitJSON(items)
			}

			st := output.NewStyler()
			printHumanHeader(st, "warn", fmt.Sprintf("Flaky tests (%d)", len(items)))
			printHumanField(st, "threshold", fmt.Sprintf("%.2f", threshold))
			for _, item := range items {
				printHumanItem(st, humanKVSummary(
					item.TestCaseID,
					fmt.Sprintf("score %.2f", item.FlakinessScore),
					"last run "+timeStringOrDash(item.LastRunAt),
				))
				printHumanField(st, "quarantined", humanBool(st, item.IsQuarantined))
				printHumanField(st, "should quar", humanBool(st, item.ShouldQuarantine))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().Float64Var(&threshold, "threshold", 0.20, "Flakiness threshold")
	return cmd
}

func newTestcasesCreateCommand(app *App) *cobra.Command {
	var project string
	var name string
	var instructions string
	var description string
	var tags []string
	var quarantined bool
	var needsReview bool
	var minEnvVersion string
	var createdByAgent string
	var createdDuringExecution string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a test case",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, client, printer, err := app.ResolveRuntime(config.ResolveInput{Project: project}, true)
			if err != nil {
				return err
			}
			if err := requireValue("project", resolved.Project); err != nil {
				return err
			}
			if err := requireValue("name", name); err != nil {
				return err
			}
			if err := requireValue("instructions", instructions); err != nil {
				return err
			}

			projectID, err := resolveProjectRouteIDFromConfig(app, resolved.Project, resolved.Profile)
			if err != nil {
				return err
			}

			input := api.TestCaseInput{
				Name:                               name,
				Instructions:                       instructions,
				Description:                        description,
				Tags:                               nonEmptyValues(tags),
				MinimumSupportedEnvironmentVersion: strings.TrimSpace(minEnvVersion),
				CreatedByAgentTemplateID:           strings.TrimSpace(createdByAgent),
				CreatedDuringExecutionID:           strings.TrimSpace(createdDuringExecution),
			}
			if cmd.Flags().Changed("quarantined") {
				input.IsQuarantined = &quarantined
			}
			if cmd.Flags().Changed("needs-review") {
				input.NeedsReview = &needsReview
			}

			testCase, err := client.CreateTestCase(cmd.Context(), projectID, input)
			if err != nil {
				return classifyProjectRouteError(err, resolved.Project, resolved.Profile, "failed to create test case")
			}
			if printer.JSON {
				return printer.EmitJSON(testCase)
			}

			fmt.Printf("Created test case %s (%s)\n", testCase.ID, testCase.Name)
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&name, "name", "", "Test case name")
	cmd.Flags().StringVar(&instructions, "instructions", "", "Test instructions")
	cmd.Flags().StringVar(&description, "description", "", "Test case description")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Tag (repeatable)")
	cmd.Flags().BoolVar(&quarantined, "quarantined", false, "Set quarantine state")
	cmd.Flags().BoolVar(&needsReview, "needs-review", false, "Set needs review state")
	cmd.Flags().StringVar(&minEnvVersion, "min-env-version", "", "Minimum supported environment version")
	cmd.Flags().StringVar(&createdByAgent, "created-by-agent", "", "Created by agent template id")
	cmd.Flags().StringVar(&createdDuringExecution, "created-during-execution", "", "Created during execution id")

	return cmd
}

func newTestcasesUpdateCommand(app *App) *cobra.Command {
	var project string
	var name string
	var instructions string
	var description string
	var tags []string
	var quarantined bool
	var needsReview bool
	var minEnvVersion string
	var createdByAgent string
	var createdDuringExecution string

	cmd := &cobra.Command{
		Use:   "update <testcase-id>",
		Short: "Update a test case",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireAnyFlagChanged(
				cmd,
				"name",
				"instructions",
				"description",
				"tag",
				"quarantined",
				"needs-review",
				"min-env-version",
				"created-by-agent",
				"created-during-execution",
			); err != nil {
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

			input := api.TestCaseInput{}
			if cmd.Flags().Changed("name") {
				input.Name = name
			}
			if cmd.Flags().Changed("instructions") {
				input.Instructions = instructions
			}
			if cmd.Flags().Changed("description") {
				input.Description = description
			}
			if cmd.Flags().Changed("tag") {
				input.Tags = nonEmptyValues(tags)
			}
			if cmd.Flags().Changed("quarantined") {
				input.IsQuarantined = &quarantined
			}
			if cmd.Flags().Changed("needs-review") {
				input.NeedsReview = &needsReview
			}
			if cmd.Flags().Changed("min-env-version") {
				input.MinimumSupportedEnvironmentVersion = strings.TrimSpace(minEnvVersion)
			}
			if cmd.Flags().Changed("created-by-agent") {
				input.CreatedByAgentTemplateID = strings.TrimSpace(createdByAgent)
			}
			if cmd.Flags().Changed("created-during-execution") {
				input.CreatedDuringExecutionID = strings.TrimSpace(createdDuringExecution)
			}

			testCase, err := client.UpdateTestCase(cmd.Context(), projectID, strings.TrimSpace(args[0]), input)
			if err != nil {
				return classifyAPIError(err, "failed to update test case")
			}
			if printer.JSON {
				return printer.EmitJSON(testCase)
			}

			fmt.Printf("Updated test case %s\n", testCase.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&name, "name", "", "Test case name")
	cmd.Flags().StringVar(&instructions, "instructions", "", "Test instructions")
	cmd.Flags().StringVar(&description, "description", "", "Test case description")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Tag (repeatable)")
	cmd.Flags().BoolVar(&quarantined, "quarantined", false, "Set quarantine state")
	cmd.Flags().BoolVar(&needsReview, "needs-review", false, "Set needs review state")
	cmd.Flags().StringVar(&minEnvVersion, "min-env-version", "", "Minimum supported environment version")
	cmd.Flags().StringVar(&createdByAgent, "created-by-agent", "", "Created by agent template id")
	cmd.Flags().StringVar(&createdDuringExecution, "created-during-execution", "", "Created during execution id")

	return cmd
}

func newTestcasesDeleteCommand(app *App) *cobra.Command {
	var project string
	var yes bool

	cmd := &cobra.Command{
		Use:   "delete <testcase-id>",
		Short: "Delete a test case",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return usageError("pass --yes to confirm test case deletion", nil)
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
			testCaseID := strings.TrimSpace(args[0])
			if err := client.DeleteTestCase(cmd.Context(), projectID, testCaseID); err != nil {
				return classifyAPIError(err, "failed to delete test case")
			}

			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"project_id":  projectID,
					"testcase_id": testCaseID,
					"deleted":     true,
				})
			}

			fmt.Printf("Deleted test case %s\n", testCaseID)
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm deletion")

	return cmd
}

func newTestcasesTagsSetCommand(app *App) *cobra.Command {
	var project string
	var tags []string
	var clear bool

	cmd := &cobra.Command{
		Use:   "set <testcase-id>",
		Short: "Set test case tags",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			normalizedTags := nonEmptyValues(tags)
			if clear && len(normalizedTags) > 0 {
				return usageError("use either --tag or --clear, not both", nil)
			}
			if !clear && len(normalizedTags) == 0 {
				return usageError("provide at least one --tag or pass --clear", nil)
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
			testCaseID := strings.TrimSpace(args[0])
			tagsValue := strings.Join(normalizedTags, ",")
			if clear {
				tagsValue = ""
				normalizedTags = []string{}
			}

			if err := client.UpdateTestCaseTags(cmd.Context(), projectID, testCaseID, tagsValue); err != nil {
				return classifyAPIError(err, "failed to update test case tags")
			}

			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"project_id":  projectID,
					"testcase_id": testCaseID,
					"tags":        normalizedTags,
					"updated":     true,
				})
			}

			fmt.Printf("Updated tags for test case %s\n", testCaseID)
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringArrayVar(&tags, "tag", nil, "Tag (repeatable)")
	cmd.Flags().BoolVar(&clear, "clear", false, "Clear all tags")

	return cmd
}

func newTestcasesExecuteCommand(app *App) *cobra.Command {
	var project string

	cmd := &cobra.Command{
		Use:   "run <testcase-id>",
		Short: "Run a single test case",
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

			testCaseID := strings.TrimSpace(args[0])
			resp, err := client.ExecuteTestCase(cmd.Context(), projectID, testCaseID)
			if err != nil {
				return classifyAPIError(err, "failed to execute test case")
			}
			if printer.JSON {
				return printer.EmitJSON(resp)
			}

			fmt.Printf("Execution requested for test case %s\n", resp.TestCaseID)
			fmt.Printf("Execution ID: %s\n", resp.ExecutionID)
			fmt.Printf("Status: %s\n", resp.Status)
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	return cmd
}

func newTestcasesExecuteBulkCommand(app *App) *cobra.Command {
	var project string
	var ids []string

	cmd := &cobra.Command{
		Use:   "run-many",
		Short: "Run multiple test cases",
		RunE: func(cmd *cobra.Command, args []string) error {
			normalizedIDs := nonEmptyValues(ids)
			if len(normalizedIDs) == 0 {
				return usageError("provide at least one --id", nil)
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

			resp, err := client.ExecuteTestCasesBulk(cmd.Context(), projectID, normalizedIDs)
			if err != nil {
				return classifyProjectRouteError(err, resolved.Project, resolved.Profile, "failed to execute test cases")
			}
			if printer.JSON {
				return printer.EmitJSON(resp)
			}

			fmt.Printf("Executed test cases: success=%d failure=%d\n", resp.SuccessCount, resp.FailureCount)
			for _, execution := range resp.Executions {
				fmt.Printf("- testcase=%s execution=%s status=%s\n",
					execution.TestCaseID,
					ptrStringOrDash(execution.ExecutionID),
					execution.Status,
				)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringArrayVar(&ids, "id", nil, "Test case id (repeatable)")
	return cmd
}

func timeStringOrDash(value *time.Time) string {
	if value == nil {
		return "-"
	}
	return value.Format("2006-01-02T15:04:05Z07:00")
}

func nonEmptyValues(values []string) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		trimmed := strings.TrimSpace(value)
		if trimmed == "" {
			continue
		}
		result = append(result, trimmed)
	}
	return result
}
