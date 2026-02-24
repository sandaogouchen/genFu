package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"genFu/internal/config"
	"genFu/internal/investment"
)

type OcrHoldingsHandler struct {
	llmConfig     config.NormalizedLLMConfig
	investmentSvc *investment.Service
	promptPath    string
}

func NewOcrHoldingsHandler(llmCfg config.NormalizedLLMConfig, svc *investment.Service, promptPath string) *OcrHoldingsHandler {
	return &OcrHoldingsHandler{llmConfig: llmCfg, investmentSvc: svc, promptPath: promptPath}
}

const (
	maxImageBytes         = 5 * 1024 * 1024
	errorInvalidImage     = "INVALID_IMAGE"
	errorImageTooLarge    = "IMAGE_TOO_LARGE"
	errorScanTimeout      = "SCAN_TIMEOUT"
	errorScanFailed       = "SCAN_FAILED"
	errorParseFailed      = "PARSE_FAILED"
	errorNoHoldings       = "NO_HOLDINGS"
	errorResolverNotReady = "RESOLVER_NOT_READY"
)

var (
	errInvalidImage   = errors.New("invalid_image")
	errImageTooLarge  = errors.New("image_too_large")
	errInvalidOCRJSON = errors.New("invalid_ocr_json")
)

type ocrHolding struct {
	Symbol      string  `json:"symbol"`
	Name        string  `json:"name"`
	FundCode    string  `json:"fund_code"`
	FundName    string  `json:"fund_name"`
	AssetType   string  `json:"asset_type"`
	Quantity    float64 `json:"quantity"`
	AvgCost     float64 `json:"avg_cost"`
	MarketPrice float64 `json:"market_price"`
	Amount      float64 `json:"amount"`
	Profit      float64 `json:"profit"`
	ProfitRate  string  `json:"profit_rate"`
	AmountUnit  string  `json:"amount_unit"`
}

type scanRequest struct {
	Image   string `json:"image"`
	AutoAdd bool   `json:"auto_add"`
}

type scanHolding struct {
	FundCode     string  `json:"fund_code"`
	FundName     string  `json:"fund_name"`
	ResolvedName string  `json:"resolved_name"`
	Amount       float64 `json:"amount"`
	Profit       float64 `json:"profit"`
	ProfitRate   string  `json:"profit_rate"`
	MatchScore   float64 `json:"match_score"`
}

type scanResponse struct {
	Holdings    []scanHolding `json:"holdings"`
	TotalAmount float64       `json:"total_amount"`
	TotalProfit float64       `json:"total_profit"`
	ScanTimeMs  int64         `json:"scan_time_ms"`
	Error       string        `json:"error,omitempty"`
	ErrorCode   string        `json:"error_code,omitempty"`
}

