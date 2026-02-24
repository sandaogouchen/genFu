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
	"os"
	"os/exec"
	"strings"
	"time"
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
	Cookie      string
	UseCurlCFFI bool
	Impersonate string
	PythonBin   string
}

func NewEastMoneyTool() EastMoneyTool {
	return NewEastMoneyToolWithOptions(EastMoneyOptions{})
}

func NewEastMoneyToolWithCookie(cookie string) EastMoneyTool {
	return NewEastMoneyToolWithOptions(EastMoneyOptions{Cookie: cookie})
}

func NewEastMoneyToolWithOptions(opts EastMoneyOptions) EastMoneyTool {
	options := EastMoneyOptions{
		Cookie:      strings.TrimSpace(opts.Cookie),
		UseCurlCFFI: opts.UseCurlCFFI,
		Impersonate: strings.TrimSpace(opts.Impersonate),
		PythonBin:   strings.TrimSpace(opts.PythonBin),
	}
	if options.Impersonate == "" {
		options.Impersonate = "chrome136"
	}
	if options.PythonBin == "" {
		options.PythonBin = "python3"
	}
	return EastMoneyTool{options: options}
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
	eastmoneyMaxRetries      = 3
	eastmoneyUTToken         = "bd1d9ddb04089700cf9c27f6f7426281"
)

var eastmoneyHTTPClient = newEastmoneyHTTPClient()

var eastmoneyFallbackHosts = []string{
	"https://push2.eastmoney.com",
	"https://82.push2.eastmoney.com",
	"https://push2his.eastmoney.com",
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
		return fetchStockListChunked(ctx, page, pageSize, maxEastmoneyListPageSize)
	}
	return fetchStockListPage(ctx, page, pageSize)
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

func parseStringFloat(s string) float64 {
	if s == "" || s == "-" {
		return 0
	}
	var f float64
	fmt.Sscanf(s, "%f", &f)
	return f
}

func doEastMoneyRequest(ctx context.Context, endpoint string, params url.Values) ([]byte, error) {
	opts := eastMoneyOptionsFromContext(ctx)
	if opts.UseCurlCFFI {
		body, err := doEastMoneyRequestViaCurlCFFI(ctx, endpoint, params, opts)
		if err == nil {
			return body, nil
		}
		log.Printf("[eastmoney] curl_cffi failed, fallback native http err=%v", err)
	}

	var lastErr error
	for attempt := 1; attempt <= eastmoneyMaxRetries; attempt++ {
		body, err := doEastMoneyRequestWithHostFallback(ctx, endpoint, params)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if attempt >= eastmoneyMaxRetries || !shouldRetryEastMoney(err) {
			return nil, err
		}
		log.Printf("[eastmoney] request retry attempt=%d endpoint=%s err=%v", attempt+1, endpoint, err)
		wait := time.Duration(attempt*200) * time.Millisecond
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(wait):
		}
	}
	return nil, lastErr
}

func doEastMoneyRequestWithHostFallback(ctx context.Context, endpoint string, params url.Values) ([]byte, error) {
	hosts := candidateHostsForEndpoint(endpoint)
	var lastErr error
	for _, host := range hosts {
		resolved := replaceEastmoneyHost(endpoint, host)
		warmupEastmoneySession(ctx, resolved)
		body, err := doEastMoneyRequestOnce(ctx, resolved, params)
		if err == nil {
			return body, nil
		}
		lastErr = err
		log.Printf("[eastmoney] host fallback failed host=%s err=%v", host, err)
	}
	if lastErr == nil {
		lastErr = errors.New("eastmoney_all_hosts_failed")
	}
	return nil, lastErr
}

func doEastMoneyRequestOnce(ctx context.Context, endpoint string, params url.Values) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", "https://quote.eastmoney.com/")
	req.Header.Set("Origin", "https://quote.eastmoney.com")
	req.Header.Set("Accept", "application/json, text/javascript, */*; q=0.01")
	req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8")
	req.Header.Set("Sec-Fetch-Site", "same-site")
	req.Header.Set("Sec-Fetch-Mode", "cors")
	req.Header.Set("Sec-Fetch-Dest", "empty")
	if cookie := eastMoneyOptionsFromContext(ctx).Cookie; cookie != "" {
		req.Header.Set("Cookie", cookie)
	}

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

