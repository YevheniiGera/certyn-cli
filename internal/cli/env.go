package cli

import (
	"fmt"

	"github.com/certyn/certyn-cli/internal/api"
	"github.com/certyn/certyn-cli/internal/config"
	"github.com/spf13/cobra"
)

func newEnvCommand(app *App) *cobra.Command {
	envCmd := &cobra.Command{
		Use:     "env",
		Aliases: []string{"environments"},
		Short:   "Environment operations",
	}
	envCmd.AddCommand(newEnvListCommand(app))
	envCmd.AddCommand(newEnvGetCommand(app))
	envCmd.AddCommand(newEnvCreateCommand(app))
	envCmd.AddCommand(newEnvUpdateCommand(app))
	envCmd.AddCommand(newEnvDeleteCommand(app))
	envCmd.AddCommand(newEnvVarsCommand(app))
	return envCmd
}

func newEnvListCommand(app *App) *cobra.Command {
	var project string
	var page int
	var pageSize int

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List environments for a project",
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
			resp, err := client.ListEnvironments(cmd.Context(), projectID, page, pageSize)
			if err != nil {
				return classifyProjectRouteError(err, resolved.Project, resolved.Profile, "failed to list environments")
			}
			if printer.JSON {
				return printer.EmitJSON(resp)
			}
			for _, env := range resp.Items {
				fmt.Printf("- %s key=%s baseUrl=%s default=%t\n", env.ID, env.Key, env.BaseURL, env.IsDefault)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().IntVar(&page, "page", 1, "Page number")
	cmd.Flags().IntVar(&pageSize, "page-size", 20, "Page size")
	return cmd
}

func newEnvGetCommand(app *App) *cobra.Command {
	var project string
	cmd := &cobra.Command{
		Use:   "get <env-id-or-key>",
		Short: "Get environment details",
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
			envID, _, err := resolveEnvironmentIDAndKey(
				cmd.Context(),
				client,
				projectID,
				args[0],
				resolved.Project,
				resolved.Profile,
			)
			if err != nil {
				return err
			}
			env, err := client.GetEnvironment(cmd.Context(), projectID, envID)
			if err != nil {
				return classifyAPIError(err, "failed to get environment")
			}
			if printer.JSON {
				return printer.EmitJSON(env)
			}
			fmt.Printf("Environment %s\n", env.ID)
			fmt.Printf("Key: %s\n", env.Key)
			fmt.Printf("Base URL: %s\n", env.BaseURL)
			fmt.Printf("Version: %s\n", env.Version)
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	return cmd
}

func newEnvCreateCommand(app *App) *cobra.Command {
	var project string
	var key string
	var label string
	var baseURL string
	var version string
	var setDefault bool
	var executionTarget string
	var runnerPoolID string
	var anthropicModel string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an environment",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, client, printer, err := app.ResolveRuntime(config.ResolveInput{Project: project}, true)
			if err != nil {
				return err
			}
			if err := requireValue("project", resolved.Project); err != nil {
				return err
			}
			if err := requireValue("key", key); err != nil {
				return err
			}
			if err := requireValue("label", label); err != nil {
				return err
			}
			if err := requireValue("base-url", baseURL); err != nil {
				return err
			}
			if err := requireValue("version", version); err != nil {
				return err
			}

			projectID, err := resolveProjectRouteIDFromConfig(app, resolved.Project, resolved.Profile)
			if err != nil {
				return err
			}
			input := api.EnvironmentInput{
				Key:             key,
				Label:           label,
				BaseURL:         baseURL,
				Version:         version,
				ExecutionTarget: executionTarget,
				RunnerPoolID:    runnerPoolID,
				AnthropicModel:  anthropicModel,
			}
			if cmd.Flags().Changed("default") {
				input.IsDefault = &setDefault
			}

			env, err := client.CreateEnvironment(cmd.Context(), projectID, input)
			if err != nil {
				return classifyAPIError(err, "failed to create environment")
			}
			if printer.JSON {
				return printer.EmitJSON(env)
			}
			fmt.Printf("Created environment %s (%s)\n", env.ID, env.Key)
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&key, "key", "", "Environment key")
	cmd.Flags().StringVar(&label, "label", "", "Environment label")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "Environment base URL")
	cmd.Flags().StringVar(&version, "version", "", "Environment version")
	cmd.Flags().BoolVar(&setDefault, "default", false, "Set as default environment")
	cmd.Flags().StringVar(&executionTarget, "execution-target", "", "Execution target: cloud_managed or self_hosted")
	cmd.Flags().StringVar(&runnerPoolID, "runner-pool-id", "", "Runner pool id (self_hosted target)")
	cmd.Flags().StringVar(&anthropicModel, "anthropic-model", "", "Anthropic model")
	return cmd
}