func (h *OcrHoldingsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method_not_allowed", http.StatusMethodNotAllowed)
		return
	}
	start := time.Now()
	prompt, err := readPromptFile(h.promptPath)
	if err != nil {
		log.Printf("ocr holdings: read prompt err=%v", err)
		writeError(w, errorScanFailed, http.StatusInternalServerError, err)
		return
	}
	imageData, mimeType, size, autoAdd, errCode, status, err := parseScanRequest(r)
	if err != nil {
		log.Printf("ocr holdings: parse request err=%v", err)
		writeError(w, errCode, status, err)
		return
	}
	log.Printf("ocr holdings: image size=%d mime=%s prompt_len=%d auto_add=%v", size, mimeType, len(prompt), autoAdd)

	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	holdings, raw, err := ocrHoldingsFromImage(ctx, h.llmConfig, prompt, imageData, mimeType)
	if err != nil {
		log.Printf("ocr holdings: llm err=%v raw_len=%d", err, len(raw))
		code, status := mapLLMError(err)
		writeError(w, code, status, err)
		return
	}
	log.Printf("ocr holdings: parsed=%d raw_len=%d", len(holdings), len(raw))

	responseHoldings, totalAmount, totalProfit := buildScanHoldings(holdings)
	scanMs := time.Since(start).Milliseconds()

	if len(responseHoldings) == 0 {
		writeJSON(w, http.StatusOK, scanResponse{
			Holdings:    []scanHolding{},
			TotalAmount: 0,
			TotalProfit: 0,
			ScanTimeMs:  scanMs,
			ErrorCode:   errorNoHoldings,
		})
		return
	}

	if autoAdd {
		if h.investmentSvc == nil {
			writeError(w, errorResolverNotReady, http.StatusServiceUnavailable, errors.New("service_not_initialized"))
			return
		}
		accountID, err := h.investmentSvc.DefaultAccountID(r.Context())
		if err != nil {
			writeError(w, errorScanFailed, http.StatusInternalServerError, err)
			return
		}
		positions, err := h.investmentSvc.ListPositions(r.Context(), accountID)
		if err != nil {
			log.Printf("ocr holdings: list positions err=%v", err)
			writeError(w, errorScanFailed, http.StatusInternalServerError, err)
			return
		}
		for _, p := range positions {
			_ = h.investmentSvc.DeletePosition(r.Context(), accountID, p.Instrument.ID)
		}
		for _, item := range holdings {
			if strings.TrimSpace(item.Symbol) == "" {
				continue
			}
			assetType := strings.TrimSpace(item.AssetType)
			if assetType == "" {
				assetType = "fund"
			}
			instrument, err := h.investmentSvc.UpsertInstrument(r.Context(), strings.TrimSpace(item.Symbol), strings.TrimSpace(item.Name), assetType)
			if err != nil {
				log.Printf("ocr holdings: upsert instrument err=%v symbol=%s", err, item.Symbol)
				continue
			}
			// 注意：持仓的 AvgCost 和 MarketPrice 是单价，通常不需要单位转换
			// 但如果 OCR 识别的是总金额，则需要除以数量得到单价
			var pricePtr *float64
			if item.MarketPrice > 0 {
				price := item.MarketPrice
				pricePtr = &price
			}
			_, err = h.investmentSvc.SetPosition(r.Context(), accountID, instrument.ID, item.Quantity, item.AvgCost, pricePtr)
			if err != nil {
				log.Printf("ocr holdings: set position err=%v symbol=%s", err, item.Symbol)
				continue
			}
		}
	}

	writeJSON(w, http.StatusOK, scanResponse{
		Holdings:    responseHoldings,
		TotalAmount: totalAmount,
		TotalProfit: totalProfit,
		ScanTimeMs:  scanMs,
	})
}

func readPromptFile(path string) (string, error) {
	if path == "" {
		return "", errors.New("missing_prompt_path")
	}
	abs := findFilePath(path)
	data, err := os.ReadFile(abs)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(data)), nil
}

func findFilePath(rel string) string {
	if filepath.IsAbs(rel) {
		return rel
	}
	cwd, err := os.Getwd()
	if err == nil {
		p := filepath.Join(cwd, rel)
		if exists(p) {
			return p
		}
	}
	root := cwd
	for i := 0; i < 6; i++ {
		parent := filepath.Dir(root)
		if parent == root || strings.TrimSpace(parent) == "" {
			break
		}
		root = parent
		p := filepath.Join(root, rel)
		if exists(p) {
			return p
		}
	}
	return rel
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func readImage(file multipart.File, header *multipart.FileHeader) (string, string, int, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return "", "", 0, err
	}
	if len(data) == 0 {
		return "", "", 0, errInvalidImage
	}
	if len(data) > maxImageBytes {
		return "", "", len(data), errImageTooLarge
	}
	headerMime := header.Header.Get("Content-Type")
	mimeType := detectMimeType(data, headerMime)
	encoded := base64.StdEncoding.EncodeToString(data)
	return encoded, mimeType, len(data), nil
}

type oaiImageURL struct {
	URL string `json:"url"`
}

type oaiContent struct {
	Type     string       `json:"type"`
	Text     string       `json:"text,omitempty"`
	ImageURL *oaiImageURL `json:"image_url,omitempty"`
}

type oaiMessage struct {
	Role    string      `json:"role"`
	Content interface{} `json:"content"`
}

type oaiRequest struct {
	Model          string            `json:"model"`
	Messages       []oaiMessage      `json:"messages"`
	Temperature    float64           `json:"temperature,omitempty"`
	MaxTokens      int               `json:"max_tokens,omitempty"`
	ResponseFormat map[string]string `json:"response_format,omitempty"`
}

type oaiResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
}

