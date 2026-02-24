package tool

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"genFu/internal/investment"
)

type KlinePoint struct {
	Time      string  `json:"time"`
	Open      float64 `json:"open"`
	Close     float64 `json:"close"`
	High      float64 `json:"high"`
	Low       float64 `json:"low"`
	Volume    float64 `json:"volume"`
	Amount    float64 `json:"amount,omitempty"`
	Amplitude float64 `json:"amplitude,omitempty"`
}

type IntradayPoint struct {
	Time     string  `json:"time"`
	Price    float64 `json:"price"`
	Volume   float64 `json:"volume"`
	AvgPrice float64 `json:"avg_price,omitempty"`
	Amount   float64 `json:"amount,omitempty"`
}

type FundNAVPoint struct {
	Date         string  `json:"date"`
	UnitNAV      float64 `json:"unit_nav"`
	AccumNAV     float64 `json:"accum_nav"`
	DailyChange  string  `json:"daily_change"`
	Subscription string  `json:"subscription"`
	Redemption   string  `json:"redemption"`
	Dividend     string  `json:"dividend"`
}

type FundRealtimeNAV struct {
	Code      string `json:"code"`
	Name      string `json:"name"`
	Date      string `json:"date"`
	UnitNAV   string `json:"unit_nav"`
	Estimate  string `json:"estimate"`
	EstimateR string `json:"estimate_rate"`
	Time      string `json:"time"`
}

type HoldingMarketData struct {
	Instrument investment.Instrument `json:"instrument"`
	Kline      []KlinePoint          `json:"kline,omitempty"`
	Intraday   []IntradayPoint       `json:"intraday,omitempty"`
	Error      string                `json:"error,omitempty"`
}

type MarketDataTool struct {
	investmentSvc *investment.Service
}

func NewMarketDataTool(svc *investment.Service) MarketDataTool {
	return MarketDataTool{investmentSvc: svc}
}

func (t MarketDataTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "marketdata",
		Description: "fetch stock and fund intraday or kline data",
		Params: map[string]string{
			"action":     "string",
			"code":       "string",
			"symbol":     "string",
			"period":     "string",
			"klt":        "number",
			"adjust":     "string",
			"start":      "string",
			"end":        "string",
			"days":       "number",
			"asset_type": "string",
			"limit":      "number",
		},
		Required: []string{"action"},
	}
}

