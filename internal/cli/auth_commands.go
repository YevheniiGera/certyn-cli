package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/certyn/certyn-cli/internal/config"
	"github.com/certyn/certyn-cli/internal/output"
	"github.com/certyn/certyn-cli/internal/secretstore"
	"github.com/spf13/cobra"
)

const (
	defaultInstallHost = "https://certyn.io"
	defaultReleaseRepo = "YevheniiGera/certyn-cli"
)

type deviceCodeResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int64  `json:"expires_in"`
	Interval                int64  `json:"interval"`
}

func newLoginCommand(app *App) *cobra.Command {
	var profile string
	var issuer string
	var audience string
	var clientID string
	var noBrowser bool

	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in with your browser",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := app.ConfigManager()
			if err != nil {
				return err
			}

			profileName := firstNonEmpty(profile, app.flags.Profile, cfg.Data.ActiveProfile, config.DefaultProfileName)
			current := ensureBrowserAuthProfile(profileName, cfg.Data.Profiles[profileName])
			explicitAPIURL := strings.TrimSpace(app.flags.APIURL)
			resolved, err := cfg.Resolve(config.ResolveInput{Profile: profileName, APIURL: explicitAPIURL})
			if err != nil {
				return usageError("failed to resolve runtime config", err)
			}

			if strings.TrimSpace(explicitAPIURL) != "" {
				current.APIURL = config.NormalizeAPIURL(explicitAPIURL)
			} else if strings.TrimSpace(current.APIURL) == "" {
				current.APIURL = resolved.APIURL
			}
			current.AuthIssuer = firstNonEmpty(issuer, resolved.AuthIssuer, current.AuthIssuer, config.DefaultAuthIssuer)
			current.AuthAudience = firstNonEmpty(audience, resolved.AuthAudience, current.AuthAudience, config.InferAuthAudience(current.APIURL))
			current.AuthClientID = firstNonEmpty(clientID, resolved.AuthClientID, current.AuthClientID)

			if strings.TrimSpace(current.AuthClientID) == "" {
				return usageError("missing auth client id", errors.New("set --auth-client-id, CERTYN_AUTH_CLIENT_ID, or config auth_client_id before running `certyn login`"))
			}

			deviceCode, err := requestDeviceCode(cmd.Context(), current.AuthIssuer, current.AuthClientID, current.AuthAudience)
			if err != nil {
				return authError("failed to start browser login", err)
			}
			if !noBrowser {
				_ = openBrowser(firstNonEmpty(deviceCode.VerificationURIComplete, deviceCode.VerificationURI))
			}

			tokenSet, err := waitForDeviceLogin(cmd.Context(), current.AuthIssuer, current.AuthClientID, deviceCode)
			if err != nil {
				return authError("browser login failed", err)
			}

			if err := cfg.Store.Set(current.AccessTokenRef, tokenSet.AccessToken); err != nil {
				return authError("failed to store browser session", err)
			}
			if strings.TrimSpace(tokenSet.RefreshToken) != "" {
				if err := cfg.Store.Set(current.RefreshTokenRef, tokenSet.RefreshToken); err != nil {
					return authError("failed to store browser session", err)
				}
			}

			cfg.UpsertProfile(profileName, current)
			if cfg.Data.ActiveProfile == "" {
				cfg.Data.ActiveProfile = profileName
			}
			if err := cfg.Save(); err != nil {
				return usageError("failed to save config", err)
			}

			claims, _ := parseAuthTokenClaims(tokenSet.AccessToken)
			printer := output.Printer{JSON: app.flags.JSON}
			payload := map[string]any{
				"profile":    profileName,
				"issuer":     current.AuthIssuer,
				"audience":   current.AuthAudience,
				"subject":    claims.Subject,
				"email":      claims.Email,
				"name":       claims.Name,
				"logged_in":  true,
				"next_step":  "certyn whoami",
				"opened_url": firstNonEmpty(deviceCode.VerificationURIComplete, deviceCode.VerificationURI),
				"user_code":  deviceCode.UserCode,
			}
			if printer.JSON {
				return printer.EmitJSON(payload)
			}

			fmt.Printf("Logged in for profile '%s'\n", profileName)
			if claims.Email != "" || claims.Name != "" {
				fmt.Printf("Account: %s <%s>\n", valueOrDash(claims.Name), valueOrDash(claims.Email))
			}
			fmt.Println("Next step: certyn whoami")
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "Profile name")
	cmd.Flags().StringVar(&issuer, "auth-issuer", "", "Auth issuer URL")
	cmd.Flags().StringVar(&audience, "auth-audience", "", "Auth audience")
	cmd.Flags().StringVar(&clientID, "auth-client-id", "", "Auth client ID")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Do not open the browser automatically")
	return cmd
}

