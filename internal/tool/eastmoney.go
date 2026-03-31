package tool

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"genFu/internal/tool/eastmoneyclient"
)

type StockQuote struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Price      float64 `json:"price"`
	Change     float64 `json:"change"`
	ChangeRate float64 `json:"change_rate"`
	Amount     float64 `json:"amount"`
}

type MarketItem struct {
	Code       string  `json:"code"`
	Name       string  `json:"name"`
	Price      float64 `json:"price"`
	Change     float64 `json:"change"`
	ChangeRate float64 `json:"change_rate"`
	Amount     float64 `json:"amount"`
	Amplitude  float64 `json:"amplitude"`
}

type EastMoneyTool struct {
	options EastMoneyOptions
}

type EastMoneyOptions struct {
	Timeout     time.Duration
	MaxRetries  int
	MinInterval time.Duration
	Referer     string
	UserAgent   string
}

func NewEastMoneyTool() EastMoneyTool {
	return NewEastMoneyToolWithOptions(EastMoneyOptions{})
}

func NewEastMoneyToolWithCookie(_ string) EastMoneyTool {
	return NewEastMoneyToolWithOptions(EastMoneyOptions{})
}

func NewEastMoneyToolWithOptions(opts EastMoneyOptions) EastMoneyTool {
	return EastMoneyTool{options: normalizeEastMoneyOptions(opts)}
}

func (t EastMoneyTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "eastmoney",
		Description: "fetch stock market data",
		Params: map[string]string{
			"action":    "string",
			"code":      "string",
			"page":      "number",
			"page_size": "number",
		},
		Required: []string{"action"},
	}
}

func (t EastMoneyTool) Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	ctx = withEastMoneyOptions(ctx, t.options)
	action, err := requireString(args, "action")
	if err != nil {
		return ToolResult{Name: "eastmoney", Error: err.Error()}, err
	}
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "get_stock_quote":
		code, err := requireString(args, "code")
		if err != nil {
			return ToolResult{Name: "eastmoney", Error: err.Error()}, err
		}
		quote, err := fetchStockQuote(ctx, code)
		return ToolResult{Name: "eastmoney", Output: quote, Error: errorString(err)}, err
	case "get_stock_list":
		page, _ := optionalInt(args, "page")
		pageSize, _ := optionalInt(args, "page_size")
		items, err := fetchStockList(ctx, page, pageSize)
		return ToolResult{Name: "eastmoney", Output: items, Error: errorString(err)}, err
	default:
		return ToolResult{Name: "eastmoney", Error: "unsupported_action"}, errors.New("unsupported_action")
	}
}

type eastmoneyQuoteResp struct {
	Data *eastmoneyQuoteData `json:"data"`
}

type eastmoneyQuoteData struct {
	Code       string          `json:"f57"`
	Name       string          `json:"f58"`
	Price      json.RawMessage `json:"f43"`
	Change     json.RawMessage `json:"f169"`
	ChangeRate json.RawMessage `json:"f170"`
	Amount     json.RawMessage `json:"f48"`
}

type eastmoneyListResp struct {
	Data *eastmoneyListData `json:"data"`
}

type eastmoneyListData struct {
	Diff json.RawMessage `json:"diff,omitempty"`
}

type eastmoneyListItem struct {
	Code       string          `json:"f12"`
	Name       string          `json:"f14"`
	Price      json.RawMessage `json:"f2"`
	Change     json.RawMessage `json:"f4"`
	ChangeRate json.RawMessage `json:"f3"`
	Amount     json.RawMessage `json:"f6"`
	Amplitude  json.RawMessage `json:"f7"`
}

const (
	maxEastmoneyListPageSize = 200
	eastmoneyUTToken         = "bd1d9ddb04089700cf9c27f6f7426281"
)

var eastmoneyHTTPClient = newEastmoneyHTTPClient()

var eastmoneyRealtimeFallbackHosts = []string{
	"https://push2.eastmoney.com",
	"https://82.push2.eastmoney.com",
}

var eastmoneyHistoricalFallbackHosts = []string{
	"https://push2his.eastmoney.com",
	"https://push2.eastmoney.com",
	"https://82.push2.eastmoney.com",
}