func (t MarketDataTool) Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	action, err := requireString(args, "action")
	if err != nil {
		return ToolResult{Name: "marketdata", Error: err.Error()}, err
	}
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "get_stock_kline":
		code, err := requireCode(args)
		if err != nil {
			return ToolResult{Name: "marketdata", Error: err.Error()}, err
		}
		period := optionalString(args, "period")
		klt, _ := optionalInt(args, "klt")
		adjust := optionalString(args, "adjust")
		start := optionalString(args, "start")
		end := optionalString(args, "end")
		days, _ := optionalInt(args, "days")
		points, err := fetchKline(ctx, code, period, klt, adjust, start, end, days)
		return ToolResult{Name: "marketdata", Output: points, Error: errorString(err)}, err
	case "get_stock_intraday":
		code, err := requireCode(args)
		if err != nil {
			return ToolResult{Name: "marketdata", Error: err.Error()}, err
		}
		days, _ := optionalInt(args, "days")
		points, err := fetchIntraday(ctx, code, days)
		return ToolResult{Name: "marketdata", Output: points, Error: errorString(err)}, err
	case "get_fund_kline":
		code, err := requireCode(args)
		if err != nil {
			return ToolResult{Name: "marketdata", Error: err.Error()}, err
		}
		start := optionalString(args, "start")
		end := optionalString(args, "end")
		points, err := fetchFundHistoryKline(ctx, code, start, end)
		return ToolResult{Name: "marketdata", Output: points, Error: errorString(err)}, err
	case "get_fund_intraday":
		code, err := requireCode(args)
		if err != nil {
			return ToolResult{Name: "marketdata", Error: err.Error()}, err
		}
		points, err := fetchFundRealtime(ctx, code)
		return ToolResult{Name: "marketdata", Output: points, Error: errorString(err)}, err
	case "get_fund_holdings_kline":
		accountID, err := resolveAccountID(ctx, t.investmentSvc, args)
		if err != nil {
			return ToolResult{Name: "marketdata", Error: err.Error()}, err
		}
		assetType := optionalString(args, "asset_type")
		limit, _ := optionalInt(args, "limit")
		period := optionalString(args, "period")
		klt, _ := optionalInt(args, "klt")
		adjust := optionalString(args, "adjust")
		start := optionalString(args, "start")
		end := optionalString(args, "end")
		output, err := t.fetchHoldingsKline(ctx, accountID, assetType, limit, period, klt, adjust, start, end)
		return ToolResult{Name: "marketdata", Output: output, Error: errorString(err)}, err
	case "get_fund_holdings_intraday":
		accountID, err := resolveAccountID(ctx, t.investmentSvc, args)
		if err != nil {
			return ToolResult{Name: "marketdata", Error: err.Error()}, err
		}
		assetType := optionalString(args, "asset_type")
		limit, _ := optionalInt(args, "limit")
		days, _ := optionalInt(args, "days")
		output, err := t.fetchHoldingsIntraday(ctx, accountID, assetType, limit, days)
		return ToolResult{Name: "marketdata", Output: output, Error: errorString(err)}, err
	default:
		return ToolResult{Name: "marketdata", Error: "unsupported_action"}, errors.New("unsupported_action")
	}
}

func requireCode(args map[string]interface{}) (string, error) {
	code := optionalString(args, "code")
	if code == "" {
		code = optionalString(args, "symbol")
	}
	if strings.TrimSpace(code) == "" {
		return "", errors.New("missing_param_code")
	}
	return code, nil
}

func (t MarketDataTool) fetchHoldingsKline(ctx context.Context, accountID int64, assetType string, limit int, period string, klt int, adjust string, start string, end string) ([]HoldingMarketData, error) {
	positions, err := t.listHoldings(ctx, accountID, assetType)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(positions) > limit {
		positions = positions[:limit]
	}
	output := make([]HoldingMarketData, 0, len(positions))
	for _, pos := range positions {
		item := HoldingMarketData{Instrument: pos.Instrument}
		code := strings.TrimSpace(pos.Instrument.Symbol)
		if code == "" {
			item.Error = "missing_symbol"
			output = append(output, item)
			continue
		}
		points, err := fetchKline(ctx, code, period, klt, adjust, start, end, 0)
		if err != nil {
			item.Error = err.Error()
		} else {
			item.Kline = points
		}
		output = append(output, item)
	}
	return output, nil
}

func (t MarketDataTool) fetchHoldingsIntraday(ctx context.Context, accountID int64, assetType string, limit int, days int) ([]HoldingMarketData, error) {
	positions, err := t.listHoldings(ctx, accountID, assetType)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(positions) > limit {
		positions = positions[:limit]
	}
	output := make([]HoldingMarketData, 0, len(positions))
	for _, pos := range positions {
		item := HoldingMarketData{Instrument: pos.Instrument}
		code := strings.TrimSpace(pos.Instrument.Symbol)
		if code == "" {
			item.Error = "missing_symbol"
			output = append(output, item)
			continue
		}
		points, err := fetchIntraday(ctx, code, days)
		if err != nil {
			item.Error = err.Error()
		} else {
			item.Intraday = points
		}
		output = append(output, item)
	}
	return output, nil
}

