package cli

import (
	"fmt"
	"strings"

	"github.com/certyn/certyn-cli/internal/api"
	"github.com/certyn/certyn-cli/internal/config"
	"github.com/certyn/certyn-cli/internal/output"
	"github.com/spf13/cobra"
)

var (
	allowedObservationStatuses    = []string{"active", "superseded", "archived"}
	allowedObservationWikiModes   = []string{"append", "replace_block"}
	allowedObservationWikiSection = []string{"overview", "business_logic", "rules"}
)

func newObservationsCommand(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "observations",
		Short: "Observation knowledge operations",
	}

	cmd.AddCommand(newObservationsListCommand(app))
	cmd.AddCommand(newObservationsGetCommand(app))
	cmd.AddCommand(newObservationsCreateCommand(app))
	cmd.AddCommand(newObservationsSearchCommand(app))
	cmd.AddCommand(newObservationsPromoteToTicketCommand(app))
	cmd.AddCommand(newObservationsPromoteToWikiCommand(app))

	return cmd
}

func newObservationsListCommand(app *App) *cobra.Command {
	var project string
	var status string
	var envKey string
	var envVersion string
	var processRunID string
	var createdByAgent string
	var createdAfter string
	var page int
	var pageSize int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List observations for a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			status, err = normalizeEnum("status", status, allowedObservationStatuses)
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

			resp, err := client.ListObservations(cmd.Context(), projectID, api.ListObservationsParams{
				Status:                         status,
				DiscoveredInEnvironmentKey:     strings.TrimSpace(envKey),
				DiscoveredInEnvironmentVersion: strings.TrimSpace(envVersion),
				ProcessRunID:                   strings.TrimSpace(processRunID),
				CreatedByAgentTemplateID:       strings.TrimSpace(createdByAgent),
				CreatedAfter:                   strings.TrimSpace(createdAfter),
				Page:                           page,
				PageSize:                       pageSize,
			})
			if err != nil {
				return classifyProjectRouteError(err, resolved.Project, resolved.Profile, "failed to list observations")
			}
			if printer.JSON {
				return printer.EmitJSON(resp)
			}

			st := output.NewStyler()
			printHumanHeader(st, "info", fmt.Sprintf("Observations (%d)", resp.TotalCount))
			for _, observation := range resp.Items {
				printHumanItem(st, humanKVSummary(
					fmt.Sprintf("#%d", observation.Number),
					st.Status(observation.Status),
					ptrStringOrDash(observation.DiscoveredInEnvironmentKey),
					observation.Title,
				))
				printHumanField(st, "id", observation.ID)
				printHumanField(st, "version", ptrStringOrDash(observation.DiscoveredInEnvironmentVersion))
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&status, "status", "", "Observation status: active, superseded, archived")
	cmd.Flags().StringVar(&envKey, "env-key", "", "Filter by environment key")
	cmd.Flags().StringVar(&envVersion, "env-version", "", "Filter by environment version")
	cmd.Flags().StringVar(&processRunID, "process-run-id", "", "Filter by process run id")
	cmd.Flags().StringVar(&createdByAgent, "created-by-agent", "", "Filter by agent template id")
	cmd.Flags().StringVar(&createdAfter, "created-after", "", "RFC3339 timestamp lower bound")
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&pageSize, "page-size", 20, "Page size")

	return cmd
}

