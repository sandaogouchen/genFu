package tushare

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Config holds Tushare Pro API configuration.
type Config struct {
	Token      string        `yaml:"token"`
	BaseURL    string        `yaml:"base_url"`
	Timeout    time.Duration `yaml:"timeout"`
	MaxRetries int           `yaml:"max_retries"`
	RateLimit  int           `yaml:"rate_limit"` // requests per minute
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		BaseURL:    "http://api.tushare.pro",
		Timeout:    15 * time.Second,
		MaxRetries: 3,
		RateLimit:  200,
	}
}

// ErrPermissionDenied is returned when Tushare responds with code 2002.
var ErrPermissionDenied = fmt.Errorf("tushare: permission denied (insufficient points)")

// Client is a lightweight Tushare Pro HTTP client with rate limiting and retry.
type Client struct {
	config Config
	http   *http.Client

	// Simple token-bucket rate limiter
	mu        sync.Mutex
	tokens    float64
	maxTokens float64
	refillRate float64 // tokens per second
	lastRefill time.Time
}

// NewClient creates a new Tushare client. Returns nil if token is empty.
func NewClient(cfg Config) *Client {
	if cfg.Token == "" {
		return nil
	}
	if cfg.BaseURL == "" {
		cfg.BaseURL = DefaultConfig().BaseURL
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = DefaultConfig().Timeout
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = DefaultConfig().MaxRetries
	}
	if cfg.RateLimit <= 0 || cfg.RateLimit > 500 {
		cfg.RateLimit = DefaultConfig().RateLimit
	}

	maxTokens := float64(cfg.RateLimit)
	return &Client{
		config:     cfg,
		http:       &http.Client{Timeout: cfg.Timeout},
		tokens:     maxTokens,
		maxTokens:  maxTokens,
		refillRate: maxTokens / 60.0, // per second
		lastRefill: time.Now(),
	}
}

// MaskedToken returns the token with all but the last 4 characters masked.
func (c *Client) MaskedToken() string {
	t := c.config.Token
	if len(t) <= 4 {
		return "***"
	}
	return strings.Repeat("*", len(t)-4) + t[len(t)-4:]
}

// request is the Tushare API request body.
type request struct {
	APIName string            `json:"api_name"`
	Token   string            `json:"token"`
	Params  map[string]string `json:"params"`
	Fields  string            `json:"fields"`
}

// response is the Tushare API response body.
type response struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data *struct {
		Fields []string        `json:"fields"`
		Items  [][]interface{} `json:"items"`
	} `json:"data"`
}

// waitForToken blocks until a rate-limit token is available.
func (c *Client) waitForToken(ctx context.Context) error {
	for {
		c.mu.Lock()
		now := time.Now()
		elapsed := now.Sub(c.lastRefill).Seconds()
		c.tokens = math.Min(c.maxTokens, c.tokens+elapsed*c.refillRate)
		c.lastRefill = now

		if c.tokens >= 1 {
			c.tokens--
			c.mu.Unlock()
			return nil
		}
		// Calculate wait time for next token
		waitDur := time.Duration((1 - c.tokens) / c.refillRate * float64(time.Second))
		c.mu.Unlock()

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(waitDur):
		}
	}
}

// Query executes a generic Tushare API call and returns the result as a slice
// of maps. This is the single entry point for all Tushare API interactions.
func (c *Client) Query(ctx context.Context, apiName string, params map[string]string, fields []string) ([]map[string]interface{}, error) {
	if err := c.waitForToken(ctx); err != nil {
		return nil, fmt.Errorf("tushare: rate limit wait cancelled: %w", err)
	}

	reqBody := request{
		APIName: apiName,
		Token:   c.config.Token,
		Params:  params,
		Fields:  strings.Join(fields, ","),
	}

	var lastErr error
	for attempt := 0; attempt <= c.config.MaxRetries; attempt++ {
		if attempt > 0 {
			backoff := time.Duration(math.Pow(2, float64(attempt-1))) * 200 * time.Millisecond
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
			}
		}

		result, err := c.doRequest(ctx, reqBody)
		if err == nil {
			return result, nil
		}

		// Do not retry permission denied errors
		if err == ErrPermissionDenied {
			return nil, err
		}

		lastErr = err
	}

	return nil, fmt.Errorf("tushare: all %d retries failed for %s: %w", c.config.MaxRetries, apiName, lastErr)
}