func (t MarketDataTool) listHoldings(ctx context.Context, accountID int64, assetType string) ([]investment.Position, error) {
	if t.investmentSvc == nil {
		return nil, errors.New("service_not_initialized")
	}
	positions, err := t.investmentSvc.ListPositions(ctx, accountID)
	if err != nil {
		return nil, err
	}
	target := strings.TrimSpace(strings.ToLower(assetType))
	filtered := make([]investment.Position, 0, len(positions))
	for _, pos := range positions {
		if matchAssetType(pos.Instrument.AssetType, target) {
			filtered = append(filtered, pos)
		}
	}
	return filtered, nil
}

func matchAssetType(value string, target string) bool {
	asset := strings.TrimSpace(strings.ToLower(value))
	if target == "" {
		return isFundAssetType(asset)
	}
	if asset == target {
		return true
	}
	return strings.Contains(asset, target)
}

func isFundAssetType(value string) bool {
	if value == "" {
		return false
	}
	if strings.Contains(value, "fund") {
		return true
	}
	return strings.Contains(value, "基金")
}

type eastmoneyKlineResp struct {
	Data *eastmoneyKlineData `json:"data"`
}

type eastmoneyKlineData struct {
	Klines []string `json:"klines"`
}

type eastmoneyIntradayResp struct {
	Data *eastmoneyIntradayData `json:"data"`
}

type eastmoneyIntradayData struct {
	Trends []string `json:"trends"`
}

func fetchKline(ctx context.Context, code string, period string, klt int, adjust string, start string, end string, days int) ([]KlinePoint, error) {
	secid := normalizeSecID(code)
	if secid == "" {
		return nil, errors.New("invalid_code")
	}

	// 限制日期范围，默认最多查询1年
	kltValue := resolveKlt(period, klt)
	fqt := resolveAdjust(adjust)

	// 解析和验证日期范围
	startTime, endTime, err := parseAndValidateDateRange(start, end, kltValue, days)
	if err != nil {
		return nil, err
	}

	beg := startTime.Format("20060102")
	endDate := endTime.Format("20060102")

	params := url.Values{}
	params.Set("secid", secid)
	params.Set("fields1", "f1,f2,f3,f4,f5,f6")
	params.Set("fields2", "f51,f52,f53,f54,f55,f56,f57,f58")
	params.Set("klt", strconv.Itoa(kltValue))
	params.Set("fqt", strconv.Itoa(fqt))
	params.Set("beg", beg)
	params.Set("end", endDate)
	endpoint := "https://push2his.eastmoney.com/api/qt/stock/kline/get"
	respBody, err := doEastMoneyRequest(ctx, endpoint, params)
	if err != nil {
		return nil, err
	}
	return parseKlineResponse(respBody)
}

// parseAndValidateDateRange 解析并验证日期范围，限制最大查询范围
func parseAndValidateDateRange(start string, end string, klt int, days int) (time.Time, time.Time, error) {
	now := time.Now()

	// 如果指定了days参数，优先使用days
	if days > 0 {
		// 根据K线类型限制最大days
		maxDays := days
		switch klt {
		case 1, 5, 15, 30, 60: // 分钟线
			if days > 30 {
				maxDays = 30
			}
		case 101: // 日线
			if days > 365 {
				maxDays = 365
			}
		case 102: // 周线
			if days > 730 { // 2年
				maxDays = 730
			}
		case 103: // 月线
			if days > 1825 { // 5年
				maxDays = 1825
			}
		default:
			if days > 365 {
				maxDays = 365
			}
		}
		return now.AddDate(0, 0, -maxDays), now, nil
	}

	// 解析开始日期
	var startTime time.Time
	if start != "" {
		var err error
		startTime, err = parseDate(start)
		if err != nil {
			startTime = now.AddDate(-1, 0, 0) // 默认1年前
		}
	} else {
		startTime = now.AddDate(-1, 0, 0) // 默认1年前
	}

	// 解析结束日期
	var endTime time.Time
	if end != "" {
		var err error
		endTime, err = parseDate(end)
		if err != nil {
			endTime = now
		}
	} else {
		endTime = now
	}

	// 根据K线类型限制最大日期范围
	var maxDuration time.Duration
	switch klt {
	case 1, 5, 15, 30, 60: // 分钟线
		maxDuration = 30 * 24 * time.Hour // 最多30天
	case 101: // 日线
		maxDuration = 365 * 24 * time.Hour // 最多1年
	case 102: // 周线
		maxDuration = 2 * 365 * 24 * time.Hour // 最多2年
	case 103: // 月线
		maxDuration = 5 * 365 * 24 * time.Hour // 最多5年
	default:
		maxDuration = 365 * 24 * time.Hour // 默认1年
	}

	// 如果日期范围超过限制，调整开始日期
	duration := endTime.Sub(startTime)
	if duration > maxDuration {
		startTime = endTime.Add(-maxDuration)
	}

	return startTime, endTime, nil
}

