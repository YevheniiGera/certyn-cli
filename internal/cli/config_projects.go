package cli

import (
	"fmt"
	"sort"
	"strings"

	"github.com/certyn/certyn-cli/internal/config"
	"github.com/certyn/certyn-cli/internal/output"
	"github.com/spf13/cobra"
)

func newConfigProjectsCommand(app *App) *cobra.Command {
	projectsCmd := &cobra.Command{
		Use:   "projects",
		Short: "Manage local project slug-to-id mappings",
	}

	projectsCmd.AddCommand(newConfigProjectsMapCommand(app))
	projectsCmd.AddCommand(newConfigProjectsUnmapCommand(app))
	projectsCmd.AddCommand(newConfigProjectsListCommand(app))
	projectsCmd.AddCommand(newConfigProjectsGetCommand(app))

	return projectsCmd
}

func newConfigProjectsMapCommand(app *App) *cobra.Command {
	var profile string
	var slug string
	var id string

	cmd := &cobra.Command{
		Use:   "map",
		Short: "Map a project slug to a project ID in local config",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireValue("slug", slug); err != nil {
				return err
			}
			if err := requireValue("id", id); err != nil {
				return err
			}

			cfg, err := app.ConfigManager()
			if err != nil {
				return err
			}
			resolved, err := cfg.Resolve(config.ResolveInput{Profile: profile})
			if err != nil {
				return usageError("failed to resolve profile", err)
			}

			cfg.SetProjectMapping(resolved.Profile, slug, id)
			if err := cfg.Save(); err != nil {
				return usageError("failed to save config", err)
			}

			printer := output.Printer{JSON: app.flags.JSON}
			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"profile": resolved.Profile,
					"slug":    strings.TrimSpace(strings.ToLower(slug)),
					"id":      strings.TrimSpace(id),
					"mapped":  true,
				})
			}
			fmt.Printf("Mapped project slug '%s' -> '%s' in profile '%s'\n",
				strings.TrimSpace(strings.ToLower(slug)),
				strings.TrimSpace(id),
				resolved.Profile,
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&profile, "profile", "", "Profile name override")
	cmd.Flags().StringVar(&slug, "slug", "", "Project slug")
	cmd.Flags().StringVar(&id, "id", "", "Project ID")
	return cmd
}

func newConfigProjectsUnmapCommand(app *App) *cobra.Command {
	var profile string
	var slug string

	cmd := &cobra.Command{
		Use:   "unmap",
		Short: "Remove a project slug mapping from local config",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireValue("slug", slug); err != nil {
				return err
			}

			cfg, err := app.ConfigManager()
			if err != nil {
				return err
			}
			resolved, err := cfg.Resolve(config.ResolveInput{Profile: profile})
			if err != nil {
				return usageError("failed to resolve profile", err)
			}

			cfg.DeleteProjectMapping(resolved.Profile, slug)
			if err := cfg.Save(); err != nil {
				return usageError("failed to save config", err)
			}

			printer := output.Printer{JSON: app.flags.JSON}
			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"profile": resolved.Profile,
					"slug":    strings.TrimSpace(strings.ToLower(slug)),
					"removed": true,
				})
			}
			fmt.Printf("Removed mapping for slug '%s' in profile '%s'\n",
				strings.TrimSpace(strings.ToLower(slug)),
				resolved.Profile,
			)
			return nil
		},
	}

	cmd.Flags().StringVar(&profile, "profile", "", "Profile name override")
	cmd.Flags().StringVar(&slug, "slug", "", "Project slug")
	return cmd
}

func newConfigProjectsListCommand(app *App) *cobra.Command {
	var profile string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List local project slug-to-id mappings",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := app.ConfigManager()
			if err != nil {
				return err
			}
			resolved, err := cfg.Resolve(config.ResolveInput{Profile: profile})
			if err != nil {
				return usageError("failed to resolve profile", err)
			}

			mappings := cfg.ListProjectMappings(resolved.Profile)
			printer := output.Printer{JSON: app.flags.JSON}
			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"profile":  resolved.Profile,
					"mappings": mappings,
				})
			}

			slugs := make([]string, 0, len(mappings))
			for slug := range mappings {
				slugs = append(slugs, slug)
			}
			sort.Strings(slugs)
			for _, slug := range slugs {
				fmt.Printf("%s -> %s\n", slug, mappings[slug])
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&profile, "profile", "", "Profile name override")
	return cmd
}

func newConfigProjectsGetCommand(app *App) *cobra.Command {
	var profile string
	var slug string

	cmd := &cobra.Command{
		Use:   "get",
		Short: "Get local project ID mapped to a slug",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireValue("slug", slug); err != nil {
				return err
			}

			cfg, err := app.ConfigManager()
			if err != nil {
				return err
			}
			resolved, err := cfg.Resolve(config.ResolveInput{Profile: profile})
			if err != nil {
				return usageError("failed to resolve profile", err)
			}

			id, ok := cfg.GetProjectMapping(resolved.Profile, slug)
			if !ok {
				return notFoundError(
					fmt.Sprintf("project slug '%s' is not mapped in profile '%s'", strings.TrimSpace(slug), resolved.Profile),
					nil,
				)
			}

			printer := output.Printer{JSON: app.flags.JSON}
			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"profile": resolved.Profile,
					"slug":    strings.TrimSpace(strings.ToLower(slug)),
					"id":      id,
				})
			}

			fmt.Printf("%s -> %s\n", strings.TrimSpace(strings.ToLower(slug)), id)
			return nil
		},
	}

	cmd.Flags().StringVar(&profile, "profile", "", "Profile name override")
	cmd.Flags().StringVar(&slug, "slug", "", "Project slug")
	return cmd
}
