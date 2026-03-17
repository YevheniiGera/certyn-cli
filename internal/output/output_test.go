package output

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWriteGitHubOutputs(t *testing.T) {
	dir := t.TempDir()
	outputPath := filepath.Join(dir, "github_output.txt")
	t.Setenv("GITHUB_OUTPUT", outputPath)

	err := WriteGitHubOutputs(map[string]string{
		"run_id":  "run-1",
		"app_url": "https://app.certyn.io/path",
	})
	if err != nil {
		t.Fatalf("WriteGitHubOutputs returned error: %v", err)
	}

	raw, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed reading output file: %v", err)
	}
	text := string(raw)
	if !strings.Contains(text, "run_id=run-1") {
		t.Fatalf("missing run_id output: %s", text)
	}
	if !strings.Contains(text, "app_url=https://app.certyn.io/path") {
		t.Fatalf("missing app_url output: %s", text)
	}
}
