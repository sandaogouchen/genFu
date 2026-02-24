package tool

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

type EastMoneyTool struct{}

func NewEastMoneyTool() EastMoneyTool {
	return EastMoneyTool{}
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
	fields := "f12,f14,f2,f4,f3,f6,f7"
	endpoint := "https://push2.eastmoney.com/api/qt/clist/get"
	params := url.Values{}
	params.Set("pn", fmt.Sprintf("%d", page))
	params.Set("pz", fmt.Sprintf("%d", pageSize))
	params.Set("fields", fields)
	params.Set("fs", "m:0+t:6,m:0+t:13,m:1+t:2,m:1+t:23")
	params.Set("invt", "2")
	params.Set("fltt", "2")
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
	firstChar := parsed.Data.Diff[0]
	if firstChar == '[' {
		// 数组格式
		if err := json.Unmarshal(parsed.Data.Diff, &items); err != nil {
			return nil, fmt.Errorf("parse diff array: %w", err)
		}
	} else if firstChar == '{' {
		// 对象格式 - 可能是 map
		var diffMap map[string]eastmoneyListItem
		if err := json.Unmarshal(parsed.Data.Diff, &diffMap); err != nil {
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
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, errors.New("eastmoney_request_failed")
	}
	return io.ReadAll(resp.Body)
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
	Quota   []interface{} `json:"Quota"`
	Data    []eastmoneySearchItem `json:"Data"`
	Total   int `json:"Total"`
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