func newLogoutCommand(app *App) *cobra.Command {
	var profile string

	cmd := &cobra.Command{
		Use:   "logout",
		Short: "Clear local browser login state",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := app.ConfigManager()
			if err != nil {
				return err
			}
			profileName := firstNonEmpty(profile, app.flags.Profile, cfg.Data.ActiveProfile, config.DefaultProfileName)
			current := ensureBrowserAuthProfile(profileName, cfg.Data.Profiles[profileName])

			if err := cfg.Store.Delete(current.AccessTokenRef); err != nil && !errors.Is(err, secretstore.ErrSecretNotFound) {
				return usageError("failed to clear access token", err)
			}
			if err := cfg.Store.Delete(current.RefreshTokenRef); err != nil && !errors.Is(err, secretstore.ErrSecretNotFound) {
				return usageError("failed to clear refresh token", err)
			}

			printer := output.Printer{JSON: app.flags.JSON}
			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"profile":    profileName,
					"logged_out": true,
				})
			}

			fmt.Printf("Logged out for profile '%s'\n", profileName)
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "Profile name")
	return cmd
}

func newWhoAmICommand(app *App) *cobra.Command {
	var profile string

	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "Show active auth and runtime identity",
		RunE: func(cmd *cobra.Command, args []string) error {
			resolved, _, printer, err := app.ResolveRuntime(config.ResolveInput{Profile: profile}, false)
			if err != nil {
				return err
			}
			if strings.TrimSpace(resolved.APIKey) == "" && strings.TrimSpace(resolved.AccessToken) == "" {
				return authError("authentication required", errors.New(authConfigurationHint()))
			}

			payload := map[string]any{
				"profile":       resolved.Profile,
				"api_url":       resolved.APIURL,
				"project":       resolved.Project,
				"environment":   resolved.Environment,
				"auth_mode":     "api_key",
				"auth_issuer":   resolved.AuthIssuer,
				"auth_audience": resolved.AuthAudience,
			}

			if strings.TrimSpace(resolved.AccessToken) != "" {
				payload["auth_mode"] = "browser"
				if info, infoErr := fetchUserInfo(cmd.Context(), resolved.AuthIssuer, resolved.AccessToken); infoErr == nil {
					payload["subject"] = info.Subject
					payload["email"] = info.Email
					payload["name"] = info.Name
				} else if claims, claimsErr := parseAuthTokenClaims(resolved.AccessToken); claimsErr == nil {
					payload["subject"] = claims.Subject
					payload["email"] = claims.Email
					payload["name"] = claims.Name
				}
			}

			if printer.JSON {
				return printer.EmitJSON(payload)
			}

			st := output.NewStyler()
			printHumanHeader(st, "info", "Active identity")
			printHumanField(st, "profile", resolved.Profile)
			printHumanField(st, "auth mode", fmt.Sprintf("%v", payload["auth_mode"]))
			printHumanField(st, "api url", resolved.APIURL)
			printHumanField(st, "project", valueOrDash(resolved.Project))
			printHumanField(st, "environment", valueOrDash(resolved.Environment))
			if subject, ok := payload["subject"].(string); ok && subject != "" {
				printHumanField(st, "subject", subject)
			}
			if email, ok := payload["email"].(string); ok && email != "" {
				printHumanField(st, "email", email)
			}
			if name, ok := payload["name"].(string); ok && name != "" {
				printHumanField(st, "name", name)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "Profile name")
	return cmd
}

func newDoctorCommand(app *App) *cobra.Command {
	var profile string

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check install, config, auth, and API connectivity",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := app.ConfigManager()
			if err != nil {
				return err
			}
			resolved, client, printer, err := app.ResolveRuntime(config.ResolveInput{Profile: profile}, false)
			if err != nil {
				return err
			}

			checks := []map[string]any{}
			record := func(name string, ok bool, detail string) {
				checks = append(checks, map[string]any{
					"name":   name,
					"ok":     ok,
					"detail": detail,
				})
			}

			record("config", true, cfg.Path)

			tempRef := fmt.Sprintf("doctor_%d", time.Now().UnixNano())
			if err := cfg.Store.Set(tempRef, "ok"); err != nil {
				record("secure_store", false, err.Error())
			} else if value, getErr := cfg.Store.Get(tempRef); getErr != nil || value != "ok" {
				record("secure_store", false, "round-trip failed")
			} else if deleteErr := cfg.Store.Delete(tempRef); deleteErr != nil {
				record("secure_store", false, deleteErr.Error())
			} else {
				record("secure_store", true, "round-trip ok")
			}

			record("api_url", strings.TrimSpace(resolved.APIURL) != "", resolved.APIURL)
			authMode := "none"
			if resolved.AccessToken != "" {
				authMode = "browser"
			} else if resolved.APIKey != "" {
				authMode = "api_key"
			}
			record("auth", authMode != "none", authMode)

			if authMode == "none" {
				record("api_connectivity", false, authConfigurationHint())
			} else if _, apiErr := client.GetProjectsOverview(cmd.Context()); apiErr != nil {
				record("api_connectivity", false, apiErr.Error())
			} else {
				record("api_connectivity", true, "projects overview ok")
			}

			payload := map[string]any{
				"profile":     resolved.Profile,
				"api_url":     resolved.APIURL,
				"project":     resolved.Project,
				"environment": resolved.Environment,
				"checks":      checks,
			}
			if printer.JSON {
				return printer.EmitJSON(payload)
			}

			st := output.NewStyler()
			printHumanHeader(st, "info", "Doctor")
			for _, check := range checks {
				kind := "ok"
				if passed, _ := check["ok"].(bool); !passed {
					kind = "fail"
				}
				fmt.Printf("%s %s: %s\n", st.Badge(kind), check["name"], check["detail"])
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&profile, "profile", "", "Profile name")
	return cmd
}

func newInitCommand(app *App) *cobra.Command {
	var profile string
	var apiURL string
	var project string
	var environment string
	var authClientID string
	var authIssuer string
	var authAudience string
	var apiKeyRef string
	var apiKey string
	var useAPIKey bool
	var loginAfter bool

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize config with a guided first-run setup",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := app.ConfigManager()
			if err != nil {
				return err
			}

			profileName := firstNonEmpty(profile, app.flags.Profile, cfg.Data.ActiveProfile, config.DefaultProfileName)
			current := ensureBrowserAuthProfile(profileName, cfg.Data.Profiles[profileName])
			reader := bufio.NewReader(cmd.InOrStdin())
			writer := cmd.ErrOrStderr()

			apiURL = firstNonEmpty(apiURL, current.APIURL, config.DefaultAPIURL)
			if !isNonInteractiveAuthContext() && strings.TrimSpace(profile) == "" {
				profileName = promptString(reader, writer, "Profile", profileName)
				apiURL = promptString(reader, writer, "API URL", apiURL)
				authChoice := "browser"
				if useAPIKey {
					authChoice = "api-key"
				}
				authChoice = promptString(reader, writer, "Auth mode (browser/api-key)", authChoice)
				useAPIKey = strings.EqualFold(strings.TrimSpace(authChoice), "api-key")
				if !useAPIKey {
					loginAfter = true
				}
			}

			current.APIURL = config.NormalizeAPIURL(apiURL)
			current.Project = strings.TrimSpace(firstNonEmpty(project, current.Project))
			current.Environment = strings.TrimSpace(firstNonEmpty(environment, current.Environment))
			current.AuthIssuer = strings.TrimSpace(firstNonEmpty(authIssuer, current.AuthIssuer, config.DefaultAuthIssuer))
			current.AuthAudience = strings.TrimSpace(firstNonEmpty(authAudience, current.AuthAudience, config.InferAuthAudience(current.APIURL)))
			current.AuthClientID = strings.TrimSpace(firstNonEmpty(authClientID, current.AuthClientID))
			if strings.TrimSpace(apiKeyRef) != "" {
				current.APIKeyRef = strings.TrimSpace(apiKeyRef)
			}
			if useAPIKey && strings.TrimSpace(current.APIKeyRef) == "" {
				current.APIKeyRef = fmt.Sprintf("%s_api_key", profileName)
			}

			cfg.UpsertProfile(profileName, current)
			cfg.Data.ActiveProfile = profileName
			if strings.TrimSpace(apiKey) != "" && current.APIKeyRef != "" {
				if err := cfg.Store.Set(current.APIKeyRef, apiKey); err != nil {
					return usageError("failed to store API key", err)
				}
			}
			if err := cfg.Save(); err != nil {
				return usageError("failed to save config", err)
			}

			if loginAfter && !useAPIKey {
				loginCmd := newLoginCommand(app)
				loginArgs := []string{"--profile", profileName}
				if current.AuthClientID != "" {
					loginArgs = append(loginArgs, "--auth-client-id", current.AuthClientID)
				}
				loginCmd.SetArgs(loginArgs)
				return loginCmd.Execute()
			}

			printer := output.Printer{JSON: app.flags.JSON}
			payload := map[string]any{
				"profile":     profileName,
				"api_url":     current.APIURL,
				"project":     current.Project,
				"environment": current.Environment,
				"auth_mode":   map[bool]string{true: "api_key", false: "browser"}[useAPIKey],
				"initialized": true,
			}
			if printer.JSON {
				return printer.EmitJSON(payload)
			}
			fmt.Printf("Profile '%s' initialized\n", profileName)
			if useAPIKey {
				fmt.Println("Next step: certyn doctor")
			} else {
				fmt.Println("Next step: certyn login")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&profile, "profile", "", "Profile name")
	cmd.Flags().StringVar(&apiURL, "api-url", "", "API URL")
	cmd.Flags().StringVar(&project, "project", "", "Default project")
	cmd.Flags().StringVar(&environment, "environment", "", "Default environment")
	cmd.Flags().StringVar(&authClientID, "auth-client-id", "", "Auth client ID")
	cmd.Flags().StringVar(&authIssuer, "auth-issuer", "", "Auth issuer URL")
	cmd.Flags().StringVar(&authAudience, "auth-audience", "", "Auth audience")
	cmd.Flags().StringVar(&apiKeyRef, "api-key-ref", "", "Secret reference for API key")
	cmd.Flags().StringVar(&apiKey, "api-key", "", "API key value to store")
	cmd.Flags().BoolVar(&useAPIKey, "use-api-key", false, "Configure API-key auth instead of browser login")
	cmd.Flags().BoolVar(&loginAfter, "login", false, "Run browser login immediately after init")
	return cmd
}

func newUpdateCommand(app *App) *cobra.Command {
	var version string
	var apply bool
	var host string
	var repo string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Check for a newer release and optionally install it",
		RunE: func(cmd *cobra.Command, args []string) error {
			targetVersion := strings.TrimSpace(version)
			if targetVersion == "" {
				latest, err := fetchLatestReleaseTag(cmd.Context(), repo)
				if err != nil {
					return &CommandError{Code: ExitGateFailed, Message: "failed to resolve latest release", Err: err}
				}
				targetVersion = latest
			}

			payload := map[string]any{
				"current_version": Version,
				"target_version":  targetVersion,
				"up_to_date":      strings.EqualFold(targetVersion, Version),
			}
			if !apply {
				payload["install_command"] = installCommandForCurrentOS(strings.TrimRight(host, "/"), targetVersion)
				if app.flags.JSON {
					return output.Printer{JSON: true}.EmitJSON(payload)
				}
				fmt.Printf("Current version: %s\n", Version)
				fmt.Printf("Target version: %s\n", targetVersion)
				fmt.Printf("Install command: %s\n", payload["install_command"])
				return nil
			}

			if err := runInstallCommand(strings.TrimRight(host, "/"), targetVersion); err != nil {
				return &CommandError{Code: ExitGateFailed, Message: "failed to run installer", Err: err}
			}
			if app.flags.JSON {
				payload["updated"] = true
				return output.Printer{JSON: true}.EmitJSON(payload)
			}
			fmt.Printf("Update command applied for %s\n", targetVersion)
			return nil
		},
	}

	cmd.Flags().StringVar(&version, "version", "", "Target release version")
	cmd.Flags().BoolVar(&apply, "apply", false, "Run the install command instead of only printing it")
	cmd.Flags().StringVar(&host, "host", defaultInstallHost, "Install host base URL")
	cmd.Flags().StringVar(&repo, "repo", defaultReleaseRepo, "Release repository for version checks")
	return cmd
}