func doEastMoneyRequestViaCurlCFFI(ctx context.Context, endpoint string, params url.Values, opts EastMoneyOptions) ([]byte, error) {
	hosts := candidateHostsForEndpoint(endpoint)
	impersonates := candidateImpersonates(opts.Impersonate)
	var lastErr error
	for _, host := range hosts {
		resolved := replaceEastmoneyHost(endpoint, host)
		for _, impersonate := range impersonates {
			body, err := curlCFFIGet(ctx, resolved, params, opts, impersonate)
			if err == nil {
				return body, nil
			}
			lastErr = err
			log.Printf("[eastmoney] curl_cffi host failed host=%s impersonate=%s err=%v", host, impersonate, err)
			// 非瞬时错误（例如不支持的impersonate）直接尝试下一个指纹，不在当前组合重试
			if isUnsupportedImpersonateError(err) {
				continue
			}
		}
	}
	if lastErr == nil {
		lastErr = errors.New("curl_cffi_all_hosts_failed")
	}
	return nil, lastErr
}

func curlCFFIGet(ctx context.Context, endpoint string, params url.Values, opts EastMoneyOptions, impersonate string) ([]byte, error) {
	targetURL := endpoint + "?" + params.Encode()
	pythonCode := `import os, sys, time
from urllib.parse import urlparse
from curl_cffi import requests

url = sys.argv[1]
impersonate = sys.argv[2] if len(sys.argv) > 2 and sys.argv[2] else "chrome136"
cookie = os.environ.get("EASTMONEY_COOKIE", "").strip()
parsed = urlparse(url)
base = f"{parsed.scheme}://{parsed.netloc}" if parsed.scheme and parsed.netloc else "https://push2.eastmoney.com"

session = requests.Session()
session.headers.update({
    # 保持与页面访问一致，避免API请求特征过于“机器人化”
    "User-Agent": "Mozilla/5.0",
    "Referer": "https://quote.eastmoney.com/",
    "Origin": "https://quote.eastmoney.com",
    "Accept": "application/json, text/javascript, */*; q=0.01",
    "Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
    "Sec-Fetch-Site": "same-site",
    "Sec-Fetch-Mode": "cors",
    "Sec-Fetch-Dest": "empty",
})
if cookie:
    session.headers["Cookie"] = cookie

for warmup in (
    "https://quote.eastmoney.com/",
    "https://quote.eastmoney.com/center/gridlist.html#hs_a_board",
    base + "/",
):
    try:
        session.get(warmup, impersonate=impersonate, timeout=8, default_headers=True)
    except Exception:
        pass

last_err = None
for _ in range(2):
    try:
        resp = session.get(url, impersonate=impersonate, timeout=15, default_headers=True)
        resp.raise_for_status()
        sys.stdout.buffer.write(resp.content)
        sys.exit(0)
    except Exception as exc:
        last_err = exc
        time.sleep(0.25)

raise last_err
	`

	cmd := exec.CommandContext(ctx, opts.PythonBin, "-c", pythonCode, targetURL, impersonate)
	cmd.Env = append(os.Environ(), "EASTMONEY_COOKIE="+opts.Cookie)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errText := strings.TrimSpace(stderr.String())
		if strings.Contains(errText, "No module named 'curl_cffi'") || strings.Contains(errText, "No module named curl_cffi") {
			return nil, fmt.Errorf("curl_cffi_not_installed: %s", errText)
		}
		if errText == "" {
			return nil, err
		}
		return nil, fmt.Errorf("%w: %s", err, errText)
	}
	if stdout.Len() == 0 {
		return nil, errors.New("curl_cffi_empty_response")
	}
	return stdout.Bytes(), nil
}