func newEastmoneyHTTPClient() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{
		Timeout: 12 * time.Second,
		Jar:     jar,
		Transport: &http.Transport{
			Proxy:               http.ProxyFromEnvironment,
			DialContext:         (&net.Dialer{Timeout: 6 * time.Second, KeepAlive: 15 * time.Second}).DialContext,
			ForceAttemptHTTP2:   true,
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 10,
			IdleConnTimeout:     30 * time.Second,
			TLSHandshakeTimeout: 6 * time.Second,
			TLSClientConfig: &tls.Config{
				MinVersion: tls.VersionTLS12,
			},
			ResponseHeaderTimeout: 10 * time.Second,
			ExpectContinueTimeout: 1 * time.Second,
			DisableCompression:    false,
			DisableKeepAlives:     false,
		},
	}
}

func fetchStockQuote(ctx context.Context, code string) (StockQuote, error) {
	secid := normalizeSecID(code)
	if secid == "" {
		return StockQuote{}, errors.New("invalid_code")
	}
	fields := "f57,f58,f43,f169,f170,f48"
	endpoint := "https://push2.eastmoney.com/api/qt/stock/get"
	params := url.Values{}
	params.Set("secid", secid)
	params.Set("fields", fields)
	params.Set("invt", "2")
	params.Set("fltt", "2")
	respBody, err := doEastMoneyRequest(ctx, endpoint, params)
	if err != nil {
		return StockQuote{}, err
	}
	var parsed eastmoneyQuoteResp
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return StockQuote{}, err
	}
	if parsed.Data == nil {
		return StockQuote{}, errors.New("empty_response")
	}
	return StockQuote{
		Code:       parsed.Data.Code,
		Name:       parsed.Data.Name,
		Price:      parseRawMessageFloat(parsed.Data.Price),
		Change:     parseRawMessageFloat(parsed.Data.Change),
		ChangeRate: scalePercent(parseRawMessageFloat(parsed.Data.ChangeRate)),
		Amount:     parseRawMessageFloat(parsed.Data.Amount),
	}, nil
}

func fetchStockList(ctx context.Context, page int, pageSize int) ([]MarketItem, error) {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 50
	}
	if pageSize > maxEastmoneyListPageSize {
		items, err := fetchStockListChunked(ctx, page, pageSize, maxEastmoneyListPageSize)
		if err == nil {
			return items, nil
		}
		// 分片全量拉取失败时，降级尝试较小样本，保证选股链路可继续。
		log.Printf("[eastmoney] chunked stock list failed page=%d requested=%d err=%v", page, pageSize, err)
		return fetchStockListBestEffort(ctx, page, pageSize, err)
	}
	return fetchStockListPage(ctx, page, pageSize)
}

func fetchStockListBestEffort(ctx context.Context, page int, requested int, baseErr error) ([]MarketItem, error) {
	sizes := []int{maxEastmoneyListPageSize, 120, 80, 50}
	seen := map[int]struct{}{}
	lastErr := baseErr
	for _, size := range sizes {
		if size <= 0 || size > maxEastmoneyListPageSize {
			continue
		}
		if requested > 0 && size > requested {
			size = requested
		}
		if size <= 0 {
			continue
		}
		if _, ok := seen[size]; ok {
			continue
		}
		seen[size] = struct{}{}
		items, err := fetchStockListPage(ctx, page, size)
		if err == nil && len(items) > 0 {
			if size < requested {
				log.Printf("[eastmoney] stock list degraded requested=%d actual=%d", requested, len(items))
			}
			return items, nil
		}
		if err != nil {
			lastErr = err
			log.Printf("[eastmoney] stock list degraded attempt failed page=%d size=%d err=%v", page, size, err)
		}
	}
	if lastErr == nil {
		lastErr = errors.New("stock_list_best_effort_failed")
	}
	return nil, lastErr
}

func fetchStockListChunked(ctx context.Context, page int, pageSize int, chunkSize int) ([]MarketItem, error) {
	if chunkSize <= 0 {
		chunkSize = maxEastmoneyListPageSize
	}
	offset := (page - 1) * pageSize
	remaining := pageSize
	result := make([]MarketItem, 0, pageSize)

	for remaining > 0 {
		apiPage := (offset / chunkSize) + 1
		offsetInPage := offset % chunkSize

		items, err := fetchStockListPage(ctx, apiPage, chunkSize)
		if err != nil {
			log.Printf("[eastmoney] chunk fetch failed page=%d size=%d err=%v", apiPage, chunkSize, err)
			if len(result) > 0 {
				return result, nil
			}
			return nil, err
		}
		if offsetInPage >= len(items) {
			break
		}

		available := len(items) - offsetInPage
		take := available
		if take > remaining {
			take = remaining
		}

		result = append(result, items[offsetInPage:offsetInPage+take]...)
		remaining -= take
		offset += take

		if len(items) < chunkSize {
			break
		}
	}
	if len(result) == 0 {
		return nil, errors.New("no_items_found")
	}
	return result, nil
}

