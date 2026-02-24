package api

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"genFu/internal/testutil"
)

func TestOcrHoldingsFromImage(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	imagePath := "/Users/bytedance/Downloads/test.jpg"
	data, err := os.ReadFile(imagePath)
	if err != nil {
		t.Fatalf("read image: %v", err)
	}
	if len(data) == 0 {
		t.Fatalf("empty image data")
	}
	prompt, err := readPromptFile("internal/agent/prompt/ocr_holdings.md")
	if err != nil {
		t.Fatalf("read prompt: %v", err)
	}
	mimeType := http.DetectContentType(data)
	encoded := base64.StdEncoding.EncodeToString(data)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	holdings, raw, err := ocrHoldingsFromImage(ctx, cfg.LLM, prompt, encoded, mimeType)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
			t.Skipf("ocr timeout, err=%v raw=%s", err, raw)
		}
		if strings.Contains(err.Error(), "tesseract_not_found") || strings.Contains(err.Error(), "invalid_ocr_json") {
			t.Skipf("ocr unavailable, err=%v raw=%s", err, raw)
		}
		t.Fatalf("call ocr llm: %v raw=%s", err, raw)
	}
	if len(holdings) == 0 {
		t.Fatalf("empty holdings raw=%s", raw)
	}
}

func TestEnsureDataURI(t *testing.T) {
	base64PNG := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR4nGNgYAAAAAMAASsJTYQAAAAASUVORK5CYII="
	dataURI := "data:image/png;base64," + base64PNG
	encoded, mimeType, size, err := ensureDataURI(dataURI)
	if err != nil {
		t.Fatalf("ensure data uri: %v", err)
	}
	if mimeType != "image/png" {
		t.Fatalf("unexpected mime: %s", mimeType)
	}
	if size == 0 {
		t.Fatalf("empty size")
	}
	if encoded != base64PNG {
		t.Fatalf("unexpected base64")
	}

	wrongPrefix := "data:image/jpeg;base64," + base64PNG
	_, mimeType, _, err = ensureDataURI(wrongPrefix)
	if err != nil {
		t.Fatalf("ensure data uri with wrong prefix: %v", err)
	}
	if mimeType != "image/png" {
		t.Fatalf("mime not corrected: %s", mimeType)
	}
}

func TestParseLLMHoldings(t *testing.T) {
	raw := "```json\n{\"holdings\":[{\"fund_name\":\"测试基金\",\"amount\":100.5,\"profit\":-1.2,\"profit_rate\":\"-1.19%\"}]}\n```"
	holdings, err := parseLLMHoldings(raw)
	if err != nil {
		t.Fatalf("parse holdings: %v", err)
	}
	if len(holdings) != 1 {
		t.Fatalf("unexpected holdings count")
	}
	if holdings[0].FundName != "测试基金" {
		t.Fatalf("unexpected fund name")
	}

	invalidRaw := "{\"holdings\":[{\"fund_name\":\"\",\"name\":\"\",\"symbol\":\"\"}]}"
	_, err = parseLLMHoldings(invalidRaw)
	if err == nil {
		t.Fatalf("expected parse error")
	}
}
