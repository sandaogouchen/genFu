package llm

import "testing"

func TestRoundTwoDecimals(t *testing.T) {
	if roundTwoDecimals(0.234) != 0.23 {
		t.Fatalf("expected 0.23")
	}
	if roundTwoDecimals(0.235) != 0.24 {
		t.Fatalf("expected 0.24")
	}
	if roundTwoDecimals(1) != 1 {
		t.Fatalf("expected 1")
	}
}