// doRequest performs a single HTTP request to the Tushare API.
func (c *Client) doRequest(ctx context.Context, reqBody request) ([]map[string]interface{}, error) {
	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("tushare: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.BaseURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("tushare: create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("tushare: http error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("tushare: server error: HTTP %d", resp.StatusCode)
	}

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("tushare: read response: %w", err)
	}

	var tsResp response
	if err := json.Unmarshal(respBytes, &tsResp); err != nil {
		return nil, fmt.Errorf("tushare: unmarshal response: %w", err)
	}

	if tsResp.Code == 2002 {
		return nil, ErrPermissionDenied
	}
	if tsResp.Code != 0 {
		return nil, fmt.Errorf("tushare: API error code %d: %s", tsResp.Code, tsResp.Msg)
	}

	if tsResp.Data == nil {
		return nil, nil
	}

	return columnarToMaps(tsResp.Data.Fields, tsResp.Data.Items), nil
}

// columnarToMaps converts Tushare's columnar response (fields + items) to a
// slice of maps keyed by field name.
func columnarToMaps(fields []string, items [][]interface{}) []map[string]interface{} {
	result := make([]map[string]interface{}, 0, len(items))
	for _, row := range items {
		m := make(map[string]interface{}, len(fields))
		for i, f := range fields {
			if i < len(row) {
				m[f] = row[i]
			}
		}
		result = append(result, m)
	}
	return result
}

// ----- Convenience typed methods -----

// Daily returns daily bar data (unadjusted).
func (c *Client) Daily(ctx context.Context, tsCode, startDate, endDate string) ([]DailyBar, error) {
	params := map[string]string{"ts_code": tsCode}
	if startDate != "" {
		params["start_date"] = startDate
	}
	if endDate != "" {
		params["end_date"] = endDate
	}
	rows, err := c.Query(ctx, "daily", params, dailyBarFields)
	if err != nil {
		return nil, err
	}
	return parseDailyBars(rows), nil
}

// Weekly returns weekly bar data.
func (c *Client) Weekly(ctx context.Context, tsCode, startDate, endDate string) ([]DailyBar, error) {
	params := map[string]string{"ts_code": tsCode}
	if startDate != "" {
		params["start_date"] = startDate
	}
	if endDate != "" {
		params["end_date"] = endDate
	}
	rows, err := c.Query(ctx, "weekly", params, dailyBarFields)
	if err != nil {
		return nil, err
	}
	return parseDailyBars(rows), nil
}

// Monthly returns monthly bar data.
func (c *Client) Monthly(ctx context.Context, tsCode, startDate, endDate string) ([]DailyBar, error) {
	params := map[string]string{"ts_code": tsCode}
	if startDate != "" {
		params["start_date"] = startDate
	}
	if endDate != "" {
		params["end_date"] = endDate
	}
	rows, err := c.Query(ctx, "monthly", params, dailyBarFields)
	if err != nil {
		return nil, err
	}
	return parseDailyBars(rows), nil
}

// AdjFactor returns adjustment factor data.
func (c *Client) AdjFactor(ctx context.Context, tsCode, startDate, endDate string) ([]AdjFactorRow, error) {
	params := map[string]string{"ts_code": tsCode}
	if startDate != "" {
		params["start_date"] = startDate
	}
	if endDate != "" {
		params["end_date"] = endDate
	}
	rows, err := c.Query(ctx, "adj_factor", params, adjFactorFields)
	if err != nil {
		return nil, err
	}
	return parseAdjFactors(rows), nil
}

