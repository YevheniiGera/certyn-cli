package config

import (
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/certyn/certyn-cli/internal/secretstore"
	"gopkg.in/yaml.v3"
)

const (
	DefaultAPIURL      = "https://api.certyn.io"
	DefaultProfileName = "default"
	DefaultAuthIssuer  = "https://auth.certyn.io"
	DefaultAuthClientID = "vOfBJycsLfy5QVQ7srQGOFfyDv8tmO7N"
)

type File struct {
	ActiveProfile string             `yaml:"active_profile"`
	Profiles      map[string]Profile `yaml:"profiles"`
}

type Profile struct {
	APIURL          string            `yaml:"api_url,omitempty"`
	Project         string            `yaml:"project,omitempty"`
	Environment     string            `yaml:"environment,omitempty"`
	APIKeyRef       string            `yaml:"api_key_ref,omitempty"`
	AuthIssuer      string            `yaml:"auth_issuer,omitempty"`
	AuthAudience    string            `yaml:"auth_audience,omitempty"`
	AuthClientID    string            `yaml:"auth_client_id,omitempty"`
	AccessTokenRef  string            `yaml:"access_token_ref,omitempty"`
	RefreshTokenRef string            `yaml:"refresh_token_ref,omitempty"`
	ProjectIDs      map[string]string `yaml:"project_ids,omitempty"`
}

type ResolveInput struct {
	Profile     string
	APIURL      string
	APIKey      string
	Project     string
	Environment string
}

type Runtime struct {
	Profile      string
	APIURL       string
	APIKey       string
	AccessToken  string
	RefreshToken string
	AuthIssuer   string
	AuthAudience string
	AuthClientID string
	Project      string
	Environment  string
}

type Manager struct {
	Path  string
	Data  *File
	Store secretstore.Store
}

func Load() (*Manager, error) {
	path, err := DefaultPath()
	if err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create config directory: %w", err)
	}

	data := &File{
		ActiveProfile: DefaultProfileName,
		Profiles: map[string]Profile{
			DefaultProfileName: {},
		},
	}

	if raw, err := os.ReadFile(path); err == nil && len(raw) > 0 {
		if err := yaml.Unmarshal(raw, data); err != nil {
			return nil, fmt.Errorf("parse config file: %w", err)
		}
	}

	if data.Profiles == nil {
		data.Profiles = map[string]Profile{DefaultProfileName: {}}
	}
	if strings.TrimSpace(data.ActiveProfile) == "" {
		data.ActiveProfile = DefaultProfileName
	}

	mgr := &Manager{
		Path:  path,
		Data:  data,
		Store: secretstore.NewHybridStore(path),
	}

	return mgr, nil
}

func (m *Manager) Save() error {
	if m == nil {
		return errors.New("config manager is nil")
	}
	if m.Data == nil {
		return errors.New("config data is nil")
	}
	if m.Data.Profiles == nil {
		m.Data.Profiles = map[string]Profile{DefaultProfileName: {}}
	}
	if strings.TrimSpace(m.Data.ActiveProfile) == "" {
		m.Data.ActiveProfile = DefaultProfileName
	}

	raw, err := yaml.Marshal(m.Data)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(m.Path, raw, 0o600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}
	return nil
}

func (m *Manager) Resolve(input ResolveInput) (Runtime, error) {
	profileName := firstNonEmpty(
		strings.TrimSpace(input.Profile),
		strings.TrimSpace(os.Getenv("CERTYN_PROFILE")),
		strings.TrimSpace(m.Data.ActiveProfile),
		DefaultProfileName,
	)

	profile := m.Data.Profiles[profileName]
	configuredAPIURL := firstNonEmpty(
		strings.TrimSpace(input.APIURL),
		strings.TrimSpace(os.Getenv("CERTYN_API_URL")),
		strings.TrimSpace(profile.APIURL),
		DefaultAPIURL,
	)

	resolved := Runtime{
		Profile: profileName,
		APIURL: firstNonEmpty(
			configuredAPIURL,
		),
		Project: firstNonEmpty(
			strings.TrimSpace(input.Project),
			strings.TrimSpace(os.Getenv("CERTYN_PROJECT")),
			strings.TrimSpace(profile.Project),
		),
		Environment: firstNonEmpty(
			strings.TrimSpace(input.Environment),
			strings.TrimSpace(os.Getenv("CERTYN_ENVIRONMENT")),
			strings.TrimSpace(profile.Environment),
		),
		AuthIssuer: firstNonEmpty(
			strings.TrimSpace(os.Getenv("CERTYN_AUTH_ISSUER")),
			strings.TrimSpace(profile.AuthIssuer),
			DefaultAuthIssuer,
		),
		AuthAudience: firstNonEmpty(
			strings.TrimSpace(os.Getenv("CERTYN_AUTH_AUDIENCE")),
			strings.TrimSpace(profile.AuthAudience),
			InferAuthAudience(configuredAPIURL),
		),
		AuthClientID: firstNonEmpty(
			strings.TrimSpace(os.Getenv("CERTYN_AUTH_CLIENT_ID")),
			strings.TrimSpace(profile.AuthClientID),
			DefaultAuthClientID,
		),
	}

	resolved.APIURL = NormalizeAPIURL(resolved.APIURL)
	resolved.APIKey = strings.TrimSpace(input.APIKey)
	if resolved.APIKey == "" {
		resolved.APIKey = strings.TrimSpace(os.Getenv("CERTYN_API_KEY"))
	}
	if resolved.APIKey == "" && profile.APIKeyRef != "" {
		secretValue, err := m.Store.Get(profile.APIKeyRef)
		if err == nil {
			resolved.APIKey = strings.TrimSpace(secretValue)
		}
	}
	if profile.AccessTokenRef != "" {
		secretValue, err := m.Store.Get(profile.AccessTokenRef)
		if err == nil {
			resolved.AccessToken = strings.TrimSpace(secretValue)
		}
	}
	if profile.RefreshTokenRef != "" {
		secretValue, err := m.Store.Get(profile.RefreshTokenRef)
		if err == nil {
			resolved.RefreshToken = strings.TrimSpace(secretValue)
		}
	}

	return resolved, nil
}

