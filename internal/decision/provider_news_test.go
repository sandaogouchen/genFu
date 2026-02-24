package decision

import (
	"context"
	"testing"
)

func TestBriefNewsProviderBuildKeywords(t *testing.T) {
	provider := &BriefNewsProvider{
		keywords: []string{"  A股 ", "A股", " ", "科技"},
	}
	output := provider.buildKeywords(context.Background())
	if len(output) != 2 {
		t.Fatalf("unexpected keywords length: %d", len(output))
	}
	if output[0] != "A股" || output[1] != "科技" {
		t.Fatalf("unexpected keywords: %#v", output)
	}
}