// parseDate 解析日期字符串
func parseDate(dateStr string) (time.Time, error) {
	// 尝试多种日期格式
	formats := []string{
		"20060102",
		"2006-01-02",
		"2006/01/02",
	}

	for _, format := range formats {
		if t, err := time.Parse(format, dateStr); err == nil {
			return t, nil
		}
	}

	return time.Time{}, errors.New("invalid_date_format")
}

func fetchIntraday(ctx context.Context, code string, days int) ([]IntradayPoint, error) {
	secid := normalizeSecID(code)
	if secid == "" {
		return nil, errors.New("invalid_code")
	}
	if days <= 0 {
		days = 1
	}
	params := url.Values{}
	params.Set("secid", secid)
	params.Set("fields1", "f1,f2,f3,f4,f5,f6,f7,f8,f9,f10,f11,f12,f13")
	params.Set("fields2", "f51,f52,f53,f54,f55,f56,f57,f58")
	params.Set("ndays", strconv.Itoa(days))
	endpoint := "https://push2.eastmoney.com/api/qt/stock/trends2/get"
	respBody, err := doEastMoneyRequest(ctx, endpoint, params)
	if err != nil {
		return nil, err
	}
	return parseIntradayResponse(respBody)
}

func fetchFundHistoryKline(ctx context.Context, code string, start string, end string) ([]KlinePoint, error) {
	clean := strings.TrimSpace(code)
	if clean == "" {
		return nil, errors.New("invalid_code")
	}

	// 限制日期范围，最多查询1年
	startDate, endDate, err := parseAndValidateFundDateRange(start, end)
	if err != nil {
		return nil, err
	}

	per := 200
	page := 1
	points := []FundNAVPoint{}
	var totalPages int
	for {
		resp, pages, err := fetchFundHistoryPage(ctx, clean, startDate, endDate, page, per)
		if err != nil {
			return nil, err
		}
		points = append(points, resp...)
		if totalPages == 0 {
			totalPages = pages
		}
		if page >= totalPages || pages == 0 {
			break
		}
		page++
	}
	if len(points) == 0 {
		return nil, errors.New("empty_response")
	}
	return fundNAVToKline(points), nil
}

// parseAndValidateFundDateRange 解析并验证基金日期范围，限制最大查询范围
func parseAndValidateFundDateRange(start string, end string) (string, string, error) {
	now := time.Now()

	// 解析开始日期
	var startTime time.Time
	if start != "" {
		var err error
		startTime, err = parseDate(start)
		if err != nil {
			startTime = now.AddDate(-1, 0, 0) // 默认1年前
		}
	} else {
		startTime = now.AddDate(-1, 0, 0) // 默认1年前
	}

	// 解析结束日期
	var endTime time.Time
	if end != "" {
		var err error
		endTime, err = parseDate(end)
		if err != nil {
			endTime = now
		}
	} else {
		endTime = now
	}

	// 限制最大日期范围为1年
	maxDuration := 365 * 24 * time.Hour
	duration := endTime.Sub(startTime)
	if duration > maxDuration {
		startTime = endTime.Add(-maxDuration)
	}

	return startTime.Format("2006-01-02"), endTime.Format("2006-01-02"), nil
}