func newUninstallCommand(app *App) *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove the local certyn binary",
		RunE: func(cmd *cobra.Command, args []string) error {
			exePath, err := os.Executable()
			if err != nil {
				return usageError("failed to resolve current executable", err)
			}

			if runtime.GOOS == "windows" {
				printer := output.Printer{JSON: app.flags.JSON}
				payload := map[string]any{
					"removed":     false,
					"binary_path": exePath,
					"next_step":   "Delete the binary manually and remove its install directory from your user PATH if needed.",
				}
				if printer.JSON {
					return printer.EmitJSON(payload)
				}
				fmt.Printf("Windows uninstall is manual for now.\nBinary: %s\n", exePath)
				fmt.Println("Remove the binary and clean the install directory from your user PATH if needed.")
				return nil
			}

			if !yes && !confirmPrompt(cmd.InOrStdin(), cmd.ErrOrStderr(), fmt.Sprintf("Remove %s?", exePath)) {
				return usageError("uninstall cancelled", nil)
			}
			if err := os.Remove(exePath); err != nil {
				return usageError("failed to remove binary", err)
			}

			printer := output.Printer{JSON: app.flags.JSON}
			if printer.JSON {
				return printer.EmitJSON(map[string]any{
					"removed":     true,
					"binary_path": exePath,
				})
			}

			fmt.Printf("Removed %s\n", exePath)
			return nil
		},
	}
	cmd.Flags().BoolVar(&yes, "yes", false, "Remove the binary without confirmation")
	return cmd
}

