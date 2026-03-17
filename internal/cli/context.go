package cli

import (
	"errors"
	"fmt"
	"strings"

	"github.com/certyn/certyn-cli/internal/api"
	"github.com/certyn/certyn-cli/internal/config"
	"github.com/certyn/certyn-cli/internal/output"
)

type GlobalFlags struct {
	APIURL      string
	APIKey      string
	Profile     string
	Project     string
	Environment string
	JSON        bool
}

type App struct {
	flags *GlobalFlags
	cfg   *config.Manager
}

func NewApp(flags *GlobalFlags) *App {
	return &App{flags: flags}
}

func (a *App) ensureConfig() error {
	if a.cfg != nil {
		return nil
	}
	cfg, err := config.Load()
	if err != nil {
		return usageError("failed to load CLI config", err)
	}
	a.cfg = cfg
	return nil
}

func (a *App) ResolveRuntime(overrides config.ResolveInput, requireAPIKey bool) (config.Runtime, *api.Client, output.Printer, error) {
	if err := a.ensureConfig(); err != nil {
		return config.Runtime{}, nil, output.Printer{}, err
	}

	resolved, err := a.cfg.Resolve(config.ResolveInput{
		Profile:     firstNonEmpty(overrides.Profile, a.flags.Profile),
		APIURL:      firstNonEmpty(overrides.APIURL, a.flags.APIURL),
		APIKey:      firstNonEmpty(overrides.APIKey, a.flags.APIKey),
		Project:     firstNonEmpty(overrides.Project, a.flags.Project),
		Environment: firstNonEmpty(overrides.Environment, a.flags.Environment),
	})
	if err != nil {
		return config.Runtime{}, nil, output.Printer{}, usageError("failed to resolve runtime config", err)
	}

	if requireAPIKey && strings.TrimSpace(resolved.APIKey) == "" {
		return config.Runtime{}, nil, output.Printer{}, usageError(
			"missing API key",
			errors.New("provide --api-key, CERTYN_API_KEY, or a profile api_key_ref in config"),
		)
	}

	client := api.NewClient(resolved.APIURL, resolved.APIKey)
	client.SetUserAgent(fmt.Sprintf("certyn-cli/%s", Version))
	printer := output.Printer{JSON: a.flags.JSON}

	return resolved, client, printer, nil
}

func (a *App) ConfigManager() (*config.Manager, error) {
	if err := a.ensureConfig(); err != nil {
		return nil, err
	}
	return a.cfg, nil
}

func classifyAPIError(err error, fallback string) error {
	if err == nil {
		return nil
	}

	var apiErr *api.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.StatusCode {
		case 400, 409, 422:
			return usageError(fallback, apiErr)
		case 401, 403:
			return authError("authentication or authorization failed", apiErr)
		case 404:
			return notFoundError("resource not found", apiErr)
		default:
			return &CommandError{Code: ExitGateFailed, Message: fallback, Err: apiErr}
		}
	}

	return &CommandError{Code: ExitGateFailed, Message: fallback, Err: err}
}

func requireValue(name, value string) error {
	if strings.TrimSpace(value) == "" {
		return usageError(fmt.Sprintf("missing required flag --%s", name), nil)
	}
	return nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
