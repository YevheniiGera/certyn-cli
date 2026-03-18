package cli

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"
)

func TestConfigCommandsRespectGlobalJSONFlag(t *testing.T) {
	configDir := t.TempDir()
	baseEnv := map[string]string{
		"XDG_CONFIG_HOME": configDir,
	}

	setOut, _, err := executeRootCommand(
		t,
		[]string{"--json", "config", "set", "--profile", "dev", "--environment", "staging"},
		baseEnv,
	)
	if err != nil {
		t.Fatalf("config set failed: %v", err)
	}
	var setPayload map[string]any
	if err := json.Unmarshal([]byte(setOut), &setPayload); err != nil {
		t.Fatalf("parse set JSON: %v\noutput: %s", err, setOut)
	}
	if setPayload["profile"] != "dev" {
		t.Fatalf("expected profile=dev, got %#v", setPayload["profile"])
	}
	if updated, ok := setPayload["updated"].(bool); !ok || !updated {
		t.Fatalf("expected updated=true, got %#v", setPayload["updated"])
	}

	useOut, _, err := executeRootCommand(t, []string{"--json", "config", "use", "dev"}, baseEnv)
	if err != nil {
		t.Fatalf("config use failed: %v", err)
	}
	var usePayload map[string]any
	if err := json.Unmarshal([]byte(useOut), &usePayload); err != nil {
		t.Fatalf("parse use JSON: %v\noutput: %s", err, useOut)
	}
	if usePayload["active_profile"] != "dev" {
		t.Fatalf("expected active_profile=dev, got %#v", usePayload["active_profile"])
	}
	if updated, ok := usePayload["updated"].(bool); !ok || !updated {
		t.Fatalf("expected updated=true, got %#v", usePayload["updated"])
	}

	listOut, _, err := executeRootCommand(t, []string{"--json", "config", "profiles", "list"}, baseEnv)
	if err != nil {
		t.Fatalf("config profiles list failed: %v", err)
	}
	var listPayload struct {
		ActiveProfile string   `json:"active_profile"`
		Profiles      []string `json:"profiles"`
	}
	if err := json.Unmarshal([]byte(listOut), &listPayload); err != nil {
		t.Fatalf("parse profiles JSON: %v\noutput: %s", err, listOut)
	}
	if listPayload.ActiveProfile != "dev" {
		t.Fatalf("expected active profile dev, got %q", listPayload.ActiveProfile)
	}
	if !slices.Contains(listPayload.Profiles, "default") || !slices.Contains(listPayload.Profiles, "dev") {
		t.Fatalf("expected profiles to include default and dev, got %#v", listPayload.Profiles)
	}
}

func TestConfigProjectsCommandsRespectGlobalJSONFlag(t *testing.T) {
	configDir := t.TempDir()
	baseEnv := map[string]string{
		"XDG_CONFIG_HOME": configDir,
	}

	mapOut, _, err := executeRootCommand(t, []string{
		"--json", "config", "projects", "map",
		"--slug", "my-project",
		"--id", "project-1",
	}, baseEnv)
	if err != nil {
		t.Fatalf("config projects map failed: %v", err)
	}
	var mapPayload map[string]any
	if err := json.Unmarshal([]byte(mapOut), &mapPayload); err != nil {
		t.Fatalf("parse map JSON: %v\noutput: %s", err, mapOut)
	}
	if mapPayload["slug"] != "my-project" || mapPayload["id"] != "project-1" {
		t.Fatalf("unexpected map payload: %#v", mapPayload)
	}
	if mapped, ok := mapPayload["mapped"].(bool); !ok || !mapped {
		t.Fatalf("expected mapped=true, got %#v", mapPayload["mapped"])
	}

	getOut, _, err := executeRootCommand(t, []string{
		"--json", "config", "projects", "get",
		"--slug", "my-project",
	}, baseEnv)
	if err != nil {
		t.Fatalf("config projects get failed: %v", err)
	}
	var getPayload map[string]any
	if err := json.Unmarshal([]byte(getOut), &getPayload); err != nil {
		t.Fatalf("parse get JSON: %v\noutput: %s", err, getOut)
	}
	if getPayload["id"] != "project-1" {
		t.Fatalf("expected id=project-1, got %#v", getPayload["id"])
	}

	listOut, _, err := executeRootCommand(t, []string{
		"--json", "config", "projects", "list",
	}, baseEnv)
	if err != nil {
		t.Fatalf("config projects list failed: %v", err)
	}
	var listPayload struct {
		Mappings map[string]string `json:"mappings"`
	}
	if err := json.Unmarshal([]byte(listOut), &listPayload); err != nil {
		t.Fatalf("parse list JSON: %v\noutput: %s", err, listOut)
	}
	if listPayload.Mappings["my-project"] != "project-1" {
		t.Fatalf("expected mapping my-project->project-1, got %#v", listPayload.Mappings)
	}

	unmapOut, _, err := executeRootCommand(t, []string{
		"--json", "config", "projects", "unmap",
		"--slug", "my-project",
	}, baseEnv)
	if err != nil {
		t.Fatalf("config projects unmap failed: %v", err)
	}
	var unmapPayload map[string]any
	if err := json.Unmarshal([]byte(unmapOut), &unmapPayload); err != nil {
		t.Fatalf("parse unmap JSON: %v\noutput: %s", err, unmapOut)
	}
	if removed, ok := unmapPayload["removed"].(bool); !ok || !removed {
		t.Fatalf("expected removed=true, got %#v", unmapPayload["removed"])
	}
}

