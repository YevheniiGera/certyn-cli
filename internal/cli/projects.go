package cli

import (
	"fmt"

	"github.com/certyn/certyn-cli/internal/api"
	"github.com/certyn/certyn-cli/internal/config"
	"github.com/spf13/cobra"
)

func newProjectsCommand(app *App) *cobra.Command {
	projectsCmd := &cobra.Command{
		Use:   "projects",
		Short: "Project operations",
	}

	projectsCmd.AddCommand(newProjectsListCommand(app))
	projectsCmd.AddCommand(newProjectsGetCommand(app))
	projectsCmd.AddCommand(newProjectsCreateCommand(app))
	projectsCmd.AddCommand(newProjectsUpdateCommand(app))
	projectsCmd.AddCommand(newProjectsDeleteCommand(app))

	return projectsCmd
}

func newProjectsListCommand(app *App) *cobra.Command {
	var page int
	var pageSize int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List projects",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, client, printer, err := app.ResolveRuntime(config.ResolveInput{}, true)
			if err != nil {
				return err
			}
			resp, err := client.ListProjects(cmd.Context(), page, pageSize)
			if err != nil {
				return classifyAPIError(err, "failed to list projects")
			}
			if printer.JSON {
				return printer.EmitJSON(resp)
			}
			for _, project := range resp.Items {
				fmt.Printf("- %s slug=%s name=%s\n", project.ID, project.Slug, project.Name)
			}
			return nil
		},
	}

	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&pageSize, "page-size", 20, "Page size")

	return cmd
}

func newProjectsGetCommand(app *App) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get <project-id-or-slug>",
		Short: "Get project details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, client, printer, err := app.ResolveRuntime(config.ResolveInput{}, true)
			if err != nil {
				return err
			}
			projectID, err := resolveProjectRouteIDFromConfig(app, args[0], resolved.Profile)
			if err != nil {
				return err
			}
			project, err := client.GetProject(cmd.Context(), projectID)
			if err != nil {
				return classifyProjectRouteError(err, args[0], resolved.Profile, "failed to get project")
			}
			if printer.JSON {
				return printer.EmitJSON(project)
			}
			fmt.Printf("Project %s\n", project.ID)
			fmt.Printf("Name: %s\n", project.Name)
			fmt.Printf("Slug: %s\n", project.Slug)
			return nil
		},
	}
	return cmd
}

func newProjectsCreateCommand(app *App) *cobra.Command {
	var name string
	var slug string
	var description string
	var color string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a project",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireValue("name", name); err != nil {
				return err
			}
			_, client, printer, err := app.ResolveRuntime(config.ResolveInput{}, true)
			if err != nil {
				return err
			}
			project, err := client.CreateProject(cmd.Context(), api.ProjectInput{
				Name:        name,
				Slug:        slug,
				Description: description,
				Color:       color,
			})
			if err != nil {
				return classifyAPIError(err, "failed to create project")
			}

			if printer.JSON {
				return printer.EmitJSON(project)
			}
			fmt.Printf("Created project %s (%s)\n", project.ID, project.Slug)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Project name")
	cmd.Flags().StringVar(&slug, "slug", "", "Project slug")
	cmd.Flags().StringVar(&description, "description", "", "Project description")
	cmd.Flags().StringVar(&color, "color", "", "Project color")

	return cmd
}

func newProjectsUpdateCommand(app *App) *cobra.Command {
	var name string
	var slug string
	var description string
	var color string

	cmd := &cobra.Command{
		Use:   "update <project-id-or-slug>",
		Short: "Update a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, client, printer, err := app.ResolveRuntime(config.ResolveInput{}, true)
			if err != nil {
				return err
			}
			projectID, err := resolveProjectRouteIDFromConfig(app, args[0], resolved.Profile)
			if err != nil {
				return err
			}
			if err := client.UpdateProject(cmd.Context(), projectID, api.ProjectInput{
				Name:        name,
				Slug:        slug,
				Description: description,
				Color:       color,
			}); err != nil {
				return classifyProjectRouteError(err, args[0], resolved.Profile, "failed to update project")
			}
			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"project_id": projectID,
					"updated":    true,
				})
			}
			fmt.Printf("Updated project %s\n", projectID)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Project name")
	cmd.Flags().StringVar(&slug, "slug", "", "Project slug")
	cmd.Flags().StringVar(&description, "description", "", "Project description")
	cmd.Flags().StringVar(&color, "color", "", "Project color")

	return cmd
}

func newProjectsDeleteCommand(app *App) *cobra.Command {
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <project-id-or-slug>",
		Short: "Delete a project",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return usageError("pass --yes to confirm project deletion", nil)
			}
			resolved, client, printer, err := app.ResolveRuntime(config.ResolveInput{}, true)
			if err != nil {
				return err
			}
			projectID, err := resolveProjectRouteIDFromConfig(app, args[0], resolved.Profile)
			if err != nil {
				return err
			}
			if err := client.DeleteProject(cmd.Context(), projectID); err != nil {
				return classifyProjectRouteError(err, args[0], resolved.Profile, "failed to delete project")
			}
			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"project_id": projectID,
					"deleted":    true,
				})
			}
			fmt.Printf("Deleted project %s\n", projectID)
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm project deletion")
	return cmd
}