func requestDeviceCode(ctx context.Context, issuer, clientID, audience string) (deviceCodeResponse, error) {
	form := url.Values{}
	form.Set("client_id", strings.TrimSpace(clientID))
	form.Set("scope", authScopes)
	form.Set("audience", strings.TrimSpace(audience))

	deviceCodeURL := strings.TrimSuffix(strings.TrimSpace(issuer), "/") + "/oauth/device/code"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, deviceCodeURL, strings.NewReader(form.Encode()))
	if err != nil {
		return deviceCodeResponse{}, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return deviceCodeResponse{}, err
	}
	defer resp.Body.Close()

	var payload deviceCodeResponse
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return deviceCodeResponse{}, fmt.Errorf("decode device code response: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return deviceCodeResponse{}, fmt.Errorf("device code request failed with status %d", resp.StatusCode)
	}
	if payload.DeviceCode == "" {
		return deviceCodeResponse{}, errors.New("device code response was empty")
	}
	if payload.Interval <= 0 {
		payload.Interval = 5
	}
	return payload, nil
}

func waitForDeviceLogin(ctx context.Context, issuer, clientID string, deviceCode deviceCodeResponse) (authTokenResponse, error) {
	tokenURL := strings.TrimSuffix(strings.TrimSpace(issuer), "/") + "/oauth/token"
	deadline := time.Now().Add(time.Duration(deviceCode.ExpiresIn) * time.Second)
	interval := time.Duration(deviceCode.Interval) * time.Second

	for time.Now().Before(deadline) {
		form := url.Values{}
		form.Set("grant_type", "urn:ietf:params:oauth:grant-type:device_code")
		form.Set("device_code", deviceCode.DeviceCode)
		form.Set("client_id", clientID)

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
		if err != nil {
			return authTokenResponse{}, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("Accept", "application/json")

		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			return authTokenResponse{}, err
		}

		var payload authTokenResponse
		decodeErr := json.NewDecoder(resp.Body).Decode(&payload)
		resp.Body.Close()
		if decodeErr != nil {
			return authTokenResponse{}, fmt.Errorf("decode login response: %w", decodeErr)
		}
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			if strings.TrimSpace(payload.AccessToken) == "" {
				return authTokenResponse{}, errors.New("login returned no access token")
			}
			return payload, nil
		}

		switch strings.TrimSpace(payload.Error) {
		case "authorization_pending":
			time.Sleep(interval)
			continue
		case "slow_down":
			interval += 5 * time.Second
			time.Sleep(interval)
			continue
		default:
			return authTokenResponse{}, errors.New(firstNonEmpty(payload.Description, payload.Error, "browser login failed"))
		}
	}

	return authTokenResponse{}, errors.New("browser login timed out")
}

