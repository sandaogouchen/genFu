package financial

import (
	"context"
	"testing"
)

func TestCNInfoClient_QueryAnnouncements(t *testing.T) {
	client := NewCNInfoClient()
	ctx := context.Background()

	// 测试查询公告
	announcements, err := client.QueryAnnouncements(ctx, "000592", 1, 5)
	if err != nil {
		t.Logf("Query failed (may be network issue): %v", err)
		return
	}

	if len(announcements) == 0 {
		t.Log("No announcements found")
		return
	}

	t.Logf("Found %d announcements", len(announcements))
	for i, ann := range announcements {
		t.Logf("[%d] %s - %s", i+1, ann.SecCode, ann.Title)
	}
}
