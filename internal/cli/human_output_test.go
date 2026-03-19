package cli

import (
	"strings"
	"testing"

	"github.com/certyn/certyn-cli/internal/api"
)

func TestPrintAskHumanOutputIncludesContextBlock(t *testing.T) {
	stdout := captureStdout(t, func() {
		printAskHumanOutput(askOutput{
			Content:        "Check the checkout execution.",
			Warnings:       []string{"using default project 'my-project'"},
			ProjectID:      "proj_123",
			ConversationID: "conv_123",
			MessageID:      "msg_123",
			ToolCalls: []api.ToolCallResult{{
				ToolName: "search_test_cases",
				Success:  true,
			}},
		})
	})

	if !strings.Contains(stdout, "[WARN] using default project 'my-project'") {
		t.Fatalf("expected warning header, got %q", stdout)
	}
	if !strings.Contains(stdout, "[INFO] Advisor answer") {
		t.Fatalf("expected advisor answer header, got %q", stdout)
	}
	if !strings.Contains(stdout, "Check the checkout execution.") {
		t.Fatalf("expected answer content, got %q", stdout)
	}
	if !strings.Contains(stdout, "[INFO] Context") {
		t.Fatalf("expected context header, got %q", stdout)
	}
	if !strings.Contains(stdout, "project        proj_123") {
		t.Fatalf("expected project metadata, got %q", stdout)
	}
	if !strings.Contains(stdout, "tool calls     1") {
		t.Fatalf("expected tool call count, got %q", stdout)
	}
}

func TestPrintVerifyHumanSummaryShowsModernRunSummary(t *testing.T) {
	stdout := captureStdout(t, func() {
		printVerifyHumanSummary(verifyOutput{
			Mode:            "suite",
			Suite:           "smoke",
			ProcessSlug:     "smoke-suite",
			EnvironmentMode: "existing",
			EnvironmentKey:  "staging",
			RunID:           "run_123",
			State:           "completed",
			Conclusion:      "passed",
			Passed:          5,
			Failed:          0,
			Blocked:         0,
			Pending:         0,
			Aborted:         0,
			ExecutionTotal:  5,
			ExecutionPassed: 5,
			StatusURL:       "https://certyn.example/runs/run_123",
		}, false)
	})

	if !strings.Contains(stdout, "[OK] Run passed") {
		t.Fatalf("expected run summary header, got %q", stdout)
	}
	if !strings.Contains(stdout, "process        smoke-suite") {
		t.Fatalf("expected process field, got %q", stdout)
	}
	if !strings.Contains(stdout, "run id         run_123") {
		t.Fatalf("expected run id field, got %q", stdout)
	}
	if !strings.Contains(stdout, "totals         5 passed, 0 failed, 0 blocked, 0 pending, 0 aborted") {
		t.Fatalf("expected totals summary, got %q", stdout)
	}
	if !strings.Contains(stdout, "executions     5 total, 5 passed, 0 failed, 0 blocked, 0 running, 0 pending") {
		t.Fatalf("expected execution summary, got %q", stdout)
	}
	if !strings.Contains(stdout, "diagnostics    not needed") {
		t.Fatalf("expected diagnostics field, got %q", stdout)
	}
}