func callOcrLLM(ctx context.Context, cfg config.NormalizedLLMConfig, prompt string, imageBase64 string, mimeType string) ([]ocrHolding, string, error) {
	endpoint := normalizeEndpoint(cfg.Endpoint)
	if strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(endpoint) == "" || strings.TrimSpace(cfg.Model) == "" {
		return nil, "", errors.New("llm_config_missing")
	}
	dataURL := "data:" + mimeType + ";base64," + imageBase64
	reqBody := oaiRequest{
		Model: cfg.Model,
		Messages: []oaiMessage{
			{Role: "system", Content: prompt},
			{Role: "user", Content: []oaiContent{
				{Type: "text", Text: "请从截图中识别持仓并输出 JSON"},
				{Type: "image_url", ImageURL: &oaiImageURL{URL: dataURL}},
			}},
		},
		Temperature:    0.1,
		MaxTokens:      1200,
		ResponseFormat: map[string]string{"type": "json_object"},
	}
	payload, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(payload)))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	client := &http.Client{Timeout: 90 * time.Second}
	resp, err := client.Do(req)
	log.Printf("ocr llm request endpoint=%s model=%s mime=%s image_b64_len=%d prompt_len=%d payload_len=%d", endpoint, cfg.Model, mimeType, len(imageBase64), len(prompt), len(payload))
	if err != nil {
		log.Printf("ocr llm request err=%v", err)
		return nil, "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	log.Printf("ocr llm response status=%d body_len=%d", resp.StatusCode, len(body))
	if resp.StatusCode >= 400 {
		return nil, "", errors.New(strings.TrimSpace(string(body)))
	}
	var parsed oaiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		log.Printf("ocr llm decode err=%v body=%s", err, truncateString(string(body), 2048))
		return nil, "", err
	}
	if len(parsed.Choices) == 0 {
		log.Printf("ocr llm empty choices")
		return nil, "", errors.New("llm_empty_response")
	}
	raw := parsed.Choices[0].Message.Content
	log.Printf("ocr llm raw_len=%d", len(raw))
	log.Printf("ocr llm raw=%s", truncateString(raw, 2048))
	output, err := parseLLMHoldings(raw)
	if err != nil {
		return nil, raw, err
	}
	return output, raw, nil
}

func callOcrTextLLM(ctx context.Context, cfg config.NormalizedLLMConfig, prompt string, text string) ([]ocrHolding, string, error) {
	endpoint := normalizeEndpoint(cfg.Endpoint)
	if strings.TrimSpace(cfg.APIKey) == "" || strings.TrimSpace(endpoint) == "" || strings.TrimSpace(cfg.Model) == "" {
		return nil, "", errors.New("llm_config_missing")
	}
	reqBody := oaiRequest{
		Model: cfg.Model,
		Messages: []oaiMessage{
			{Role: "system", Content: prompt},
			{Role: "user", Content: "以下为OCR文本，请提取持仓并输出JSON对象：\n" + text},
		},
		Temperature:    0.1,
		MaxTokens:      1200,
		ResponseFormat: map[string]string{"type": "json_object"},
	}
	payload, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(string(payload)))
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)
	client := &http.Client{Timeout: 90 * time.Second}
	log.Printf("ocr llm text request start endpoint=%s model=%s text_len=%d payload_len=%d", endpoint, cfg.Model, len(text), len(payload))
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("ocr llm text request err=%v", err)
		return nil, "", err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	log.Printf("ocr llm text response status=%d body_len=%d", resp.StatusCode, len(body))
	if resp.StatusCode >= 400 {
		return nil, "", errors.New(strings.TrimSpace(string(body)))
	}
	var parsed oaiResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		log.Printf("ocr llm text decode err=%v body=%s", err, truncateString(string(body), 2048))
		return nil, "", err
	}
	if len(parsed.Choices) == 0 {
		log.Printf("ocr llm text empty choices")
		return nil, "", errors.New("llm_empty_response")
	}
	raw := parsed.Choices[0].Message.Content
	log.Printf("ocr llm text raw_len=%d", len(raw))
	output, err := parseLLMHoldings(raw)
	if err != nil {
		return nil, raw, err
	}
	return output, raw, nil
}

func ocrHoldingsFromImage(ctx context.Context, cfg config.NormalizedLLMConfig, prompt string, imageBase64 string, mimeType string) ([]ocrHolding, string, error) {
	holdings, raw, err := callOcrLLM(ctx, cfg, prompt, imageBase64, mimeType)
	if err == nil {
		return holdings, raw, nil
	}
	log.Printf("ocr holdings: vision fallback err=%v", err)
	text, ocrErr := ocrTextWithTesseract(ctx, imageBase64, mimeType)
	if ocrErr != nil {
		log.Printf("ocr holdings: tesseract err=%v", ocrErr)
		return nil, raw, err
	}
	log.Printf("ocr holdings: tesseract text_len=%d", len(text))
	holdings, rawText, err2 := callOcrTextLLM(ctx, cfg, prompt, text)
	if err2 != nil {
		log.Printf("ocr holdings: text llm err=%v", err2)
		return nil, rawText, err2
	}
	return holdings, rawText, nil
}