func TestConfigSetProjectResolvesAndStoresMapping(t *testing.T) {
	const projectID = "11111111-2222-4333-8444-555555555555"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/projects/overview" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, `{"defaultProjectId":"`+projectID+`","projects":[{"id":"`+projectID+`","slug":"my-project","name":"My Project"}]}`)
	}))
	defer server.Close()

	configDir := t.TempDir()
	baseEnv := map[string]string{
		"XDG_CONFIG_HOME": configDir,
		"CERTYN_API_URL":  server.URL,
		"CERTYN_API_KEY":  "test-key",
	}

	setOut, _, err := executeRootCommand(t, []string{
		"--json", "config", "set",
		"--profile", "dev",
		"--api-url", server.URL,
		"--api-key-ref", "dev-key",
		"--api-key", "test-key",
		"--project", "my-project",
	}, baseEnv)
	if err != nil {
		t.Fatalf("config set failed: %v", err)
	}
	var setPayload map[string]any
	if err := json.Unmarshal([]byte(setOut), &setPayload); err != nil {
		t.Fatalf("parse set JSON: %v\noutput: %s", err, setOut)
	}
	if updated, ok := setPayload["updated"].(bool); !ok || !updated {
		t.Fatalf("expected updated=true, got %#v", setPayload["updated"])
	}

	getOut, _, err := executeRootCommand(t, []string{"--json", "config", "projects", "get", "--profile", "dev", "--slug", "my-project"}, baseEnv)
	if err != nil {
		t.Fatalf("config projects get failed: %v", err)
	}
	var getPayload map[string]any
	if err := json.Unmarshal([]byte(getOut), &getPayload); err != nil {
		t.Fatalf("parse get JSON: %v\noutput: %s", err, getOut)
	}
	if getPayload["id"] != projectID {
		t.Fatalf("expected mapped id %s, got %#v", projectID, getPayload["id"])
	}
}

func TestConfigSetProjectUnknownSlugLeavesMappingsUnchanged(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/projects/overview" {
			_, _ = io.WriteString(w, `{"defaultProjectId":"","projects":[{"id":"project-1","slug":"other-project","name":"Other"}]}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	configDir := t.TempDir()
	baseEnv := map[string]string{
		"XDG_CONFIG_HOME": configDir,
		"CERTYN_API_URL":  server.URL,
		"CERTYN_API_KEY":  "test-key",
	}

	_, _, err := executeRootCommand(t, []string{
		"config", "set",
		"--profile", "dev",
		"--project", "my-project",
	}, baseEnv)
	if err == nil {
		t.Fatal("expected unknown slug error")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitNotFound {
		t.Fatalf("expected not found exit code %d, got %d", ExitNotFound, cmdErr.Code)
	}

	listOut, _, err := executeRootCommand(t, []string{
		"--json", "config", "projects", "list", "--profile", "dev",
	}, baseEnv)
	if err != nil {
		t.Fatalf("config projects list failed: %v", err)
	}
	var listPayload struct {
		Mappings map[string]string `json:"mappings"`
	}
	if err := json.Unmarshal([]byte(listOut), &listPayload); err != nil {
		t.Fatalf("parse list JSON: %v\noutput: %s", err, listOut)
	}
	if len(listPayload.Mappings) != 0 {
		t.Fatalf("expected empty mappings, got %#v", listPayload.Mappings)
	}
}

func TestConfigSetProjectAuthFailureLeavesMappingsUnchanged(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/projects/overview" {
			w.WriteHeader(http.StatusForbidden)
			_, _ = io.WriteString(w, `{"error":"forbidden"}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	configDir := t.TempDir()
	baseEnv := map[string]string{
		"XDG_CONFIG_HOME": configDir,
	}

	_, _, err := executeRootCommand(t, []string{
		"config", "set",
		"--profile", "dev",
		"--api-url", server.URL,
		"--api-key", "test-key",
		"--project", "my-project",
	}, baseEnv)
	if err == nil {
		t.Fatal("expected auth error")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitAuth {
		t.Fatalf("expected auth exit code %d, got %d", ExitAuth, cmdErr.Code)
	}

	listOut, _, err := executeRootCommand(t, []string{
		"--json", "config", "projects", "list", "--profile", "dev",
	}, baseEnv)
	if err != nil {
		t.Fatalf("config projects list failed: %v", err)
	}
	var listPayload struct {
		Mappings map[string]string `json:"mappings"`
	}
	if err := json.Unmarshal([]byte(listOut), &listPayload); err != nil {
		t.Fatalf("parse list JSON: %v\noutput: %s", err, listOut)
	}
	if len(listPayload.Mappings) != 0 {
		t.Fatalf("expected empty mappings, got %#v", listPayload.Mappings)
	}
}

func TestConfigInitRemovedShowsMigrationGuidance(t *testing.T) {
	_, _, err := executeRootCommand(t, []string{"config", "init"}, map[string]string{
		"XDG_CONFIG_HOME": t.TempDir(),
	})
	if err == nil {
		t.Fatal("expected config init to fail")
	}

	var cmdErr *CommandError
	if !errors.As(err, &cmdErr) {
		t.Fatalf("expected CommandError, got %T (%v)", err, err)
	}
	if cmdErr.Code != ExitUsage {
		t.Fatalf("expected usage exit code %d, got %d", ExitUsage, cmdErr.Code)
	}
	if got := cmdErr.Message; got != "certyn config init was removed; use `certyn init`" {
		t.Fatalf("unexpected migration guidance: %q", got)
	}
}
