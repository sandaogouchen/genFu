package tool

import (
	"context"
	"testing"

	"genFu/internal/testutil"
)

type testTool struct{}

func (t testTool) Spec() ToolSpec {
	return ToolSpec{Name: "test"}
}

func (t testTool) Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	_ = ctx
	return ToolResult{Name: "test", Output: args["v"]}, nil
}

func TestRegistryExecute(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	if cfg.LLM.Model == "" {
		t.Fatalf("missing config")
	}
	reg := NewRegistry()
	reg.Register(testTool{})
	result, err := reg.Execute(context.Background(), ToolCall{Name: "test", Arguments: map[string]interface{}{"v": "ok"}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Output.(string) != "ok" {
		t.Fatalf("unexpected output")
	}
}

func TestRegistryNotFound(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	if cfg.RSSHub.BaseURL == "" {
		t.Fatalf("missing config")
	}
	reg := NewRegistry()
	_, err := reg.Execute(context.Background(), ToolCall{Name: "missing"})
	if err == nil {
		t.Fatalf("expected error")
	}
}
