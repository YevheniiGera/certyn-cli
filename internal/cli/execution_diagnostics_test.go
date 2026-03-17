package cli

import "testing"

func TestClassifyExecutionFailureReasonPrecedence(t *testing.T) {
	tests := []struct {
		name               string
		status             string
		outcome            string
		conversationStatus string
		counts             executionDiagnosticCounts
		want               string
	}{
		{
			name:               "blocked beats conversation and tool failures",
			status:             "blocked",
			conversationStatus: "failed",
			counts: executionDiagnosticCounts{
				ToolErrors: 1,
				Network5xx: 2,
			},
			want: "execution_blocked",
		},
		{
			name:               "aborted beats conversation failures",
			outcome:            "aborted",
			conversationStatus: "failed",
			counts: executionDiagnosticCounts{
				ToolErrors: 1,
			},
			want: "execution_aborted",
		},
		{
			name:               "conversation failed beats tool errors",
			status:             "failed",
			conversationStatus: "failed",
			counts: executionDiagnosticCounts{
				ToolErrors: 2,
			},
			want: "conversation_failed",
		},
		{
			name:   "tool errors beat network failures",
			status: "failed",
			counts: executionDiagnosticCounts{
				ToolErrors: 1,
				Network5xx: 3,
				Network4xx: 2,
			},
			want: "tool_error",
		},
		{
			name: "network 5xx beats network 4xx",
			counts: executionDiagnosticCounts{
				Network5xx: 2,
				Network4xx: 1,
			},
			want: "network_5xx",
		},
		{
			name:   "network 4xx beats generic failure",
			status: "failed",
			counts: executionDiagnosticCounts{
				Network4xx: 1,
			},
			want: "network_4xx",
		},
		{
			name:   "generic failed fallback",
			status: "failed",
			want:   "execution_failed",
		},
		{
			name: "unknown fallback",
			want: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := classifyExecutionFailureReason(tt.status, tt.outcome, tt.conversationStatus, tt.counts)
			if got != tt.want {
				t.Fatalf("expected reason=%s, got %s", tt.want, got)
			}
		})
	}
}
