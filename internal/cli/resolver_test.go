package cli

import (
	"errors"
	"strings"
	"testing"

	"github.com/certyn/certyn-cli/internal/config"
)

func TestResolveProjectRouteIDFromConfigPassthroughUUIDAndULID(t *testing.T) {
	app := &App{
		flags: &GlobalFlags{},
		cfg: &config.Manager{
			Data: &config.File{
				ActiveProfile: "dev",
				Profiles: map[string]config.Profile{
					"dev": {},
				},
			},
		},
	}

	uuid := "11111111-2222-4333-8444-555555555555"
	got, err := resolveProjectRouteIDFromConfig(app, uuid, "dev")
	if err != nil {
		t.Fatalf("expected UUID passthrough, got error: %v", err)
	}
	if got != uuid {
		t.Fatalf("expected %q, got %q", uuid, got)
	}

	ulid := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	got, err = resolveProjectRouteIDFromConfig(app, ulid, "dev")
	if err != nil {
		t.Fatalf("expected ULID passthrough, got error: %v", err)
	}
	if got != ulid {
		t.Fatalf("expected %q, got %q", ulid, got)
	}
}

func TestResolveProjectRouteIDFromConfigUsesLocalSlugMapping(t *testing.T) {
	const projectID = "11111111-2222-4333-8444-555555555555"

	cfg := &config.Manager{
		Data: &config.File{
			ActiveProfile: "dev",
			Profiles: map[string]config.Profile{
				"dev": {
					ProjectIDs: map[string]string{
						"my-project": projectID,
					},
				},
			},
		},
	}
	app := &App{flags: &GlobalFlags{}, cfg: cfg}

	got, err := resolveProjectRouteIDFromConfig(app, "my-project", "dev")
	if err != nil {
		t.Fatalf("expected mapping success, got error: %v", err)
	}
	if got != projectID {
		t.Fatalf("expected %s, got %q", projectID, got)
	}
}

func TestResolveProjectRouteIDFromConfigMissingMappingReturnsUsageError(t *testing.T) {
	cfg := &config.Manager{
		Data: &config.File{
			ActiveProfile: "dev",
			Profiles: map[string]config.Profile{
				"dev": {},
			},
		},
	}
	app := &App{flags: &GlobalFlags{}, cfg: cfg}

	_, err := resolveProjectRouteIDFromConfig(app, "my-project", "dev")
	if err == nil {
		t.Fatal("expected missing mapping error")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitUsage {
		t.Fatalf("expected usage exit code %d, got %d", ExitUsage, cmdErr.Code)
	}
	if !strings.Contains(cmdErr.Error(), "config set --profile dev --project my-project") {
		t.Fatalf("expected mapping hint in error, got %q", cmdErr.Error())
	}
}

func TestResolveProjectRouteIDFromConfigInvalidMappingReturnsUsageError(t *testing.T) {
	cfg := &config.Manager{
		Data: &config.File{
			ActiveProfile: "dev",
			Profiles: map[string]config.Profile{
				"dev": {
					ProjectIDs: map[string]string{
						"my-project": "not-an-id",
					},
				},
			},
		},
	}
	app := &App{flags: &GlobalFlags{}, cfg: cfg}

	_, err := resolveProjectRouteIDFromConfig(app, "my-project", "dev")
	if err == nil {
		t.Fatal("expected invalid mapping error")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitUsage {
		t.Fatalf("expected usage exit code %d, got %d", ExitUsage, cmdErr.Code)
	}
	if !strings.Contains(cmdErr.Error(), "config set --profile dev --project my-project") {
		t.Fatalf("expected remediaton hint in error, got %q", cmdErr.Error())
	}
}