func newEnvUpdateCommand(app *App) *cobra.Command {
	var project string
	var key string
	var label string
	var baseURL string
	var version string
	var changelog string
	var setDefault bool
	var executionTarget string
	var runnerPoolID string
	var anthropicModel string

	cmd := &cobra.Command{
		Use:   "update <env-id-or-key>",
		Short: "Update an environment",
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
			envID, _, err := resolveEnvironmentIDAndKey(
				cmd.Context(),
				client,
				projectID,
				args[0],
				resolved.Project,
				resolved.Profile,
			)
			if err != nil {
				return err
			}

			input := api.EnvironmentInput{}
			if cmd.Flags().Changed("key") {
				input.Key = key
			}
			if cmd.Flags().Changed("label") {
				input.Label = label
			}
			if cmd.Flags().Changed("base-url") {
				input.BaseURL = baseURL
			}
			if cmd.Flags().Changed("version") {
				input.Version = version
			}
			if cmd.Flags().Changed("changelog") {
				input.Changelog = changelog
			}
			if cmd.Flags().Changed("default") {
				input.IsDefault = &setDefault
			}
			if cmd.Flags().Changed("execution-target") {
				input.ExecutionTarget = executionTarget
			}
			if cmd.Flags().Changed("runner-pool-id") {
				input.RunnerPoolID = runnerPoolID
			}
			if cmd.Flags().Changed("anthropic-model") {
				input.AnthropicModel = anthropicModel
			}

			if err := client.UpdateEnvironment(cmd.Context(), projectID, envID, input); err != nil {
				return classifyAPIError(err, "failed to update environment")
			}
			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"project_id":     projectID,
					"environment_id": envID,
					"updated":        true,
				})
			}
			fmt.Printf("Updated environment %s\n", envID)
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&key, "key", "", "Environment key")
	cmd.Flags().StringVar(&label, "label", "", "Environment label")
	cmd.Flags().StringVar(&baseURL, "base-url", "", "Environment base URL")
	cmd.Flags().StringVar(&version, "version", "", "Environment version")
	cmd.Flags().StringVar(&changelog, "changelog", "", "Version changelog")
	cmd.Flags().BoolVar(&setDefault, "default", false, "Set as default environment")
	cmd.Flags().StringVar(&executionTarget, "execution-target", "", "Execution target: cloud_managed or self_hosted")
	cmd.Flags().StringVar(&runnerPoolID, "runner-pool-id", "", "Runner pool id")
	cmd.Flags().StringVar(&anthropicModel, "anthropic-model", "", "Anthropic model")
	return cmd
}

func newEnvDeleteCommand(app *App) *cobra.Command {
	var project string
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete <env-id-or-key>",
		Short: "Delete an environment",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return usageError("pass --yes to confirm environment deletion", nil)
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
			envID, _, err := resolveEnvironmentIDAndKey(
				cmd.Context(),
				client,
				projectID,
				args[0],
				resolved.Project,
				resolved.Profile,
			)
			if err != nil {
				return err
			}
			if err := client.DeleteEnvironment(cmd.Context(), projectID, envID); err != nil {
				return classifyAPIError(err, "failed to delete environment")
			}
			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"project_id":     projectID,
					"environment_id": envID,
					"deleted":        true,
				})
			}
			fmt.Printf("Deleted environment %s\n", envID)
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm deletion")
	return cmd
}

func newEnvVarsCommand(app *App) *cobra.Command {
	varsCmd := &cobra.Command{Use: "vars", Short: "Environment variable operations"}
	varsCmd.AddCommand(newEnvVarsListCommand(app))
	varsCmd.AddCommand(newEnvVarsCreateCommand(app))
	varsCmd.AddCommand(newEnvVarsUpdateCommand(app))
	varsCmd.AddCommand(newEnvVarsDeleteCommand(app))
	return varsCmd
}

func newEnvVarsListCommand(app *App) *cobra.Command {
	var project string
	var environment string
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List environment variables",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, client, printer, err := app.ResolveRuntime(config.ResolveInput{Project: project, Environment: environment}, true)
			if err != nil {
				return err
			}
			if err := requireValue("project", resolved.Project); err != nil {
				return err
			}
			if err := requireValue("env", resolved.Environment); err != nil {
				return err
			}

			projectID, err := resolveProjectRouteIDFromConfig(app, resolved.Project, resolved.Profile)
			if err != nil {
				return err
			}
			envID, _, err := resolveEnvironmentIDAndKey(
				cmd.Context(),
				client,
				projectID,
				resolved.Environment,
				resolved.Project,
				resolved.Profile,
			)
			if err != nil {
				return err
			}

			items, err := client.ListEnvironmentVariables(cmd.Context(), projectID, envID)
			if err != nil {
				return classifyAPIError(err, "failed to list environment variables")
			}
			if printer.JSON {
				return printer.EmitJSON(items)
			}
			for _, item := range items {
				fmt.Printf("- %s name=%s\n", item.ID, item.Name)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&environment, "env", "", "Environment key or id")
	return cmd
}

