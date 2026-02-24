//go:build integration
// +build integration

package analyze

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

const defaultFupanReportURL = "https://stock.10jqka.com.cn/fupan/20260224.shtml"

type FupanCleaningLogPayload struct {
	URL      string       `json:"url"`
	ParsedAt time.Time    `json:"parsed_at"`
	Report   *FupanReport `json:"report"`
}

// TestFupanCleaningFromURL 指定同花顺复盘链接，执行清洗并将结构化结果输出到日志。
func TestFupanCleaningFromURL(t *testing.T) {
	url := defaultFupanReportURL
	if fromEnv := os.Getenv("FUPAN_REPORT_URL"); fromEnv != "" {
		url = fromEnv
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	report, err := fetchAndParseFupanURL(ctx, url, "")
	if err != nil {
		t.Fatalf("fetch_and_parse_failed url=%s err=%v", url, err)
	}

	payload := FupanCleaningLogPayload{
		URL:      url,
		ParsedAt: time.Now(),
		Report:   report,
	}
	raw, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		t.Fatalf("marshal_payload_failed err=%v", err)
	}

	t.Logf("fupan_cleaning_payload=%s", string(raw))
}
