package eval

import (
	"strings"
	"testing"
)

func TestRenderMarkdownSummary(t *testing.T) {
	report := Report{
		Systems: []SystemSummary{
			{Name: "multi-agent", ScenarioCount: 2, AverageScore: 1.0, ScenarioWins: 2},
			{Name: "single-agent", ScenarioCount: 2, AverageScore: 0.875, ScenarioWins: 1},
		},
	}

	out := RenderMarkdownSummary(report)
	if !strings.Contains(out, "| System | Scenarios | Avg Score | Wins |") {
		t.Fatalf("expected markdown header, got %q", out)
	}
	if !strings.Contains(out, "multi-agent") {
		t.Fatalf("expected multi-agent row, got %q", out)
	}
}