func fetchFundHistoryPage(ctx context.Context, code string, start string, end string, page int, per int) ([]FundNAVPoint, int, error) {
	params := url.Values{}
	params.Set("type", "lsjz")
	params.Set("code", code)
	params.Set("page", strconv.Itoa(page))
	params.Set("per", strconv.Itoa(per))
	params.Set("sdate", start)
	params.Set("edate", end)
	endpoint := "https://fundf10.eastmoney.com/F10DataApi.aspx"
	body, err := doFundRequest(ctx, endpoint+"?"+params.Encode())
	if err != nil {
		return nil, 0, err
	}
	return parseFundHistoryResponse(body)
}

func fetchFundRealtime(ctx context.Context, code string) ([]IntradayPoint, error) {
	clean := strings.TrimSpace(code)
	if clean == "" {
		return nil, errors.New("invalid_code")
	}
	endpoint := "https://fundgz.1234567.com.cn/js/" + clean + ".js"
	body, err := doFundRequest(ctx, endpoint)
	if err != nil {
		return nil, err
	}
	nav, err := parseFundRealtime(body)
	if err != nil {
		return nil, err
	}
	price := parseNumber(nav.Estimate)
	avg := parseNumber(nav.UnitNAV)
	if price == 0 {
		price = parseNumber(nav.UnitNAV)
	}
	point := IntradayPoint{
		Time:     strings.TrimSpace(nav.Time),
		Price:    price,
		AvgPrice: avg,
	}
	if point.Time == "" {
		point.Time = strings.TrimSpace(nav.Date)
	}
	if point.Price == 0 {
		return nil, errors.New("empty_response")
	}
	return []IntradayPoint{point}, nil
}

func parseFundHistoryResponse(body []byte) ([]FundNAVPoint, int, error) {
	raw := string(body)
	content, pages, err := extractFundContent(raw)
	if err != nil {
		return nil, 0, err
	}
	rows := extractTableRows(content)
	points := make([]FundNAVPoint, 0, len(rows))
	for _, row := range rows {
		cells := extractCells(row)
		if len(cells) < 3 {
			continue
		}
		point := FundNAVPoint{
			Date:         strings.TrimSpace(cells[0]),
			UnitNAV:      parseNumber(cells[1]),
			AccumNAV:     parseNumber(cells[2]),
			DailyChange:  pickCell(cells, 3),
			Subscription: pickCell(cells, 4),
			Redemption:   pickCell(cells, 5),
			Dividend:     pickCell(cells, 6),
		}
		if point.Date != "" && point.UnitNAV > 0 {
			points = append(points, point)
		}
	}
	if len(points) == 0 {
		return nil, pages, errors.New("empty_response")
	}
	return points, pages, nil
}

func extractFundContent(raw string) (string, int, error) {
	idx := strings.Index(raw, "content:")
	if idx == -1 {
		return "", 0, errors.New("invalid_response")
	}
	rest := raw[idx+len("content:"):]
	rest = strings.TrimSpace(rest)
	if len(rest) == 0 {
		return "", 0, errors.New("invalid_response")
	}
	quote := rest[0]
	if quote != '"' && quote != '\'' {
		return "", 0, errors.New("invalid_response")
	}
	endToken := string(quote) + ",records:"
	endIdx := strings.Index(rest, endToken)
	if endIdx == -1 {
		return "", 0, errors.New("invalid_response")
	}
	content := rest[1:endIdx]
	pages := parsePages(raw)
	return content, pages, nil
}

func parsePages(raw string) int {
	idx := strings.Index(raw, "pages:")
	if idx == -1 {
		return 0
	}
	rest := raw[idx+len("pages:"):]
	end := strings.IndexAny(rest, ",}")
	if end == -1 {
		end = len(rest)
	}
	value := strings.TrimSpace(rest[:end])
	num, err := strconv.Atoi(value)
	if err != nil {
		return 0
	}
	return num
}