func fetchStockListPage(ctx context.Context, page int, pageSize int) ([]MarketItem, error) {
	fields := "f12,f14,f2,f4,f3,f6,f7"
	endpoint := "https://push2.eastmoney.com/api/qt/clist/get"
	params := url.Values{}
	params.Set("pn", fmt.Sprintf("%d", page))
	params.Set("pz", fmt.Sprintf("%d", pageSize))
	params.Set("po", "1")
	params.Set("np", "1")
	params.Set("fields", fields)
	params.Set("fs", "m:0+t:6,m:0+t:13,m:1+t:2,m:1+t:23")
	params.Set("fid", "f3")
	params.Set("ut", eastmoneyUTToken)
	params.Set("invt", "2")
	params.Set("fltt", "2")
	params.Set("_", fmt.Sprintf("%d", time.Now().UnixMilli()))
	respBody, err := doEastMoneyRequest(ctx, endpoint, params)
	if err != nil {
		return nil, err
	}
	var parsed eastmoneyListResp
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return nil, err
	}
	if parsed.Data == nil || len(parsed.Data.Diff) == 0 {
		return nil, errors.New("empty_response")
	}

	// 尝试解析 diff，可能是数组或对象
	var items []eastmoneyListItem
	diff := bytes.TrimSpace(parsed.Data.Diff)
	if len(diff) == 0 {
		return nil, errors.New("empty_diff")
	}
	firstChar := diff[0]
	if firstChar == '[' {
		// 数组格式
		if err := json.Unmarshal(diff, &items); err != nil {
			return nil, fmt.Errorf("parse diff array: %w", err)
		}
	} else if firstChar == '{' {
		// 对象格式 - 可能是 map
		var diffMap map[string]eastmoneyListItem
		if err := json.Unmarshal(diff, &diffMap); err != nil {
			return nil, fmt.Errorf("parse diff map: %w", err)
		}
		items = make([]eastmoneyListItem, 0, len(diffMap))
		for _, item := range diffMap {
			items = append(items, item)
		}
	} else {
		return nil, fmt.Errorf("unexpected diff format: %c", firstChar)
	}

	if len(items) == 0 {
		return nil, errors.New("no_items_found")
	}

	result := make([]MarketItem, 0, len(items))
	for _, item := range items {
		result = append(result, MarketItem{
			Code:       item.Code,
			Name:       item.Name,
			Price:      parseRawMessageFloat(item.Price),
			Change:     parseRawMessageFloat(item.Change),
			ChangeRate: scalePercent(parseRawMessageFloat(item.ChangeRate)),
			Amount:     parseRawMessageFloat(item.Amount),
			Amplitude:  scalePercent(parseRawMessageFloat(item.Amplitude)),
		})
	}
	return result, nil
}

func parseRawMessageFloat(raw json.RawMessage) float64 {
	if raw == nil || len(raw) == 0 {
		return 0
	}
	// 尝试解析为数字
	var f float64
	if err := json.Unmarshal(raw, &f); err == nil {
		return f
	}
	// 尝试解析为字符串
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		if s == "" || s == "-" {
			return 0
		}
		var f float64
		fmt.Sscanf(s, "%f", &f)
		return f
	}
	return 0
}

func doEastMoneyRequest(ctx context.Context, endpoint string, params url.Values) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	opts := eastMoneyOptionsFromContext(ctx)
	retries := opts.MaxRetries
	if retries <= 0 {
		retries = 1
	}
	var lastErr error
	for attempt := 1; attempt <= retries; attempt++ {
		attemptCtx := ctx
		cancel := func() {}
		if opts.Timeout > 0 {
			if deadline, ok := ctx.Deadline(); !ok || time.Until(deadline) > opts.Timeout {
				attemptCtx, cancel = context.WithTimeout(ctx, opts.Timeout)
			}
		}
		var (
			body []byte
			err  error
		)
		if shouldUseUTLSBackend(endpoint) {
			body, err = doEastMoneyRequestWithUTLSHostFallback(attemptCtx, endpoint, params, opts)
		} else {
			body, err = doEastMoneyRequestWithHostFallback(attemptCtx, endpoint, params, opts)
		}
		cancel()
		if err == nil {
			return body, nil
		}
		lastErr = err
		if attempt >= retries || !shouldRetryEastMoney(err) {
			return nil, err
		}
		log.Printf("[eastmoney] request retry attempt=%d endpoint=%s err=%v", attempt+1, eastmoneyEndpointLabel(endpoint), err)
		wait := time.Duration(attempt*200) * time.Millisecond
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
	return nil, lastErr
}