func newObservationsGetCommand(app *App) *cobra.Command {
	var project string

	cmd := &cobra.Command{
		Use:   "get <observation-id>",
		Short: "Get observation details",
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

			observation, err := client.GetObservation(cmd.Context(), projectID, strings.TrimSpace(args[0]))
			if err != nil {
				return classifyAPIError(err, "failed to get observation")
			}
			if printer.JSON {
				return printer.EmitJSON(observation)
			}

			st := output.NewStyler()
			printHumanHeader(st, "info", fmt.Sprintf("Observation #%d", observation.Number))
			printHumanField(st, "id", observation.ID)
			printHumanField(st, "title", observation.Title)
			printHumanField(st, "status", st.Status(observation.Status))
			printHumanField(st, "source", observation.Source)
			printHumanField(st, "env key", ptrStringOrDash(observation.DiscoveredInEnvironmentKey))
			printHumanField(st, "env version", ptrStringOrDash(observation.DiscoveredInEnvironmentVersion))
			printHumanField(st, "execution", ptrStringOrDash(observation.CreatedDuringExecutionID))
			printHumanField(st, "process run", ptrStringOrDash(observation.DiscoveredDuringRunID))
			printHumanField(st, "description", ptrStringOrDash(observation.Description))
			printHumanField(st, "attachments", fmt.Sprintf("%d", len(observation.Attachments)))
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	return cmd
}

func newObservationsCreateCommand(app *App) *cobra.Command {
	var project string
	var title string
	var description string
	var status string
	var envKey string
	var envVersion string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an observation",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireValue("title", title); err != nil {
				return err
			}

			var err error
			status, err = normalizeEnum("status", status, allowedObservationStatuses)
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

			observation, err := client.CreateObservation(cmd.Context(), projectID, api.ObservationInput{
				Title:                          strings.TrimSpace(title),
				Description:                    strings.TrimSpace(description),
				Status:                         status,
				DiscoveredInEnvironmentKey:     strings.TrimSpace(envKey),
				DiscoveredInEnvironmentVersion: strings.TrimSpace(envVersion),
			})
			if err != nil {
				return classifyProjectRouteError(err, resolved.Project, resolved.Profile, "failed to create observation")
			}
			if printer.JSON {
				return printer.EmitJSON(observation)
			}

			st := output.NewStyler()
			printHumanHeader(st, "ok", "Observation created")
			printHumanField(st, "id", observation.ID)
			printHumanField(st, "title", observation.Title)
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&title, "title", "", "Observation title")
	cmd.Flags().StringVar(&description, "description", "", "Observation description")
	cmd.Flags().StringVar(&status, "status", "active", "Observation status: active, superseded, archived")
	cmd.Flags().StringVar(&envKey, "env-key", "", "Environment key")
	cmd.Flags().StringVar(&envVersion, "env-version", "", "Environment version")
	return cmd
}

