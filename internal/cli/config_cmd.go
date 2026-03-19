package cli

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/certyn/certyn-cli/internal/config"
	"github.com/certyn/certyn-cli/internal/output"
	"github.com/spf13/cobra"
)

func newConfigCommand(app *App) *cobra.Command {
	configCmd := &cobra.Command{
		Use:   "config",
		Short: "Manage profiles and stored defaults",
	}

	configCmd.AddCommand(newConfigSetCommand(app))
	configCmd.AddCommand(newConfigUseCommand(app))
	configCmd.AddCommand(newConfigShowCommand(app))
	configCmd.AddCommand(newConfigProfilesCommand(app))
	configCmd.AddCommand(newConfigProjectsCommand(app))
	configCmd.AddCommand(newRemovedCommand("init", "init"))

	return configCmd
}

func newConfigSetCommand(app *App) *cobra.Command {
	var profile string
	var apiURL string
	var project string
	var environment string
	var apiKeyRef string
	var apiKey string
	var authIssuer string
	var authAudience string
	var authClientID string
	var accessTokenRef string
	var refreshTokenRef string

	cmd := &cobra.Command{
		Use:   "set",
		Short: "Set profile values",
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireValue("profile", profile); err != nil {
				return err
			}

			cfg, err := app.ConfigManager()
			if err != nil {
				return err
			}

			current := cfg.Data.Profiles[profile]
			if cmd.Flags().Changed("api-url") {
				current.APIURL = config.NormalizeAPIURL(apiURL)
			}
			if cmd.Flags().Changed("project") {
				if err := validateProjectSlug(project); err != nil {
					return err
				}
				current.Project = strings.TrimSpace(project)
			}
			if cmd.Flags().Changed("environment") {
				current.Environment = environment
			}
			if cmd.Flags().Changed("api-key-ref") {
				current.APIKeyRef = apiKeyRef
			}
			if cmd.Flags().Changed("auth-issuer") {
				current.AuthIssuer = strings.TrimSpace(authIssuer)
			}
			if cmd.Flags().Changed("auth-audience") {
				current.AuthAudience = strings.TrimSpace(authAudience)
			}
			if cmd.Flags().Changed("auth-client-id") {
				current.AuthClientID = strings.TrimSpace(authClientID)
			}
			if cmd.Flags().Changed("access-token-ref") {
				current.AccessTokenRef = strings.TrimSpace(accessTokenRef)
			}
			if cmd.Flags().Changed("refresh-token-ref") {
				current.RefreshTokenRef = strings.TrimSpace(refreshTokenRef)
			}

			cfg.UpsertProfile(profile, current)

			if cmd.Flags().Changed("project") {
				_, client, _, err := app.ResolveRuntime(
					config.ResolveInput{
						Profile: profile,
						APIURL:  current.APIURL,
						APIKey:  strings.TrimSpace(apiKey),
					},
					true,
				)
				if err != nil {
					return err
				}
				projectID, err := resolveProjectSlugToID(cmd.Context(), client, current.Project)
				if err != nil {
					var cmdErr *CommandError
					if errors.As(err, &cmdErr) {
						return err
					}
					return classifyAPIError(err, "failed to resolve project slug")
				}
				if current.ProjectIDs == nil {
					current.ProjectIDs = map[string]string{}
				}
				current.ProjectIDs[strings.ToLower(current.Project)] = strings.TrimSpace(projectID)
				cfg.UpsertProfile(profile, current)
			}

			if current.APIKeyRef != "" {
				secretValue := apiKey
				if secretValue == "" {
					resolved, _, _, err := app.ResolveRuntime(config.ResolveInput{APIKey: "", Profile: profile}, false)
					if err == nil {
						secretValue = resolved.APIKey
					}
				}
				if secretValue != "" {
					if err := cfg.Store.Set(current.APIKeyRef, secretValue); err != nil {
						return usageError("failed to store API key secret", err)
					}
				}
			}

			if err := cfg.Save(); err != nil {
				return usageError("failed to save config", err)
			}
			printer := output.Printer{JSON: app.flags.JSON}
			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"profile": profile,
					"updated": true,
				})
			}
			st := output.NewStyler()
			printHumanHeader(st, "ok", "Profile updated")
			printHumanField(st, "profile", profile)
			return nil
		},
	}

	cmd.Flags().StringVar(&profile, "profile", "", "Profile name")
	cmd.Flags().StringVar(&apiURL, "api-url", "", "API URL")
	cmd.Flags().StringVar(&project, "project", "", "Default project")
	cmd.Flags().StringVar(&environment, "environment", "", "Default environment")
	cmd.Flags().StringVar(&apiKeyRef, "api-key-ref", "", "Secret reference for API key")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key value to store under --api-key-ref")
	cmd.Flags().StringVar(&authIssuer, "auth-issuer", "", "Auth issuer URL")
	cmd.Flags().StringVar(&authAudience, "auth-audience", "", "Auth audience")
	cmd.Flags().StringVar(&authClientID, "auth-client-id", "", "Auth client ID")
	cmd.Flags().StringVar(&accessTokenRef, "access-token-ref", "", "Secret reference for browser access token")
	cmd.Flags().StringVar(&refreshTokenRef, "refresh-token-ref", "", "Secret reference for browser refresh token")

	return cmd
}