func doEastMoneyRequestWithUTLSHostFallback(ctx context.Context, endpoint string, params url.Values, opts EastMoneyOptions) ([]byte, error) {
	hosts := candidateHostsForEndpoint(endpoint)
	client := eastMoneyUTLSClientFromOptions(opts)
	var lastErr error
	for _, host := range hosts {
		resolved := replaceEastmoneyHost(endpoint, host)
		targetURL := resolved + "?" + params.Encode()
		body, err := client.GetWithContext(ctx, targetURL)
		if err == nil {
			if len(body) == 0 {
				err = errors.New("empty_response_body")
			} else {
				return body, nil
			}
		}
		lastErr = err
		log.Printf("[eastmoney] utls host failed endpoint=%s host=%s err=%v", eastmoneyEndpointLabel(resolved), host, err)
	}
	if lastErr == nil {
		lastErr = errors.New("eastmoney_all_hosts_failed")
	}
	return nil, lastErr
}

func doEastMoneyRequestWithHostFallback(ctx context.Context, endpoint string, params url.Values, opts EastMoneyOptions) ([]byte, error) {
	hosts := candidateHostsForEndpoint(endpoint)
	var lastErr error
	for _, host := range hosts {
		resolved := replaceEastmoneyHost(endpoint, host)
		warmupEastmoneySession(ctx, resolved, opts)
		body, err := doEastMoneyRequestOnce(ctx, resolved, params, opts)
		if err == nil {
			return body, nil
		}
		lastErr = err
		log.Printf("[eastmoney] host fallback failed endpoint=%s host=%s err=%v", eastmoneyEndpointLabel(resolved), host, err)
	}
	if lastErr == nil {
		lastErr = errors.New("eastmoney_all_hosts_failed")
	}
	return nil, lastErr
}

func doEastMoneyRequestOnce(ctx context.Context, endpoint string, params url.Values, opts EastMoneyOptions) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", opts.UserAgent)
	req.Header.Set("Referer", opts.Referer)
	req.Header.Set("Origin", "https://quote.eastmoney.com")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Sec-Fetch-Site", "same-site")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Dest", "empty")

	resp, err := eastmoneyHTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return nil, fmt.Errorf("eastmoney_http_%d", resp.StatusCode)
	}
	if resp.StatusCode >= 400 {
		return nil, errors.New("eastmoney_request_failed")
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if len(body) == 0 {
		return nil, errors.New("empty_response_body")
	}
	return body, nil
}

func shouldUseUTLSBackend(endpoint string) bool {
	if isLegacyHTTPOnlyEndpoint(endpoint) {
		return false
	}
	normalized := strings.ToLower(endpoint)
	switch {
	case strings.Contains(normalized, "/api/qt/stock/get"),
		strings.Contains(normalized, "/api/qt/clist/get"),
		strings.Contains(normalized, "/api/qt/stock/kline/get"):
		return true
	default:
		return false
	}
}

func isLegacyHTTPOnlyEndpoint(endpoint string) bool {
	normalized := strings.ToLower(endpoint)
	return strings.Contains(normalized, "/api/qt/stock/trends2/get") ||
		strings.Contains(normalized, "fundf10.eastmoney.com") ||
		strings.Contains(normalized, "fundgz.1234567.com.cn")
}

type eastMoneyOptionsKey struct{}

func withEastMoneyOptions(ctx context.Context, opts EastMoneyOptions) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, eastMoneyOptionsKey{}, normalizeEastMoneyOptions(opts))
}