func ocrTextWithTesseract(ctx context.Context, imageBase64 string, mimeType string) (string, error) {
	_, err := exec.LookPath("tesseract")
	if err != nil {
		log.Printf("ocr holdings: tesseract not found")
		return "", errors.New("tesseract_not_found")
	}
	data, err := base64.StdEncoding.DecodeString(imageBase64)
	if err != nil {
		return "", err
	}
	ext := ".img"
	if strings.Contains(mimeType, "png") {
		ext = ".png"
	}
	if strings.Contains(mimeType, "jpeg") || strings.Contains(mimeType, "jpg") {
		ext = ".jpg"
	}
	tmpFile, err := os.CreateTemp("", "ocr_*"+ext)
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(data); err != nil {
		return "", err
	}
	if err := tmpFile.Close(); err != nil {
		return "", err
	}
	cmd := exec.CommandContext(ctx, "tesseract", tmpFile.Name(), "stdout")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func extractJSON(raw string) string {
	text := strings.TrimSpace(raw)
	if text == "" {
		return ""
	}
	if strings.HasPrefix(text, "[") || strings.HasPrefix(text, "{") {
		return text
	}
	start := strings.Index(text, "[")
	end := strings.LastIndex(text, "]")
	if start >= 0 && end > start {
		return text[start : end+1]
	}
	start = strings.Index(text, "{")
	end = strings.LastIndex(text, "}")
	if start >= 0 && end > start {
		return text[start : end+1]
	}
	return ""
}

func parseLLMHoldings(raw string) ([]ocrHolding, error) {
	cleaned := stripMarkdownCodeFence(raw)
	jsonText := extractJSON(cleaned)
	if strings.TrimSpace(jsonText) == "" {
		return nil, errInvalidOCRJSON
	}
	var output []ocrHolding
	if err := json.Unmarshal([]byte(jsonText), &output); err != nil {
		var wrapper struct {
			Holdings []ocrHolding `json:"holdings"`
		}
		if err2 := json.Unmarshal([]byte(jsonText), &wrapper); err2 != nil {
			return nil, errInvalidOCRJSON
		}
		output = wrapper.Holdings
	}
	if err := validateHoldings(output); err != nil {
		return nil, err
	}
	return output, nil
}

func validateHoldings(items []ocrHolding) error {
	if len(items) == 0 {
		return nil
	}
	hasName := false
	for _, item := range items {
		name := strings.TrimSpace(item.FundName)
		if name == "" {
			name = strings.TrimSpace(item.Name)
		}
		if name != "" || strings.TrimSpace(item.Symbol) != "" {
			hasName = true
		}
		if !isValidNumber(item.Amount) || !isValidNumber(item.Profit) || !isValidNumber(item.Quantity) || !isValidNumber(item.AvgCost) || !isValidNumber(item.MarketPrice) {
			return errInvalidOCRJSON
		}
	}
	if !hasName {
		return errInvalidOCRJSON
	}
	return nil
}

func isValidNumber(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func stripMarkdownCodeFence(value string) string {
	text := strings.TrimSpace(value)
	if !strings.HasPrefix(text, "```") {
		return text
	}
	text = strings.TrimPrefix(text, "```")
	text = strings.TrimSpace(text)
	if strings.HasPrefix(text, "json") {
		text = strings.TrimPrefix(text, "json")
	}
	text = strings.TrimSpace(text)
	if idx := strings.LastIndex(text, "```"); idx >= 0 {
		text = text[:idx]
	}
	return strings.TrimSpace(text)
}

func detectMimeType(data []byte, headerMime string) string {
	detected := http.DetectContentType(data)
	if strings.HasPrefix(detected, "image/") {
		return detected
	}
	if strings.HasPrefix(headerMime, "image/") {
		return headerMime
	}
	return detected
}

func parseScanRequest(r *http.Request) (string, string, int, bool, string, int, error) {
	contentType := r.Header.Get("Content-Type")
	if strings.Contains(contentType, "application/json") {
		var req scanRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			return "", "", 0, false, errorInvalidImage, http.StatusBadRequest, errInvalidImage
		}
		base64Data, mimeType, size, err := ensureDataURI(req.Image)
		if err != nil {
			return "", "", 0, false, errorInvalidImage, http.StatusBadRequest, err
		}
		if size > maxImageBytes {
			return "", "", size, req.AutoAdd, errorImageTooLarge, http.StatusBadRequest, errImageTooLarge
		}
		return base64Data, mimeType, size, req.AutoAdd, "", http.StatusOK, nil
	}

	file, header, err := r.FormFile("image")
	if err != nil {
		return "", "", 0, false, errorInvalidImage, http.StatusBadRequest, errInvalidImage
	}
	defer file.Close()
	base64Data, mimeType, size, err := readImage(file, header)
	if err != nil {
		if errors.Is(err, errImageTooLarge) {
			return "", "", size, false, errorImageTooLarge, http.StatusBadRequest, err
		}
		return "", "", 0, false, errorInvalidImage, http.StatusBadRequest, err
	}
	return base64Data, mimeType, size, false, "", http.StatusOK, nil
}

func ensureDataURI(value string) (string, string, int, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", "", 0, errInvalidImage
	}
	mimeType, encoded, _ := parseDataURI(trimmed)
	data, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", "", 0, errInvalidImage
	}
	if len(data) == 0 {
		return "", "", 0, errInvalidImage
	}
	detected := detectMimeType(data, mimeType)
	if !strings.HasPrefix(detected, "image/") {
		return "", "", 0, errInvalidImage
	}
	return base64.StdEncoding.EncodeToString(data), detected, len(data), nil
}