func newObservationsSearchCommand(app *App) *cobra.Command {
	var project string
	var query string
	var status string
	var envKey string
	var envVersion string

	cmd := &cobra.Command{
		Use:   "search",
		Short: "Search observations semantically",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireValue("query", query); err != nil {
				return err
			}

			var err error
			status, err = normalizeEnum("status", status, allowedObservationStatuses)
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

			resp, err := client.SearchObservations(
				cmd.Context(),
				projectID,
				strings.TrimSpace(query),
				status,
				strings.TrimSpace(envKey),
				strings.TrimSpace(envVersion),
			)
			if err != nil {
				return classifyProjectRouteError(err, resolved.Project, resolved.Profile, "failed to search observations")
			}
			if printer.JSON {
				return printer.EmitJSON(resp)
			}
			if !resp.Success {
				return usageError(valueOrDash(ptrStringOrDash(resp.Error)), nil)
			}

			st := output.NewStyler()
			printHumanHeader(st, "info", fmt.Sprintf("Matching observations (%d)", resp.TotalCount))
			for _, observation := range resp.Observations {
				score := "-"
				if observation.RelevanceScore != nil {
					score = fmt.Sprintf("%.2f", *observation.RelevanceScore)
				}
				printHumanItem(st, humanKVSummary(
					fmt.Sprintf("#%d", observation.Number),
					st.Status(observation.Status),
					"score "+score,
					observation.Title,
				))
				printHumanField(st, "id", observation.ID)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&query, "query", "", "Semantic search query")
	cmd.Flags().StringVar(&status, "status", "", "Observation status: active, superseded, archived")
	cmd.Flags().StringVar(&envKey, "env-key", "", "Filter by environment key")
	cmd.Flags().StringVar(&envVersion, "env-version", "", "Filter by environment version")
	return cmd
}

func newObservationsPromoteToTicketCommand(app *App) *cobra.Command {
	var project string
	var observationType string
	var severity string
	var labels []string

	cmd := &cobra.Command{
		Use:   "promote-to-ticket <observation-id>",
		Short: "Promote an observation into a ticket",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			observationType, err = normalizeEnum("type", observationType, []string{"bug", "improvement"})
			if err != nil {
				return err
			}
			if err := requireValue("type", observationType); err != nil {
				return err
			}
			severity, err = normalizeEnum("severity", severity, allowedIssueSeverities)
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

			ticket, err := client.PromoteObservationToTicket(cmd.Context(), projectID, strings.TrimSpace(args[0]), api.PromoteObservationToTicketInput{
				Type:     observationType,
				Severity: severity,
				Labels:   nonEmptyValues(labels),
			})
			if err != nil {
				return classifyProjectRouteError(err, resolved.Project, resolved.Profile, "failed to promote observation to ticket")
			}
			if printer.JSON {
				return printer.EmitJSON(ticket)
			}

			st := output.NewStyler()
			printHumanHeader(st, "ok", "Observation promoted to issue")
			printHumanField(st, "issue", ticket.ID)
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&observationType, "type", "", "Target ticket type: bug or improvement")
	cmd.Flags().StringVar(&severity, "severity", "", "Severity: critical, major, minor, trivial")
	cmd.Flags().StringArrayVar(&labels, "label", nil, "Label (repeatable)")
	return cmd
}

func newObservationsPromoteToWikiCommand(app *App) *cobra.Command {
	var project string
	var section string
	var mode string
	var contentMarkdown string
	var expectedExistingText string

	cmd := &cobra.Command{
		Use:   "promote-to-wiki <observation-id>",
		Short: "Promote an observation into the wiki",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			section, err = normalizeEnum("section", section, allowedObservationWikiSection)
			if err != nil {
				return err
			}
			mode, err = normalizeEnum("mode", mode, allowedObservationWikiModes)
			if err != nil {
				return err
			}
			if err := requireValue("section", section); err != nil {
				return err
			}
			if err := requireValue("mode", mode); err != nil {
				return err
			}
			if err := requireValue("content-markdown", contentMarkdown); err != nil {
				return err
			}
			if mode == "replace_block" && strings.TrimSpace(expectedExistingText) == "" {
				return usageError("expected-existing-text is required when --mode=replace_block", nil)
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

			err = client.PromoteObservationToWiki(cmd.Context(), projectID, strings.TrimSpace(args[0]), api.PromoteObservationToWikiInput{
				Section:              section,
				Mode:                 mode,
				ContentMarkdown:      contentMarkdown,
				ExpectedExistingText: strings.TrimSpace(expectedExistingText),
			})
			if err != nil {
				return classifyProjectRouteError(err, resolved.Project, resolved.Profile, "failed to promote observation to wiki")
			}
			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"project_id":     projectID,
					"observation_id": strings.TrimSpace(args[0]),
					"section":        section,
					"mode":           mode,
					"wiki_promoted":  true,
				})
			}

			st := output.NewStyler()
			printHumanHeader(st, "ok", "Observation promoted to wiki")
			printHumanField(st, "observation", strings.TrimSpace(args[0]))
			printHumanField(st, "section", section)
			printHumanField(st, "mode", mode)
			return nil
		},
	}

	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&section, "section", "", "Wiki section: overview, business_logic, rules")
	cmd.Flags().StringVar(&mode, "mode", "append", "Promotion mode: append or replace_block")
	cmd.Flags().StringVar(&contentMarkdown, "content-markdown", "", "Markdown to write into the wiki")
	cmd.Flags().StringVar(&expectedExistingText, "expected-existing-text", "", "Exact existing text to replace when using replace_block mode")
	return cmd
}
