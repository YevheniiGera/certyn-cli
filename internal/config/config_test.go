package config

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/certyn/certyn-cli/internal/secretstore"
	"gopkg.in/yaml.v3"
)

type fakeStore struct {
	items map[string]string
}

func (f *fakeStore) Get(ref string) (string, error) {
	value, ok := f.items[ref]
	if !ok {
		return "", secretstore.ErrSecretNotFound
	}
	return value, nil
}

func (f *fakeStore) Set(ref, value string) error {
	if f.items == nil {
		f.items = map[string]string{}
	}
	f.items[ref] = value
	return nil
}

func (f *fakeStore) Delete(ref string) error {
	delete(f.items, ref)
	return nil
}

func TestNormalizeAPIURL(t *testing.T) {
	got := NormalizeAPIURL("https://api.certyn.io")
	if got != "https://api.certyn.io/api" {
		t.Fatalf("NormalizeAPIURL did not append /api: %q", got)
	}

	already := NormalizeAPIURL("https://api.certyn.io/api")
	if already != "https://api.certyn.io/api" {
		t.Fatalf("NormalizeAPIURL changed existing /api path: %q", already)
	}
}

func TestResolvePrecedence(t *testing.T) {
	t.Setenv("CERTYN_PROFILE", "")
	t.Setenv("CERTYN_API_URL", "https://env.certyn.io")
	t.Setenv("CERTYN_PROJECT", "env-project")
	t.Setenv("CERTYN_ENVIRONMENT", "env-environment")
	t.Setenv("CERTYN_API_KEY", "env-api-key")

	manager := &Manager{
		Data: &File{
			ActiveProfile: "dev",
			Profiles: map[string]Profile{
				"dev": {
					APIURL:      "https://profile.certyn.io",
					Project:     "profile-project",
					Environment: "profile-environment",
					APIKeyRef:   "profile-key",
				},
			},
		},
		Store: &fakeStore{items: map[string]string{"profile-key": "profile-api-key"}},
	}

	resolved, err := manager.Resolve(ResolveInput{
		APIURL:      "https://flag.certyn.io",
		Project:     "flag-project",
		Environment: "flag-environment",
		APIKey:      "flag-api-key",
	})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}

	if resolved.APIURL != "https://flag.certyn.io/api" {
		t.Fatalf("expected flag API URL precedence, got %q", resolved.APIURL)
	}
	if resolved.Project != "flag-project" {
		t.Fatalf("expected flag project precedence, got %q", resolved.Project)
	}
	if resolved.Environment != "flag-environment" {
		t.Fatalf("expected flag environment precedence, got %q", resolved.Environment)
	}
	if resolved.APIKey != "flag-api-key" {
		t.Fatalf("expected flag API key precedence, got %q", resolved.APIKey)
	}
}

func TestResolveUsesSecretStoreWhenEnvAndFlagMissing(t *testing.T) {
	t.Setenv("CERTYN_API_URL", "")
	t.Setenv("CERTYN_PROJECT", "")
	t.Setenv("CERTYN_ENVIRONMENT", "")
	t.Setenv("CERTYN_API_KEY", "")

	manager := &Manager{
		Data: &File{
			ActiveProfile: "dev",
			Profiles: map[string]Profile{
				"dev": {
					APIURL:      "https://profile.certyn.io",
					APIKeyRef:   "profile-key",
					Project:     "profile-project",
					Environment: "profile-environment",
				},
			},
		},
		Store: &fakeStore{items: map[string]string{"profile-key": "secret-api-key"}},
	}

	resolved, err := manager.Resolve(ResolveInput{})
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}

	if resolved.APIKey != "secret-api-key" {
		t.Fatalf("expected secret API key from store, got %q", resolved.APIKey)
	}
}

func TestResolveHandlesMissingSecret(t *testing.T) {
	t.Setenv("CERTYN_API_KEY", "")
	manager := &Manager{
		Data: &File{
			ActiveProfile: "dev",
			Profiles: map[string]Profile{
				"dev": {APIKeyRef: "missing"},
			},
		},
		Store: &fakeStore{items: map[string]string{}},
	}

	resolved, err := manager.Resolve(ResolveInput{})
	if err != nil {
		t.Fatalf("Resolve should not fail when secret is missing: %v", err)
	}
	if resolved.APIKey != "" {
		t.Fatalf("expected empty API key when secret missing, got %q", resolved.APIKey)
	}

	_, err = manager.Store.Get("missing")
	if !errors.Is(err, secretstore.ErrSecretNotFound) {
		t.Fatalf("expected ErrSecretNotFound, got %v", err)
	}
}

func TestProjectMappingsRoundTrip(t *testing.T) {
	manager := &Manager{
		Path: filepath.Join(t.TempDir(), "config.yaml"),
		Data: &File{
			ActiveProfile: "dev",
			Profiles: map[string]Profile{
				"dev": {},
			},
		},
		Store: &fakeStore{items: map[string]string{}},
	}

	manager.SetProjectMapping("dev", "my-project", "project-1")
	manager.SetProjectMapping("dev", "other-project", "project-2")

	if err := manager.Save(); err != nil {
		t.Fatalf("save config: %v", err)
	}

	raw, err := os.ReadFile(manager.Path)
	if err != nil {
		t.Fatalf("read config file: %v", err)
	}
	reloadedFile := File{}
	if err := yaml.Unmarshal(raw, &reloadedFile); err != nil {
		t.Fatalf("parse config yaml: %v", err)
	}
	reloaded := &Manager{
		Path:  manager.Path,
		Data:  &reloadedFile,
		Store: &fakeStore{items: map[string]string{}},
	}

	id, ok := reloaded.GetProjectMapping("dev", "MY-PROJECT")
	if !ok {
		t.Fatal("expected mapping for my-project")
	}
	if id != "project-1" {
		t.Fatalf("expected project-1, got %q", id)
	}

	mappings := reloaded.ListProjectMappings("dev")
	if len(mappings) != 2 {
		t.Fatalf("expected 2 mappings, got %#v", mappings)
	}

	reloaded.DeleteProjectMapping("dev", "other-project")
	id, ok = reloaded.GetProjectMapping("dev", "other-project")
	if ok || id != "" {
		t.Fatalf("expected deleted mapping, got id=%q ok=%t", id, ok)
	}
}

func TestInferAuthAudienceStripsApiSuffix(t *testing.T) {
	got := InferAuthAudience("https://dev.api.certyn.io/api")
	if got != "https://dev.api.certyn.io" {
		t.Fatalf("expected https://dev.api.certyn.io, got %q", got)
	}
}