// StockBasic returns A-share stock list.
func (c *Client) StockBasic(ctx context.Context, listStatus string) ([]StockInfo, error) {
	params := map[string]string{}
	if listStatus != "" {
		params["list_status"] = listStatus
	}
	rows, err := c.Query(ctx, "stock_basic", params, stockInfoFields)
	if err != nil {
		return nil, err
	}
	return parseStockInfos(rows), nil
}

// TradeCal returns trading calendar.
func (c *Client) TradeCal(ctx context.Context, exchange, startDate, endDate string) ([]CalendarDay, error) {
	params := map[string]string{}
	if exchange != "" {
		params["exchange"] = exchange
	}
	if startDate != "" {
		params["start_date"] = startDate
	}
	if endDate != "" {
		params["end_date"] = endDate
	}
	rows, err := c.Query(ctx, "trade_cal", params, calendarDayFields)
	if err != nil {
		return nil, err
	}
	return parseCalendarDays(rows), nil
}

// DailyBasic returns daily basic indicators (PE, PB, etc.).
func (c *Client) DailyBasic(ctx context.Context, tsCode, tradeDate string) ([]DailyIndicator, error) {
	params := map[string]string{}
	if tsCode != "" {
		params["ts_code"] = tsCode
	}
	if tradeDate != "" {
		params["trade_date"] = tradeDate
	}
	rows, err := c.Query(ctx, "daily_basic", params, dailyIndicatorFields)
	if err != nil {
		return nil, err
	}
	return parseDailyIndicators(rows), nil
}

// Income returns income statement data.
func (c *Client) Income(ctx context.Context, tsCode, period string) ([]IncomeStatement, error) {
	params := map[string]string{"ts_code": tsCode}
	if period != "" {
		params["period"] = period
	}
	rows, err := c.Query(ctx, "income", params, incomeFields)
	if err != nil {
		return nil, err
	}
	return parseIncomeStatements(rows), nil
}

// BalanceSheet returns balance sheet data.
func (c *Client) BalanceSheet(ctx context.Context, tsCode, period string) ([]BalanceSheetRow, error) {
	params := map[string]string{"ts_code": tsCode}
	if period != "" {
		params["period"] = period
	}
	rows, err := c.Query(ctx, "balancesheet", params, balanceSheetFields)
	if err != nil {
		return nil, err
	}
	return parseBalanceSheetRows(rows), nil
}

// CashFlow returns cash flow statement data.
func (c *Client) CashFlow(ctx context.Context, tsCode, period string) ([]CashFlowRow, error) {
	params := map[string]string{"ts_code": tsCode}
	if period != "" {
		params["period"] = period
	}
	rows, err := c.Query(ctx, "cashflow", params, cashFlowFields)
	if err != nil {
		return nil, err
	}
	return parseCashFlowRows(rows), nil
}

// FinaIndicator returns financial indicator data.
func (c *Client) FinaIndicator(ctx context.Context, tsCode, period string) ([]FinaIndicatorRow, error) {
	params := map[string]string{"ts_code": tsCode}
	if period != "" {
		params["period"] = period
	}
	rows, err := c.Query(ctx, "fina_indicator", params, finaIndicatorFields)
	if err != nil {
		return nil, err
	}
	return parseFinaIndicatorRows(rows), nil
}

// IndexWeight returns index constituent weights.
func (c *Client) IndexWeight(ctx context.Context, indexCode, tradeDate string) ([]IndexConst, error) {
	params := map[string]string{"index_code": indexCode}
	if tradeDate != "" {
		params["trade_date"] = tradeDate
	}
	rows, err := c.Query(ctx, "index_weight", params, indexConstFields)
	if err != nil {
		return nil, err
	}
	return parseIndexConsts(rows), nil
}

// Dividend returns dividend data.
func (c *Client) Dividend(ctx context.Context, tsCode string) ([]DividendRow, error) {
	params := map[string]string{"ts_code": tsCode}
	rows, err := c.Query(ctx, "dividend", params, dividendFields)
	if err != nil {
		return nil, err
	}
	return parseDividendRows(rows), nil
}