func extractTableRows(html string) []string {
	bodyStart := strings.Index(html, "<tbody>")
	bodyEnd := strings.Index(html, "</tbody>")
	if bodyStart == -1 || bodyEnd == -1 || bodyEnd <= bodyStart {
		return nil
	}
	body := html[bodyStart+len("<tbody>") : bodyEnd]
	parts := strings.Split(body, "<tr")
	rows := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.Contains(p, "</tr>") {
			rows = append(rows, p)
		}
	}
	return rows
}

func extractCells(row string) []string {
	out := []string{}
	rest := row
	for {
		tdIdx := strings.Index(rest, "<td")
		if tdIdx == -1 {
			break
		}
		rest = rest[tdIdx+len("<td"):]
		gt := strings.Index(rest, ">")
		if gt == -1 {
			break
		}
		rest = rest[gt+1:]
		end := strings.Index(rest, "</td>")
		if end == -1 {
			break
		}
		cell := rest[:end]
		cell = stripTags(cell)
		out = append(out, strings.TrimSpace(cell))
		rest = rest[end+len("</td>"):]
	}
	return out
}

func stripTags(input string) string {
	out := strings.Builder{}
	inTag := false
	for _, r := range input {
		if r == '<' {
			inTag = true
			continue
		}
		if r == '>' {
			inTag = false
			continue
		}
		if !inTag {
			out.WriteRune(r)
		}
	}
	return out.String()
}

func pickCell(cells []string, idx int) string {
	if idx >= len(cells) {
		return ""
	}
	return strings.TrimSpace(cells[idx])
}

func fundNAVToKline(points []FundNAVPoint) []KlinePoint {
	sort.Slice(points, func(i, j int) bool {
		return points[i].Date < points[j].Date
	})
	out := make([]KlinePoint, 0, len(points))
	var prevClose float64
	for _, p := range points {
		closeVal := p.UnitNAV
		openVal := closeVal
		if prevClose > 0 {
			openVal = prevClose
		}
		highVal := openVal
		if closeVal > highVal {
			highVal = closeVal
		}
		lowVal := openVal
		if closeVal < lowVal {
			lowVal = closeVal
		}
		out = append(out, KlinePoint{
			Time:   p.Date,
			Open:   openVal,
			Close:  closeVal,
			High:   highVal,
			Low:    lowVal,
			Volume: 0,
		})
		prevClose = closeVal
	}
	return out
}

func parseFundRealtime(body []byte) (FundRealtimeNAV, error) {
	raw := strings.TrimSpace(string(body))
	if raw == "" {
		return FundRealtimeNAV{}, errors.New("empty_response")
	}
	start := strings.Index(raw, "{")
	end := strings.LastIndex(raw, "}")
	if start == -1 || end == -1 || end <= start {
		return FundRealtimeNAV{}, errors.New("invalid_response")
	}
	payload := raw[start : end+1]
	var parsed struct {
		FundCode string `json:"fundcode"`
		Name     string `json:"name"`
		JZRQ     string `json:"jzrq"`
		DWJZ     string `json:"dwjz"`
		GSZ      string `json:"gsz"`
		GSZZL    string `json:"gszzl"`
		GZTime   string `json:"gztime"`
	}
	if err := json.Unmarshal([]byte(payload), &parsed); err != nil {
		return FundRealtimeNAV{}, err
	}
	return FundRealtimeNAV{
		Code:      parsed.FundCode,
		Name:      parsed.Name,
		Date:      parsed.JZRQ,
		UnitNAV:   parsed.DWJZ,
		Estimate:  parsed.GSZ,
		EstimateR: parsed.GSZZL,
		Time:      parsed.GZTime,
	}, nil
}

func doFundRequest(ctx context.Context, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
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
		return nil, errors.New("fund_request_failed")
	}
	return io.ReadAll(resp.Body)
}

