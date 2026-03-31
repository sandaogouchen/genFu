package llm

import (
	"strings"
	"testing"
)

func TestMergeToolCallArguments_IncrementalChunks(t *testing.T) {
	current := ""
	current = mergeToolCallArguments(current, `{"action":"get_quote",`)
	current = mergeToolCallArguments(current, `"code":"000001"}`)
	got := normalizeToolCallArguments(current)
	want := `{"action":"get_quote","code":"000001"}`
	if got != want {
		t.Fatalf("unexpected merged args: got=%s want=%s", got, want)
	}
}

func TestMergeToolCallArguments_CumulativeChunks(t *testing.T) {
	current := ""
	current = mergeToolCallArguments(current, `{"action":"get_quote"`)
	current = mergeToolCallArguments(current, `{"action":"get_quote","code":"000001"}`)
	got := normalizeToolCallArguments(current)
	want := `{"action":"get_quote","code":"000001"}`
	if got != want {
		t.Fatalf("unexpected cumulative args: got=%s want=%s", got, want)
	}
}

func TestNormalizeToolCallArguments_ConcatenatedJSONValues(t *testing.T) {
	raw := `{"a":1}{"a":1,"b":2}`
	got := normalizeToolCallArguments(raw)
	if !strings.Contains(got, `"b":2`) {
		t.Fatalf("expected normalized latest json, got=%s", got)
	}
}