func openBrowser(target string) error {
	if strings.TrimSpace(target) == "" {
		return nil
	}

	var name string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		name, args = "open", []string{target}
	case "windows":
		name, args = "rundll32", []string{"url.dll,FileProtocolHandler", target}
	default:
		name, args = "xdg-open", []string{target}
	}
	return exec.Command(name, args...).Start()
}

func promptString(reader *bufio.Reader, writer io.Writer, label, defaultValue string) string {
	if strings.TrimSpace(defaultValue) == "" {
		fmt.Fprintf(writer, "%s: ", label)
	} else {
		fmt.Fprintf(writer, "%s [%s]: ", label, defaultValue)
	}
	raw, _ := reader.ReadString('\n')
	value := strings.TrimSpace(raw)
	if value == "" {
		return strings.TrimSpace(defaultValue)
	}
	return value
}

func confirmPrompt(reader io.Reader, writer io.Writer, message string) bool {
	fmt.Fprintf(writer, "%s [y/N]: ", message)
	buf := bufio.NewReader(reader)
	raw, _ := buf.ReadString('\n')
	value := strings.ToLower(strings.TrimSpace(raw))
	return value == "y" || value == "yes"
}

func fetchLatestReleaseTag(ctx context.Context, repo string) (string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", strings.TrimSpace(repo))
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, apiURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "certyn-cli/"+Version)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var payload struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 || strings.TrimSpace(payload.TagName) == "" {
		return "", fmt.Errorf("latest release lookup failed with status %d", resp.StatusCode)
	}
	return strings.TrimSpace(payload.TagName), nil
}

func installCommandForCurrentOS(host, version string) string {
	if runtime.GOOS == "windows" {
		return fmt.Sprintf(`iex "& { $(iwr -useb %s/install.ps1) } -Version %s"`, host, version)
	}
	return fmt.Sprintf("curl -fsSL %s/install | bash -s -- --version %s", host, version)
}

func runInstallCommand(host, version string) error {
	if runtime.GOOS == "windows" {
		command := fmt.Sprintf(`& { $(iwr -useb %s/install.ps1) } -Version %s`, host, version)
		cmd := exec.Command("powershell", "-NoProfile", "-Command", command)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		return cmd.Run()
	}

	command := fmt.Sprintf("curl -fsSL %s/install | bash -s -- --version %s", host, version)
	cmd := exec.Command("sh", "-c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
