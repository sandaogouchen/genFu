package workflow

import "testing"

func TestParseNewsSummary(t *testing.T) {
	content := "```json\n{\"summary\":\"ok\",\"sentiment\":\"中性\"}\n```"
	summary, sentiment, ok := parseNewsSummary(content)
	if !ok {
		t.Fatalf("expected ok")
	}
	if summary != "ok" || sentiment != "中性" {
		t.Fatalf("unexpected summary")
	}
}