func normalizeFundDate(value string, fallback time.Time) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return fallback.Format("2006-01-02")
	}
	if t, err := time.Parse("2006-01-02", v); err == nil {
		return t.Format("2006-01-02")
	}
	if len(v) == 8 {
		if t, err := time.Parse("20060102", v); err == nil {
			return t.Format("2006-01-02")
		}
	}
	return fallback.Format("2006-01-02")
}

func parseNumber(value string) float64 {
	v := strings.TrimSpace(value)
	if v == "" {
		return 0
	}
	num, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0
	}
	return num
}

func parseKlineResponse(body []byte) ([]KlinePoint, error) {
	var parsed eastmoneyKlineResp
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if parsed.Data == nil {
		return nil, errors.New("empty_response")
	}
	points := make([]KlinePoint, 0, len(parsed.Data.Klines))
	for _, item := range parsed.Data.Klines {
		point, err := parseKlineRow(item)
		if err != nil {
			return nil, err
		}
		points = append(points, point)
	}
	return points, nil
}

func parseIntradayResponse(body []byte) ([]IntradayPoint, error) {
	var parsed eastmoneyIntradayResp
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	if parsed.Data == nil {
		return nil, errors.New("empty_response")
	}
	points := make([]IntradayPoint, 0, len(parsed.Data.Trends))
	for _, item := range parsed.Data.Trends {
		point := parseIntradayRow(item)
		points = append(points, point)
	}
	return points, nil
}

func parseKlineRow(row string) (KlinePoint, error) {
	parts := strings.Split(row, ",")
	if len(parts) < 6 {
		return KlinePoint{}, errors.New("invalid_kline_row")
	}
	point := KlinePoint{
		Time:   parts[0],
		Open:   parseFloat(parts, 1),
		Close:  parseFloat(parts, 2),
		High:   parseFloat(parts, 3),
		Low:    parseFloat(parts, 4),
		Volume: parseFloat(parts, 5),
	}
	if len(parts) > 6 {
		// 东方财富K线数据的Amount字段已经是元单位，不需要转换
		point.Amount = parseFloat(parts, 6)
	}
	if len(parts) > 7 {
		point.Amplitude = parseFloat(parts, 7)
	}
	return point, nil
}

func parseIntradayRow(row string) IntradayPoint {
	parts := strings.Split(row, ",")
	point := IntradayPoint{}
	if len(parts) == 0 {
		return point
	}
	point.Time = parts[0]
	point.Price = parseFloat(parts, 1)
	point.Volume = parseFloat(parts, 2)
	point.AvgPrice = parseFloat(parts, 3)
	// 东方财富分时数据的Amount字段已经是元单位，不需要转换
	point.Amount = parseFloat(parts, 4)
	return point
}

func parseFloat(parts []string, index int) float64 {
	if index >= len(parts) {
		return 0
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(parts[index]), 64)
	if err != nil {
		return 0
	}
	return value
}

func resolveKlt(period string, klt int) int {
	if klt > 0 {
		return klt
	}
	switch strings.ToLower(strings.TrimSpace(period)) {
	case "1m", "1min", "1minute", "minute":
		return 1
	case "5m", "5min", "5minute":
		return 5
	case "15m", "15min", "15minute":
		return 15
	case "30m", "30min", "30minute":
		return 30
	case "60m", "60min", "60minute", "hour":
		return 60
	case "week", "weekly":
		return 102
	case "month", "monthly":
		return 103
	default:
		return 101
	}
}

func resolveAdjust(adjust string) int {
	value := strings.ToLower(strings.TrimSpace(adjust))
	switch value {
	case "1", "forward", "front", "qfq":
		return 1
	case "2", "backward", "back", "hfq":
		return 2
	default:
		return 0
	}
}

func normalizeDate(value string, fallback string) string {
	clean := strings.TrimSpace(value)
	if clean == "" {
		return fallback
	}
	clean = strings.ReplaceAll(clean, "-", "")
	if len(clean) == 8 {
		return clean
	}
	if len(clean) == 6 {
		return clean + "01"
	}
	return fallback
}