func eastMoneyOptionsFromContext(ctx context.Context) EastMoneyOptions {
	if ctx == nil {
		return normalizeEastMoneyOptions(EastMoneyOptions{})
	}
	v := ctx.Value(eastMoneyOptionsKey{})
	opts, ok := v.(EastMoneyOptions)
	if !ok {
		return normalizeEastMoneyOptions(EastMoneyOptions{})
	}
	return normalizeEastMoneyOptions(opts)
}

func normalizeEastMoneyOptions(opts EastMoneyOptions) EastMoneyOptions {
	defaults := eastmoneyclient.DefaultConfig()
	if opts.Timeout <= 0 {
		opts.Timeout = defaults.Timeout
	}
	if opts.MaxRetries <= 0 {
		opts.MaxRetries = defaults.MaxRetries
	}
	if opts.MinInterval <= 0 {
		opts.MinInterval = defaults.MinInterval
	}
	opts.Referer = strings.TrimSpace(opts.Referer)
	if opts.Referer == "" {
		opts.Referer = defaults.Referer
	}
	opts.UserAgent = strings.TrimSpace(opts.UserAgent)
	if opts.UserAgent == "" {
		opts.UserAgent = defaults.UserAgent
	}
	return opts
}

func eastMoneyUTLSClientFromOptions(opts EastMoneyOptions) *eastmoneyclient.Client {
	return eastmoneyclient.NewClientWithConfig(eastmoneyclient.Config{
		Timeout:     opts.Timeout,
		MaxRetries:  1,
		MinInterval: opts.MinInterval,
		Referer:     opts.Referer,
		UserAgent:   opts.UserAgent,
	})
}

func warmupEastmoneySession(ctx context.Context, endpoint string, opts EastMoneyOptions) {
	base := extractEndpointBase(endpoint)
	if base == "" {
		base = "https://quote.eastmoney.com"
	}
	warmupURLs := []string{
		"https://quote.eastmoney.com/",
		"https://quote.eastmoney.com/center/gridlist.html#hs_a_board",
		base + "/",
	}
	for _, u := range warmupURLs {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", opts.UserAgent)
		req.Header.Set("Referer", opts.Referer)
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
		resp, err := eastmoneyHTTPClient.Do(req)
		if err != nil {
			continue
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
	}
}

func candidateHostsForEndpoint(endpoint string) []string {
	fallbackHosts := fallbackHostsForEndpoint(endpoint)
	// Keep original host first, then endpoint-aware fallbacks.
	result := make([]string, 0, len(fallbackHosts)+1)
	seen := map[string]struct{}{}
	original := extractEndpointBase(endpoint)
	if original != "" {
		result = append(result, original)
		seen[original] = struct{}{}
	}
	for _, host := range fallbackHosts {
		if host == "" {
			continue
		}
		if _, ok := seen[host]; ok {
			continue
		}
		result = append(result, host)
		seen[host] = struct{}{}
	}
	return result
}

func fallbackHostsForEndpoint(endpoint string) []string {
	normalized := strings.ToLower(endpoint)
	switch {
	case strings.Contains(normalized, "/api/qt/stock/kline/get"):
		return eastmoneyHistoricalFallbackHosts
	case strings.Contains(normalized, "/api/qt/clist/get"),
		strings.Contains(normalized, "/api/qt/stock/get"),
		strings.Contains(normalized, "/api/qt/stock/trends2/get"):
		return eastmoneyRealtimeFallbackHosts
	default:
		merged := make([]string, 0, len(eastmoneyRealtimeFallbackHosts)+len(eastmoneyHistoricalFallbackHosts))
		merged = append(merged, eastmoneyRealtimeFallbackHosts...)
		merged = append(merged, eastmoneyHistoricalFallbackHosts...)
		return merged
	}
}

func extractEndpointBase(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return ""
	}
	if u.Scheme == "" || u.Host == "" {
		return ""
	}
	return u.Scheme + "://" + u.Host
}

func eastmoneyEndpointLabel(endpoint string) string {
	u, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}
	if u.Scheme == "" || u.Host == "" {
		return endpoint
	}
	return u.Scheme + "://" + u.Host + u.Path
}

func replaceEastmoneyHost(endpoint string, base string) string {
	src, err := url.Parse(endpoint)
	if err != nil {
		return endpoint
	}
	dst, err := url.Parse(base)
	if err != nil {
		return endpoint
	}
	src.Scheme = dst.Scheme
	src.Host = dst.Host
	return src.String()
}

