package cli

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/certyn/certyn-cli/internal/api"
	"github.com/certyn/certyn-cli/internal/config"
)

func resolveProjectIDAndSlug(ctx context.Context, client *api.Client, identifier string) (string, string, error) {
	id := strings.TrimSpace(identifier)
	if id == "" {
		return "", "", usageError("missing required project", nil)
	}

	overview, err := client.GetProjectsOverview(ctx)
	if err != nil {
		return "", "", classifyAPIError(err, "failed to fetch projects overview")
	}

	for _, project := range overview.Projects {
		if strings.EqualFold(project.ID, id) || strings.EqualFold(project.Slug, id) {
			return project.ID, project.Slug, nil
		}
	}

	project, err := client.GetProject(ctx, id)
	if err == nil {
		return project.ID, project.Slug, nil
	}

	return "", "", notFoundError(fmt.Sprintf("project '%s' not found", identifier), err)
}

func resolveProjectID(ctx context.Context, client *api.Client, identifier string) (string, error) {
	id, _, err := resolveProjectIDAndSlug(ctx, client, identifier)
	return id, err
}

func resolveProjectRouteIDFromConfig(app *App, identifier, profileOverride string) (string, error) {
	needle := strings.TrimSpace(identifier)
	if needle == "" {
		return "", usageError("missing required flag --project", nil)
	}

	if looksLikeProjectID(needle) {
		return needle, nil
	}

	cfg, err := app.ConfigManager()
	if err != nil {
		return "", err
	}
	resolved, err := cfg.Resolve(config.ResolveInput{Profile: profileOverride})
	if err != nil {
		return "", usageError("failed to resolve runtime config", err)
	}

	if projectID, ok := cfg.GetProjectMapping(resolved.Profile, needle); ok {
		if !looksLikeProjectID(projectID) {
			return "", usageError(
				fmt.Sprintf(
					"project slug '%s' has invalid local ID mapping in profile '%s'. rerun `certyn config set --profile %s --project %s`",
					needle,
					resolved.Profile,
					resolved.Profile,
					needle,
				),
				nil,
			)
		}
		return projectID, nil
	}

	return "", usageError(
		fmt.Sprintf(
			"project slug '%s' has no local ID mapping for profile '%s'. run `certyn config set --profile %s --project %s`",
			needle,
			resolved.Profile,
			resolved.Profile,
			needle,
		),
		nil,
	)
}

func resolveEnvironmentIDAndKey(
	ctx context.Context,
	client *api.Client,
	projectID,
	identifier,
	projectSlug,
	profile string,
) (string, string, error) {
	needle := strings.TrimSpace(identifier)
	if needle == "" {
		return "", "", usageError("missing required environment", nil)
	}

	page := 1
	for {
		resp, err := client.ListEnvironments(ctx, projectID, page, 100)
		if err != nil {
			return "", "", classifyProjectRouteError(err, projectSlug, profile, "failed to list environments")
		}

		for _, env := range resp.Items {
			if strings.EqualFold(env.ID, needle) || strings.EqualFold(env.Key, needle) {
				return env.ID, env.Key, nil
			}
		}

		if !resp.HasNextPage {
			break
		}
		page++
	}

	if env, err := client.GetEnvironment(ctx, projectID, needle); err == nil {
		return env.ID, env.Key, nil
	}

	return "", "", notFoundError(fmt.Sprintf("environment '%s' not found", identifier), nil)
}

func resolveVariableID(ctx context.Context, client *api.Client, projectID, envID, identifier string) (string, error) {
	needle := strings.TrimSpace(identifier)
	if needle == "" {
		return "", usageError("missing required variable identifier", nil)
	}

	vars, err := client.ListEnvironmentVariables(ctx, projectID, envID)
	if err != nil {
		return "", classifyAPIError(err, "failed to list environment variables")
	}

	for _, v := range vars {
		if strings.EqualFold(v.ID, needle) || strings.EqualFold(v.Name, needle) {
			return v.ID, nil
		}
	}

	return "", notFoundError(fmt.Sprintf("variable '%s' not found", identifier), nil)
}

var projectSlugPattern = regexp.MustCompile(`^[a-z0-9]+(?:-[a-z0-9]+)*$`)

func validateProjectSlug(value string) error {
	slug := strings.TrimSpace(value)
	if slug == "" {
		return usageError("missing required flag --project", nil)
	}
	if !projectSlugPattern.MatchString(slug) {
		return usageError(
			fmt.Sprintf("invalid project slug '%s': use lowercase letters, numbers, and single hyphens", value),
			nil,
		)
	}
	return nil
}

func resolveProjectSlugToID(ctx context.Context, client *api.Client, slug string) (string, error) {
	needle := strings.TrimSpace(slug)
	if err := validateProjectSlug(needle); err != nil {
		return "", err
	}

	overview, overviewErr := client.GetProjectsOverview(ctx)
	if overviewErr == nil {
		for _, project := range overview.Projects {
			if strings.EqualFold(strings.TrimSpace(project.Slug), needle) {
				return strings.TrimSpace(project.ID), nil
			}
		}
	}

	page := 1
	for {
		resp, err := client.ListProjects(ctx, page, 100)
		if err != nil {
			if overviewErr != nil {
				return "", overviewErr
			}
			return "", err
		}
		for _, project := range resp.Items {
			if strings.EqualFold(strings.TrimSpace(project.Slug), needle) {
				return strings.TrimSpace(project.ID), nil
			}
		}
		if !resp.HasNextPage {
			break
		}
		page++
	}

	return "", notFoundError(fmt.Sprintf("project slug '%s' not found", needle), nil)
}

func classifyProjectRouteError(err error, projectSlug, profile, fallback string) error {
	var apiErr *api.APIError
	if errors.As(err, &apiErr) && apiErr.StatusCode == 404 {
		slug := strings.TrimSpace(projectSlug)
		if slug != "" && !looksLikeProjectID(slug) {
			return usageError(
				fmt.Sprintf(
					"project mapping for slug '%s' in profile '%s' appears stale. rerun `certyn config set --profile %s --project %s`",
					slug,
					profile,
					profile,
					slug,
				),
				apiErr,
			)
		}
	}
	return classifyAPIError(err, fallback)
}
