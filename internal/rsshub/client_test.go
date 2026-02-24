//go:build live

package rsshub

import (
	"os"
	"testing"

	"genFu/internal/config"
	"genFu/internal/testutil"
)

func TestFetchLive(t *testing.T) {
	if os.Getenv("GENFU_LIVE_TESTS") == "" {
		t.Skip("skip live test")
	}
	path, err := testutil.ConfigPath()
	if err != nil {
		t.Fatalf("config path: %v", err)
	}
	appConfig, err := config.Load(path)
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	baseURL := appConfig.RSSHub.BaseURL
	route := "/aicaijing/latest"
	if len(appConfig.RSSHub.Routes) > 0 && appConfig.RSSHub.Routes[0] != "" {
		route = appConfig.RSSHub.Routes[0]
	}
	client := NewClient(baseURL, appConfig.RSSHub.Timeout)
	items, err := client.Fetch(route, 5)
	if err != nil {
		t.Fatalf("fetch failed: %v", err)
	}
	if len(items) == 0 {
		t.Fatalf("empty items")
	}
	if items[0].Title == "" || items[0].Link == "" {
		t.Fatalf("missing title or link")
	}
}