func shouldRetryEastMoney(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) {
		return true
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, keyword := range []string{
		"eof",
		"timeout",
		"connection reset",
		"broken pipe",
		"refused",
		"temporarily unavailable",
		"eastmoney_http_5",
	} {
		if strings.Contains(msg, keyword) {
			return true
		}
	}
	return false
}

func normalizeSecID(code string) string {
	clean := strings.TrimSpace(strings.ToUpper(code))
	if clean == "" {
		return ""
	}
	if strings.Contains(clean, ".") {
		return clean
	}
	if strings.HasPrefix(clean, "SH") {
		return "1." + strings.TrimPrefix(clean, "SH")
	}
	if strings.HasPrefix(clean, "SZ") {
		return "0." + strings.TrimPrefix(clean, "SZ")
	}
	if strings.HasPrefix(clean, "BJ") {
		return "0." + strings.TrimPrefix(clean, "BJ")
	}
	if strings.HasPrefix(clean, "OF") {
		return "0." + strings.TrimPrefix(clean, "OF")
	}
	if strings.HasPrefix(clean, "6") || strings.HasPrefix(clean, "9") {
		return "1." + clean
	}
	return "0." + clean
}

func scalePrice(value float64) float64 {
	// 东方财富API返回的价格已经是元单位，不需要转换
	return value
}

func scalePercent(value float64) float64 {
	// 东方财富API返回的涨跌幅需要除以100得到百分比
	if value == 0 {
		return 0
	}
	return value / 100
}

// scaleAmount 东方财富API返回的成交额已经是元单位，不需要转换
func scaleAmount(value float64) float64 {
	// 东方财富API返回的金额已经是元单位，直接返回
	return value
}

// SearchItem 搜索结果
type SearchItem struct {
	Code      string  `json:"code"`
	Name      string  `json:"name"`
	Type      string  `json:"type"`
	Price     float64 `json:"price,omitempty"`
	Change    float64 `json:"change,omitempty"`
	ChangePct float64 `json:"change_pct,omitempty"`
}

type eastmoneySearchResp struct {
	Datas json.RawMessage `json:"Datas"`
	Data  json.RawMessage `json:"Data"`
}

type eastmoneySearchItem struct {
	Code   string `json:"Code"`
	Name   string `json:"Name"`
	ID     string `json:"ID"`
	Type   string `json:"Type"`
	Mcode  string `json:"Mcode"`
	Market string `json:"MktNum"`
}

type eastmoneySearchAltItem struct {
	Code      string `json:"Code"`
	Name      string `json:"Name"`
	ShortName string `json:"ShortName"`
	FCode     string `json:"FCode"`
}

// SearchInstruments 使用东方财富基金搜索接口进行模糊匹配
func (t EastMoneyTool) SearchInstruments(ctx context.Context, query string, limit int) ([]SearchItem, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return []SearchItem{}, nil
	}
	if limit <= 0 {
		limit = 20
	}

	endpoint := "https://fundsuggest.eastmoney.com/FundSearch/api/FundSearchAPI.ashx"
	params := url.Values{}
	params.Set("m", "1")
	params.Set("key", query)

	targetURL := endpoint + "?" + params.Encode()
	client := eastMoneyUTLSClientFromOptions(t.options)
	log.Printf("[eastmoney] search_instruments backend=utls endpoint=%s", eastmoneyEndpointLabel(endpoint))
	body, err := client.GetWithContext(ctx, targetURL)
	if err != nil {
		return nil, err
	}
	return parseFundSearchResponse(body, limit)
}

// SearchStockByCode 按股票代码搜索并返回单条结果
func (t EastMoneyTool) SearchStockByCode(ctx context.Context, code string) (*SearchItem, error) {
	code = strings.TrimSpace(code)
	if code == "" {
		return nil, nil
	}
	quote, err := fetchStockQuote(withEastMoneyOptions(ctx, t.options), code)
	if err != nil {
		return nil, err
	}
	symbol := strings.TrimSpace(quote.Code)
	if symbol == "" {
		symbol = code
	}
	name := strings.TrimSpace(quote.Name)
	if name == "" {
		name = symbol
	}
	return &SearchItem{
		Code:      symbol,
		Name:      name,
		Type:      "stock",
		Price:     quote.Price,
		Change:    quote.Change,
		ChangePct: quote.ChangeRate,
	}, nil
}

