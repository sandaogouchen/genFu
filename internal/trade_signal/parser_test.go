package trade_signal

import (
	"testing"

	"genFu/internal/testutil"
)

func TestParseDecisionOutput(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	raw := `{"decision_id":"d1","decisions":[{"account_id":1,"symbol":"A","action":"buy","quantity":1,"price":2}]}`
	output, signals, err := ParseDecisionOutput(raw, cfg.News.AccountID)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if output.DecisionID != "d1" {
		t.Fatalf("unexpected decision id")
	}
	if len(signals) != 1 || signals[0].Symbol != "A" {
		t.Fatalf("unexpected signals")
	}
}

func TestParseDecisionOutputInvalid(t *testing.T) {
	_, _, err := ParseDecisionOutput("", 1)
	if err == nil {
		t.Fatalf("expected error")
	}
}