func newConfigUseCommand(app *App) *cobra.Command {
	return &cobra.Command{
		Use:   "use <profile>",
		Short: "Set active profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := app.ConfigManager()
			if err != nil {
				return err
			}
			if err := cfg.SetActiveProfile(args[0]); err != nil {
				return usageError("failed to set active profile", err)
			}
			if err := cfg.Save(); err != nil {
				return usageError("failed to save config", err)
			}
			printer := output.Printer{JSON: app.flags.JSON}
			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"active_profile": args[0],
					"updated":        true,
				})
			}
			st := output.NewStyler()
			printHumanHeader(st, "ok", "Active profile updated")
			printHumanField(st, "profile", args[0])
			return nil
		},
	}
}

func newConfigShowCommand(app *App) *cobra.Command {
	var profile string
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show resolved config",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, _, printer, err := app.ResolveRuntime(config.ResolveInput{Profile: profile}, false)
			if err != nil {
				return err
			}

			payload := map[string]any{
				"profile":          resolved.Profile,
				"api_url":          resolved.APIURL,
				"project":          resolved.Project,
				"environment":      resolved.Environment,
				"api_key_set":      resolved.APIKey != "",
				"access_token_set": resolved.AccessToken != "",
				"auth_issuer":      resolved.AuthIssuer,
				"auth_audience":    resolved.AuthAudience,
				"auth_client_id":   resolved.AuthClientID,
			}

			if printer.JSON {
				return printer.EmitJSON(payload)
			}

			st := output.NewStyler()
			printHumanHeader(st, "info", "Resolved config")
			printHumanField(st, "profile", resolved.Profile)
			printHumanField(st, "api url", resolved.APIURL)
			printHumanField(st, "project", valueOrDash(resolved.Project))
			printHumanField(st, "environment", valueOrDash(resolved.Environment))
			printHumanField(st, "api key", humanBool(st, resolved.APIKey != ""))
			printHumanField(st, "browser auth", humanBool(st, resolved.AccessToken != ""))
			printHumanField(st, "auth issuer", valueOrDash(resolved.AuthIssuer))
			printHumanField(st, "audience", valueOrDash(resolved.AuthAudience))
			printHumanField(st, "client id", valueOrDash(resolved.AuthClientID))
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "Profile name")
	return cmd
}

func newConfigProfilesCommand(app *App) *cobra.Command {
	profilesCmd := &cobra.Command{Use: "profiles", Short: "Profile operations"}
	profilesCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List profiles",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := app.ConfigManager()
			if err != nil {
				return err
			}
			names := make([]string, 0, len(cfg.Data.Profiles))
			for name := range cfg.Data.Profiles {
				names = append(names, name)
			}
			sort.Strings(names)
			printer := output.Printer{JSON: app.flags.JSON}
			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"active_profile": cfg.Data.ActiveProfile,
					"profiles":       names,
				})
			}
			st := output.NewStyler()
			printHumanHeader(st, "info", fmt.Sprintf("Profiles (%d)", len(names)))
			for _, name := range names {
				summary := name
				if name == cfg.Data.ActiveProfile {
					summary = st.Bold(name + " (active)")
				}
				printHumanItem(st, summary)
			}
			return nil
		},
	})
	return profilesCmd
}