func newEnvVarsCreateCommand(app *App) *cobra.Command {
	var project string
	var environment string
	var name string
	var value string
	var description string
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create an environment variable",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, client, printer, err := app.ResolveRuntime(config.ResolveInput{Project: project, Environment: environment}, true)
			if err != nil {
				return err
			}
			if err := requireValue("project", resolved.Project); err != nil {
				return err
			}
			if err := requireValue("env", resolved.Environment); err != nil {
				return err
			}
			if err := requireValue("name", name); err != nil {
				return err
			}
			if err := requireValue("value", value); err != nil {
				return err
			}

			projectID, err := resolveProjectRouteIDFromConfig(app, resolved.Project, resolved.Profile)
			if err != nil {
				return err
			}
			envID, _, err := resolveEnvironmentIDAndKey(
				cmd.Context(),
				client,
				projectID,
				resolved.Environment,
				resolved.Project,
				resolved.Profile,
			)
			if err != nil {
				return err
			}

			created, err := client.CreateEnvironmentVariable(cmd.Context(), projectID, envID, api.EnvironmentVariableInput{
				Name:        name,
				Value:       value,
				Description: description,
			})
			if err != nil {
				return classifyAPIError(err, "failed to create environment variable")
			}
			if printer.JSON {
				return printer.EmitJSON(created)
			}
			fmt.Printf("Created variable %s (%s)\n", created.Name, created.ID)
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&environment, "env", "", "Environment key or id")
	cmd.Flags().StringVar(&name, "name", "", "Variable name")
	cmd.Flags().StringVar(&value, "value", "", "Variable value")
	cmd.Flags().StringVar(&description, "description", "", "Variable description")
	return cmd
}

func newEnvVarsUpdateCommand(app *App) *cobra.Command {
	var project string
	var environment string
	var variable string
	var name string
	var value string
	var description string
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update an environment variable",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, client, printer, err := app.ResolveRuntime(config.ResolveInput{Project: project, Environment: environment}, true)
			if err != nil {
				return err
			}
			if err := requireValue("project", resolved.Project); err != nil {
				return err
			}
			if err := requireValue("env", resolved.Environment); err != nil {
				return err
			}
			if err := requireValue("var", variable); err != nil {
				return err
			}
			if err := requireValue("name", name); err != nil {
				return err
			}
			if err := requireValue("value", value); err != nil {
				return err
			}

			projectID, err := resolveProjectRouteIDFromConfig(app, resolved.Project, resolved.Profile)
			if err != nil {
				return err
			}
			envID, _, err := resolveEnvironmentIDAndKey(
				cmd.Context(),
				client,
				projectID,
				resolved.Environment,
				resolved.Project,
				resolved.Profile,
			)
			if err != nil {
				return err
			}
			variableID, err := resolveVariableID(cmd.Context(), client, projectID, envID, variable)
			if err != nil {
				return err
			}

			if err := client.UpdateEnvironmentVariable(cmd.Context(), projectID, envID, variableID, api.EnvironmentVariableInput{
				Name:        name,
				Value:       value,
				Description: description,
			}); err != nil {
				return classifyAPIError(err, "failed to update environment variable")
			}
			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"project_id":      projectID,
					"environment_id":  envID,
					"environment_var": variableID,
					"updated":         true,
				})
			}
			fmt.Printf("Updated variable %s\n", variableID)
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&environment, "env", "", "Environment key or id")
	cmd.Flags().StringVar(&variable, "var", "", "Variable id or name")
	cmd.Flags().StringVar(&name, "name", "", "Variable name")
	cmd.Flags().StringVar(&value, "value", "", "Variable value")
	cmd.Flags().StringVar(&description, "description", "", "Variable description")
	return cmd
}

func newEnvVarsDeleteCommand(app *App) *cobra.Command {
	var project string
	var environment string
	var variable string
	var yes bool
	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete an environment variable",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !yes {
				return usageError("pass --yes to confirm variable deletion", nil)
			}
			resolved, client, printer, err := app.ResolveRuntime(config.ResolveInput{Project: project, Environment: environment}, true)
			if err != nil {
				return err
			}
			if err := requireValue("project", resolved.Project); err != nil {
				return err
			}
			if err := requireValue("env", resolved.Environment); err != nil {
				return err
			}
			if err := requireValue("var", variable); err != nil {
				return err
			}
			projectID, err := resolveProjectRouteIDFromConfig(app, resolved.Project, resolved.Profile)
			if err != nil {
				return err
			}
			envID, _, err := resolveEnvironmentIDAndKey(
				cmd.Context(),
				client,
				projectID,
				resolved.Environment,
				resolved.Project,
				resolved.Profile,
			)
			if err != nil {
				return err
			}
			variableID, err := resolveVariableID(cmd.Context(), client, projectID, envID, variable)
			if err != nil {
				return err
			}
			if err := client.DeleteEnvironmentVariable(cmd.Context(), projectID, envID, variableID); err != nil {
				return classifyAPIError(err, "failed to delete environment variable")
			}
			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"project_id":      projectID,
					"environment_id":  envID,
					"environment_var": variableID,
					"deleted":         true,
				})
			}
			fmt.Printf("Deleted variable %s\n", variableID)
			return nil
		},
	}
	cmd.Flags().StringVar(&project, "project", "", "Project slug or id")
	cmd.Flags().StringVar(&environment, "env", "", "Environment key or id")
	cmd.Flags().StringVar(&variable, "var", "", "Variable id or name")
	cmd.Flags().BoolVar(&yes, "yes", false, "Confirm deletion")
	return cmd
}
