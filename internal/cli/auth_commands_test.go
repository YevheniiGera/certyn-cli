package cli

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestWhoAmIWithoutAuthReturnsLoginGuidance(t *testing.T) {
	configDir := t.TempDir()
	baseEnv := map[string]string{
		"XDG_CONFIG_HOME": configDir,
		"CI":              "",
	}

	_, _, err := executeRootCommand(t, []string{"whoami"}, baseEnv)
	if err == nil {
		t.Fatal("expected auth error")
	}

	cmdErr, ok := err.(*CommandError)
	if !ok {
		t.Fatalf("expected CommandError, got %T", err)
	}
	if cmdErr.Code != ExitAuth {
		t.Fatalf("expected auth exit code %d, got %d", ExitAuth, cmdErr.Code)
	}
	if !strings.Contains(cmdErr.Err.Error(), "certyn login") {
		t.Fatalf("expected login guidance, got %v", cmdErr.Err)
	}
}

func TestUpdateJsonOutputsInstallCommand(t *testing.T) {
	out, _, err := executeRootCommand(t, []string{"--json", "update", "--version", "v1.2.3"}, map[string]string{})
	if err != nil {
		t.Fatalf("update failed: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	if payload["target_version"] != "v1.2.3" {
		t.Fatalf("expected target_version=v1.2.3, got %#v", payload["target_version"])
	}
	if strings.TrimSpace(payload["install_command"].(string)) == "" {
		t.Fatal("expected install command in payload")
	}
}

func TestLoginPersistsExplicitAPIURLToProfile(t *testing.T) {
	authServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/oauth/device/code":
			_, _ = io.WriteString(w, `{"device_code":"device-1","user_code":"code-1","verification_uri":"https://auth.example.com/activate","verification_uri_complete":"https://auth.example.com/activate?user_code=code-1","expires_in":30,"interval":1}`)
		case "/oauth/token":
			_, _ = io.WriteString(w, `{"access_token":"header.eyJzdWIiOiJhdXRoMHwxMjMiLCJlbWFpbCI6ImRldkBjZXJ0eW4uaW8iLCJuYW1lIjoiRGV2IFVzZXIiLCJleHAiOjQxMDI0NDQ4MDB9.sig","refresh_token":"refresh-1","token_type":"Bearer","expires_in":3600}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer authServer.Close()

	configDir := t.TempDir()
	baseEnv := map[string]string{
		"XDG_CONFIG_HOME":        configDir,
		"CERTYN_AUTH_ISSUER":     authServer.URL,
		"CERTYN_AUTH_CLIENT_ID":  "test-client-id",
	}

	_, _, err := executeRootCommand(
		t,
		[]string{"--profile", "dev", "--api-url", "https://dev.api.certyn.io", "login", "--no-browser"},
		baseEnv,
	)
	if err != nil {
		t.Fatalf("login failed: %v", err)
	}

	out, _, err := executeRootCommand(
		t,
		[]string{"--json", "--profile", "dev", "config", "show"},
		baseEnv,
	)
	if err != nil {
		t.Fatalf("config show failed: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("parse config show JSON: %v", err)
	}
	if payload["api_url"] != "https://dev.api.certyn.io/api" {
		t.Fatalf("expected persisted API URL, got %#v", payload["api_url"])
	}
}