func candidateImpersonates(primary string) []string {
	result := make([]string, 0, 6)
	appendUnique := func(v string) {
		v = strings.TrimSpace(v)
		if v == "" {
			return
		}
		for _, existing := range result {
			if existing == v {
				return
			}
		}
		result = append(result, v)
	}

	appendUnique(primary)
	appendUnique("chrome")
	appendUnique("chrome136")
	appendUnique("chrome133")
	appendUnique("chrome131")
	appendUnique("safari17_0")
	return result
}

func isUnsupportedImpersonateError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "impersonate") &&
		(strings.Contains(msg, "not supported") || strings.Contains(msg, "unsupported"))
}

type eastMoneyOptionsKey struct{}

func withEastMoneyOptions(ctx context.Context, opts EastMoneyOptions) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, eastMoneyOptionsKey{}, opts)
}

func eastMoneyOptionsFromContext(ctx context.Context) EastMoneyOptions {
	defaults := EastMoneyOptions{
		Impersonate: "chrome136",
		PythonBin:   "python3",
	}
	if ctx == nil {
		return defaults
	}
	v := ctx.Value(eastMoneyOptionsKey{})
	opts, ok := v.(EastMoneyOptions)
	if !ok {
		return defaults
	}
	if strings.TrimSpace(opts.Impersonate) == "" {
		opts.Impersonate = defaults.Impersonate
	}
	if strings.TrimSpace(opts.PythonBin) == "" {
		opts.PythonBin = defaults.PythonBin
	}
	opts.Cookie = strings.TrimSpace(opts.Cookie)
	return opts
}

func warmupEastmoneySession(ctx context.Context, endpoint string) {
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
		req.Header.Set("User-Agent", "Mozilla/5.0")
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
	// Keep original host first, then fallbacks.
	result := make([]string, 0, len(eastmoneyFallbackHosts)+1)
	original := extractEndpointBase(endpoint)
	if original != "" {
		result = append(result, original)
	}
	for _, host := range eastmoneyFallbackHosts {
		if host == "" {
			continue
		}
		if host == original {
			continue
		}
		result = append(result, host)
	}
	return result
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
	Quota []interface{}         `json:"Quota"`
	Data  []eastmoneySearchItem `json:"Data"`
	Total int                   `json:"Total"`
}

type eastmoneySearchItem struct {
	Code   string `json:"Code"`
	Name   string `json:"Name"`
	ID     string `json:"ID"`
	Type   string `json:"Type"`
	Mcode  string `json:"Mcode"`
	Market string `json:"MktNum"`
}

// SearchInstruments 搜索股票和基金
func (t EastMoneyTool) SearchInstruments(ctx context.Context, query string, limit int) ([]SearchItem, error) {
	if limit <= 0 {
		limit = 20
	}
	endpoint := "https://searchapi.eastmoney.com/bussiness/web/QuotationLabelSearch"
	params := url.Values{}
	params.Set("keyword", query)
	params.Set("pagesize", fmt.Sprintf("%d", limit))
	params.Set("type", "stock,fund") // 同时搜索股票和基金
	params.Set("pi", "1")
	params.Set("token", "D43BF722C8E33BDC906FB84D85E326E8")
	params.Set("cb", "")

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	req.Header.Set("Referer", "https://quote.eastmoney.com/")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// 处理 JSONP 响应
	raw := strings.TrimSpace(string(body))
	// 移除 JSONP 回调包装
	if strings.HasPrefix(raw, "(") && strings.HasSuffix(raw, ")") {
		raw = raw[1 : len(raw)-1]
	}

	var parsed eastmoneySearchResp
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil, err
	}

	items := make([]SearchItem, 0, len(parsed.Data))
	for _, d := range parsed.Data {
		item := SearchItem{
			Code: d.Code,
			Name: d.Name,
		}
		// 判断类型
		if strings.Contains(strings.ToLower(d.Type), "fund") || len(d.Code) == 6 && (strings.HasPrefix(d.Code, "0") || strings.HasPrefix(d.Code, "1") || strings.HasPrefix(d.Code, "2") || strings.HasPrefix(d.Code, "3") || strings.HasPrefix(d.Code, "5")) {
			item.Type = "fund"
		} else {
			item.Type = "stock"
		}
		items = append(items, item)
	}

	return items, nil
}
