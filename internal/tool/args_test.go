package tool

import (
	"testing"

	"genFu/internal/testutil"
)

func TestRequireStringSliceArg(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	if cfg.LLM.Endpoint == "" {
		t.Fatalf("missing config")
	}
	args := map[string]interface{}{
		"keywords": []interface{}{"a", "b"},
	}
	got, err := requireStringSliceArg(args, "keywords")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("unexpected result: %v", got)
	}
}

func TestRequireStringSliceArgMissing(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	if cfg.PG.DSN == "" {
		t.Fatalf("missing config")
	}
	_, err := requireStringSliceArg(map[string]interface{}{}, "keywords")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestOptionalIntArg(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	if cfg.Server.Port == 0 {
		t.Fatalf("missing config")
	}
	args := map[string]interface{}{
		"limit": 5,
	}
	v, err := optionalIntArg(args, "limit")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v != 5 {
		t.Fatalf("unexpected value: %d", v)
	}
}
