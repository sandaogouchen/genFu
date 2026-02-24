package db

import (
	"database/sql"
	"testing"

	"genFu/internal/testutil"
)

func TestParseTime(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	if cfg.PG.DSN == "" {
		t.Fatalf("missing config")
	}
	tests := []sql.NullString{
		{String: "2026-02-15T00:00:00Z", Valid: true},
		{String: "2026-02-15 08:00:00", Valid: true},
	}
	for _, tt := range tests {
		parsed, ok := ParseTime(tt)
		if !ok || parsed.IsZero() {
			t.Fatalf("expected parsed time")
		}
	}
	if _, ok := ParseTime(sql.NullString{Valid: false}); ok {
		t.Fatalf("expected invalid")
	}
}