func parseDataURI(value string) (string, string, bool) {
	if !strings.HasPrefix(value, "data:") {
		return "", value, false
	}
	idx := strings.Index(value, ",")
	if idx <= 0 {
		return "", "", true
	}
	header := value[:idx]
	data := value[idx+1:]
	header = strings.TrimPrefix(header, "data:")
	header = strings.TrimSpace(header)
	parts := strings.Split(header, ";")
	mimeType := strings.TrimSpace(parts[0])
	return mimeType, data, true
}

func buildScanHoldings(items []ocrHolding) ([]scanHolding, float64, float64) {
	holdings := make([]scanHolding, 0, len(items))
	var totalAmount float64
	var totalProfit float64
	for _, item := range items {
		name := strings.TrimSpace(item.FundName)
		if name == "" {
			name = strings.TrimSpace(item.Name)
		}
		if name == "" && strings.TrimSpace(item.Symbol) != "" {
			name = strings.TrimSpace(item.Symbol)
		}
		if name == "" {
			continue
		}
		// 将金额统一转换为"元"为单位
		amount := convertAmountToYuan(item.Amount, item.AmountUnit)
		profit := convertAmountToYuan(item.Profit, item.AmountUnit)

		if amount == 0 && item.Quantity > 0 {
			price := item.MarketPrice
			if price == 0 {
				price = item.AvgCost
			}
			amount = item.Quantity * price
		}
		if profit == 0 && item.Quantity > 0 && item.MarketPrice > 0 && item.AvgCost > 0 {
			profit = (item.MarketPrice - item.AvgCost) * item.Quantity
		}
		if math.Abs(amount) < 1e-9 {
			amount = 0
		}
		if math.Abs(profit) < 1e-9 {
			profit = 0
		}
		profitRate := strings.TrimSpace(item.ProfitRate)
		if profitRate == "" && amount != 0 {
			profitRate = fmt.Sprintf("%.2f%%", (profit/amount)*100)
		}
		holdings = append(holdings, scanHolding{
			FundCode:     strings.TrimSpace(item.FundCode),
			FundName:     name,
			ResolvedName: "",
			Amount:       amount,
			Profit:       profit,
			ProfitRate:   profitRate,
			MatchScore:   0,
		})
		totalAmount += amount
		totalProfit += profit
	}
	return holdings, totalAmount, totalProfit
}

func mapLLMError(err error) (string, int) {
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return errorScanTimeout, http.StatusGatewayTimeout
	}
	if errors.Is(err, errInvalidOCRJSON) || strings.Contains(err.Error(), "invalid_ocr_json") {
		return errorParseFailed, http.StatusInternalServerError
	}
	return errorScanFailed, http.StatusInternalServerError
}

func writeJSON(w http.ResponseWriter, status int, payload scanResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, code string, status int, err error) {
	writeJSON(w, status, scanResponse{
		Error:     err.Error(),
		ErrorCode: code,
	})
}

func normalizeEndpoint(endpoint string) string {
	trimmed := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	if strings.HasSuffix(trimmed, "/chat/completions") {
		return trimmed
	}
	return trimmed + "/chat/completions"
}

func truncateString(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	if len(value) <= limit {
		return value
	}
	return value[:limit]
}

// convertAmountToYuan 将金额统一转换为"元"为单位
// unit 可能的值：元、万元、亿元、万、亿等
func convertAmountToYuan(amount float64, unit string) float64 {
	if amount == 0 {
		return 0
	}

	unitLower := strings.ToLower(strings.TrimSpace(unit))
	switch {
	case strings.Contains(unitLower, "亿"):
		return amount * 100000000
	case strings.Contains(unitLower, "万"):
		return amount * 10000
	default:
		// 默认认为是元，不需要转换
		return amount
	}
}