func (m *Manager) UpsertProfile(name string, p Profile) {
	if m.Data.Profiles == nil {
		m.Data.Profiles = map[string]Profile{}
	}
	m.Data.Profiles[name] = p
}

func (m *Manager) SetActiveProfile(name string) error {
	if name == "" {
		return errors.New("profile name is required")
	}
	if _, ok := m.Data.Profiles[name]; !ok {
		return fmt.Errorf("profile '%s' not found", name)
	}
	m.Data.ActiveProfile = name
	return nil
}

func (m *Manager) SetProjectMapping(profile, slug, id string) {
	if m == nil || m.Data == nil {
		return
	}

	profileName := m.profileName(profile, true)
	current := m.Data.Profiles[profileName]
	if current.ProjectIDs == nil {
		current.ProjectIDs = map[string]string{}
	}
	current.ProjectIDs[normalizeSlug(slug)] = strings.TrimSpace(id)
	m.Data.Profiles[profileName] = current
}

func (m *Manager) DeleteProjectMapping(profile, slug string) {
	if m == nil || m.Data == nil {
		return
	}

	profileName := m.profileName(profile, false)
	current, ok := m.Data.Profiles[profileName]
	if !ok || len(current.ProjectIDs) == 0 {
		return
	}
	delete(current.ProjectIDs, normalizeSlug(slug))
	if len(current.ProjectIDs) == 0 {
		current.ProjectIDs = nil
	}
	m.Data.Profiles[profileName] = current
}

func (m *Manager) GetProjectMapping(profile, slug string) (string, bool) {
	if m == nil || m.Data == nil {
		return "", false
	}

	profileName := m.profileName(profile, false)
	current, ok := m.Data.Profiles[profileName]
	if !ok || len(current.ProjectIDs) == 0 {
		return "", false
	}
	value, found := current.ProjectIDs[normalizeSlug(slug)]
	if !found {
		return "", false
	}
	return strings.TrimSpace(value), strings.TrimSpace(value) != ""
}

func (m *Manager) ListProjectMappings(profile string) map[string]string {
	result := map[string]string{}
	if m == nil || m.Data == nil {
		return result
	}

	profileName := m.profileName(profile, false)
	current, ok := m.Data.Profiles[profileName]
	if !ok || len(current.ProjectIDs) == 0 {
		return result
	}
	for slug, id := range current.ProjectIDs {
		if strings.TrimSpace(slug) == "" || strings.TrimSpace(id) == "" {
			continue
		}
		result[slug] = strings.TrimSpace(id)
	}
	return result
}

func (m *Manager) profileName(override string, createIfMissing bool) string {
	name := firstNonEmpty(strings.TrimSpace(override), strings.TrimSpace(m.Data.ActiveProfile), DefaultProfileName)
	if m.Data.Profiles == nil {
		m.Data.Profiles = map[string]Profile{}
	}
	if createIfMissing {
		if _, ok := m.Data.Profiles[name]; !ok {
			m.Data.Profiles[name] = Profile{}
		}
	}
	if strings.TrimSpace(m.Data.ActiveProfile) == "" {
		m.Data.ActiveProfile = DefaultProfileName
	}
	return name
}

func normalizeSlug(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func NormalizeAPIURL(input string) string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" {
		trimmed = DefaultAPIURL
	}
	trimmed = strings.TrimSuffix(trimmed, "/")

	u, err := url.Parse(trimmed)
	if err != nil || u.Scheme == "" || u.Host == "" {
		if strings.HasSuffix(trimmed, "/api") {
			return trimmed
		}
		return trimmed + "/api"
	}

	path := strings.TrimSuffix(u.Path, "/")
	if !strings.HasSuffix(path, "/api") {
		path += "/api"
	}
	u.Path = path
	u.RawPath = ""
	return strings.TrimSuffix(u.String(), "/")
}

func InferAuthAudience(input string) string {
	normalized := NormalizeAPIURL(input)
	u, err := url.Parse(normalized)
	if err != nil || u.Scheme == "" || u.Host == "" {
		return strings.TrimSuffix(strings.TrimSuffix(normalized, "/"), "/api")
	}

	path := strings.TrimSuffix(u.Path, "/")
	path = strings.TrimSuffix(path, "/api")
	if path == "" {
		path = "/"
	}
	u.Path = path
	u.RawPath = ""
	u.RawQuery = ""
	u.Fragment = ""
	return strings.TrimSuffix(u.String(), "/")
}

func DefaultPath() (string, error) {
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(xdg, "certyn", "config.yaml"), nil
	}

	if runtime.GOOS == "windows" {
		if appData := strings.TrimSpace(os.Getenv("AppData")); appData != "" {
			return filepath.Join(appData, "certyn", "config.yaml"), nil
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".config", "certyn", "config.yaml"), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}