func parseFundSearchResponse(body []byte, limit int) ([]SearchItem, error) {
	raw := unwrapJSONP(body)
	if len(raw) == 0 {
		return []SearchItem{}, nil
	}

	var parsed eastmoneySearchResp
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}

	items := make([]SearchItem, 0, limit)
	seen := make(map[string]struct{})
	appendItem := func(code, name string) bool {
		code = strings.TrimSpace(code)
		name = strings.TrimSpace(name)
		if code == "" || name == "" {
			return false
		}
		if _, ok := seen[code]; ok {
			return false
		}
		seen[code] = struct{}{}
		items = append(items, SearchItem{
			Code: code,
			Name: name,
			Type: "fund",
		})
		return true
	}

	for _, row := range collectFundSearchRows(parsed.Datas) {
		code, name, ok := parseFundSearchDataRow(row)
		if !ok {
			continue
		}
		if appendItem(code, name) && len(items) >= limit {
			return items, nil
		}
	}

	for _, row := range collectFundSearchRows(parsed.Data) {
		code, name, ok := parseFundSearchDataRow(row)
		if !ok {
			continue
		}
		if appendItem(code, name) && len(items) >= limit {
			return items, nil
		}
	}
	return items, nil
}

func collectFundSearchRows(raw json.RawMessage) []json.RawMessage {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return nil
	}

	if trimmed[0] == '[' {
		var rows []json.RawMessage
		if err := json.Unmarshal(trimmed, &rows); err == nil {
			return rows
		}
		return nil
	}

	if trimmed[0] == '{' {
		// 有些返回会把结果按 key 组织成对象，这里转成行列表继续复用解析器。
		var obj map[string]json.RawMessage
		if err := json.Unmarshal(trimmed, &obj); err == nil {
			rows := make([]json.RawMessage, 0, len(obj))
			for _, v := range obj {
				rows = append(rows, v)
			}
			return rows
		}
	}

	return []json.RawMessage{trimmed}
}

func unwrapJSONP(body []byte) []byte {
	raw := strings.TrimSpace(string(body))
	if raw == "" {
		return nil
	}
	if strings.HasPrefix(raw, "(") && strings.HasSuffix(raw, ")") {
		return []byte(strings.TrimSpace(raw[1 : len(raw)-1]))
	}
	if open := strings.IndexByte(raw, '('); open > 0 && strings.HasSuffix(raw, ")") {
		candidate := strings.TrimSpace(raw[open+1 : len(raw)-1])
		if json.Valid([]byte(candidate)) {
			return []byte(candidate)
		}
	}
	return []byte(raw)
}

func parseFundSearchDataLine(line string) (string, string, bool) {
	parts := strings.Split(strings.TrimSpace(line), ",")
	if len(parts) < 2 {
		return "", "", false
	}
	code := strings.TrimSpace(parts[0])
	name := strings.TrimSpace(parts[1])
	if code == "" || name == "" {
		return "", "", false
	}
	return code, name, true
}

func parseFundSearchDataRow(row json.RawMessage) (string, string, bool) {
	if len(row) == 0 {
		return "", "", false
	}

	var s string
	if err := json.Unmarshal(row, &s); err == nil {
		return parseFundSearchDataLine(s)
	}

	var item eastmoneySearchAltItem
	if err := json.Unmarshal(row, &item); err == nil {
		code := strings.TrimSpace(item.Code)
		if code == "" {
			code = strings.TrimSpace(item.FCode)
		}
		name := strings.TrimSpace(item.Name)
		if name == "" {
			name = strings.TrimSpace(item.ShortName)
		}
		if code != "" && name != "" {
			return code, name, true
		}
	}

	var generic map[string]interface{}
	if err := json.Unmarshal(row, &generic); err != nil {
		return "", "", false
	}
	code := firstNonEmptyString(generic, "Code", "CODE", "code", "FCode", "FCODE", "fcode")
	name := firstNonEmptyString(generic, "Name", "NAME", "name", "ShortName", "SHORTNAME", "short_name", "shortname")
	if code == "" || name == "" {
		return "", "", false
	}
	return code, name, true
}

func firstNonEmptyString(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		v, ok := m[key]
		if !ok || v == nil {
			continue
		}
		switch value := v.(type) {
		case string:
			s := strings.TrimSpace(value)
			if s != "" {
				return s
			}
		case fmt.Stringer:
			s := strings.TrimSpace(value.String())
			if s != "" {
				return s
			}
		}
	}
	return ""
}
