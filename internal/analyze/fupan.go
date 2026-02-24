package analyze

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
)

var (
	legendLabelPattern = regexp.MustCompile(`(?is)<[^>]*class\s*=\s*["'][^"']*\blegendLabel\b[^"']*["'][^>]*>(.*?)</[^>]+>`)
	legendUpPattern    = regexp.MustCompile(`上涨\s*([0-9,，]+)`)
	legendDownPattern  = regexp.MustCompile(`下跌\s*([0-9,，]+)`)
	legendHaltPattern  = regexp.MustCompile(`停牌\s*([0-9,，]+)`)
	legendFlatPattern  = regexp.MustCompile(`平盘\s*([0-9,，]+)`)
)

const minReasonableBreadthTotal = 500

type FupanIndex struct {
	Name       string `json:"name"`
	Price      string `json:"price"`
	Change     string `json:"change"`
	ChangeRate string `json:"change_rate"`
}

type FupanReport struct {
	Date       string       `json:"date"`
	Summary    string       `json:"summary"`
	Highlights []string     `json:"highlights"`
	Indexes    []FupanIndex `json:"indexes"`
	UpCount    int          `json:"up_count,omitempty"`
	DownCount  int          `json:"down_count,omitempty"`
	HaltCount  int          `json:"halt_count,omitempty"`
	Breadth    string       `json:"breadth_text,omitempty"`
}

func (s *DailyReviewService) fetchFupanReport(ctx context.Context, date string) (*FupanReport, error) {
	url := "https://stock.10jqka.com.cn/fupan/"
	if date != "" {
		url = fmt.Sprintf("https://stock.10jqka.com.cn/fupan/%s.shtml", date)
	}
	return fetchAndParseFupanURL(ctx, url, date)
}

func fetchAndParseFupanURL(ctx context.Context, url string, date string) (*FupanReport, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, errors.New("fupan_request_failed")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(body)
	if err != nil {
		return parseFupanHTML(string(body), date)
	}
	return parseFupanHTML(string(decoded), date)
}

func parseFupanHTML(html string, date string) (*FupanReport, error) {
	report := &FupanReport{}
	report.Summary = cleanHTML(extractByID(html, "block_1887"))
	report.Highlights = splitHighlights(cleanHTML(extractByID(html, "block_1889")))
	report.Indexes = parseFupanIndexes(html)
	report.UpCount, report.HaltCount, report.DownCount, report.Breadth = parseFupanBreadth(html)
	report.Date = extractGlobalDate(html)
	if report.Date == "" {
		report.Date = date
	}
	if report.Summary == "" && len(report.Indexes) == 0 && len(report.Highlights) == 0 {
		return nil, errors.New("fupan_parse_failed")
	}
	return report, nil
}

func extractGlobalDate(html string) string {
	re := regexp.MustCompile(`Global\.date\s*=\s*"(\d{8})"`)
	m := re.FindStringSubmatch(html)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

func parseFupanIndexes(html string) []FupanIndex {
	segment := extractByClass(html, "nav_list")
	if segment == "" {
		return nil
	}
	liRe := regexp.MustCompile(`<li[^>]*>([\s\S]*?)</li>`)
	matches := liRe.FindAllStringSubmatch(segment, -1)
	indexes := []FupanIndex{}
	for _, m := range matches {
		content := stripTags(m[1])
		fields := splitAndClean(content)
		if len(fields) < 4 {
			continue
		}
		canonicalName := canonicalFupanIndexName(fields[0])
		if canonicalName == "" {
			continue
		}
		indexes = append(indexes, FupanIndex{
			Name:       canonicalName,
			Price:      fields[1],
			Change:     fields[2],
			ChangeRate: fields[3],
		})
	}
	return indexes
}

func canonicalFupanIndexName(name string) string {
	normalized := strings.ReplaceAll(strings.TrimSpace(name), " ", "")
	switch normalized {
	case "上证指数":
		return "上证指数"
	case "深证指数", "深证成指":
		return "深证成指"
	case "创业板指":
		return "创业板指"
	case "北证50", "北证50指数":
		return "北证50"
	case "科创50":
		return "科创50"
	case "中证500":
		return "中证500"
	case "中证1000":
		return "中证1000"
	case "沪深300":
		return "沪深300"
	case "恒生指数":
		return "恒生指数"
	case "纳斯达克", "纳斯达克指数":
		return "纳斯达克指数"
	case "道琼斯", "道琼斯指数":
		return "道琼斯指数"
	case "美元指数":
		return "美元指数"
	case "期货连续":
		return "期货连续"
	default:
		return ""
	}
}

func parseFupanBreadth(html string) (int, int, int, string) {
	if up, halt, down, breadth, ok := parseBreadthFromLegendLabels(html); ok {
		return up, halt, down, breadth
	}

	plain := cleanHTML(html)
	if anchor := strings.Index(plain, "个股涨跌图"); anchor >= 0 {
		plain = plain[anchor:]
	}
	re := regexp.MustCompile(`上涨\s*([0-9]+)\s*\([^)]*\)\s*停牌\s*([0-9]+)\s*\([^)]*\)\s*下跌\s*([0-9]+)\s*\([^)]*\)`)
	match := re.FindStringSubmatch(plain)
	if len(match) == 4 {
		up, _ := parseInt(match[1])
		halt, _ := parseInt(match[2])
		down, _ := parseInt(match[3])
		if !isReasonableBreadth(up, down) {
			return 0, 0, 0, ""
		}
		return up, halt, down, match[0]
	}

	up, upOK := parseBreadthCount(plain, legendUpPattern)
	down, downOK := parseBreadthCount(plain, legendDownPattern)
	if !upOK || !downOK {
		return 0, 0, 0, ""
	}

	halt, haltOK := parseBreadthCount(plain, legendHaltPattern)
	if !haltOK {
		halt, _ = parseBreadthCount(plain, legendFlatPattern)
	}
	if !isReasonableBreadth(up, down) {
		return 0, 0, 0, ""
	}
	return up, halt, down, fmt.Sprintf("上涨%d 下跌%d", up, down)
}

func parseBreadthFromLegendLabels(html string) (int, int, int, string, bool) {
	segment := extractBreadthSegment(html)
	// fmt.Print(segment)
	matches := legendLabelPattern.FindAllStringSubmatch(segment, -1)
	// if len(matches) == 0 && segment != html {
	// 	matches = legendLabelPattern.FindAllStringSubmatch(html, -1)
	// }
	if len(matches) == 0 {
		return 0, 0, 0, "", false
	}

	labels := make([]string, 0, len(matches))
	for _, m := range matches {
		if len(m) < 2 {
			continue
		}
		text := cleanHTML(m[1])
		if text != "" {
			labels = append(labels, text)
		}
	}
	if len(labels) == 0 {
		return 0, 0, 0, "", false
	}

	joined := strings.Join(labels, " ")
	up, upOK := parseBreadthCount(joined, legendUpPattern)
	down, downOK := parseBreadthCount(joined, legendDownPattern)
	if !upOK || !downOK {
		return 0, 0, 0, "", false
	}

	halt, haltOK := parseBreadthCount(joined, legendHaltPattern)
	if !haltOK {
		halt, _ = parseBreadthCount(joined, legendFlatPattern)
	}
	if !isReasonableBreadth(up, down) {
		return 0, 0, 0, "", false
	}
	return up, halt, down, joined, true
}

func extractBreadthSegment(html string) string {
	if strings.TrimSpace(html) == "" {
		return html
	}

	// Use structural DOM hints from the fupan page instead of text anchors.
	// Typical targets: fp_item_4 -> chart1 -> pie_legend -> legendLabel / flsh_tip_num.
	lower := strings.ToLower(html)
	candidates := []string{
		`class="pie_legend"`,
		`class='pie_legend'`,
		`id="chart1"`,
		`id='chart1'`,
		`id="fp_item_4"`,
		`id='fp_item_4'`,
		`class="legendlabel"`,
		`class='legendlabel'`,
		`class="flsh_tip_num"`,
		`class='flsh_tip_num'`,
	}

	const (
		prefixWindow = 1500
		suffixWindow = 10000
	)
	for _, candidate := range candidates {
		index := strings.Index(lower, candidate)
		if index < 0 {
			continue
		}
		start := index - prefixWindow
		if start < 0 {
			start = 0
		}
		end := index + len(candidate) + suffixWindow
		if end > len(html) {
			end = len(html)
		}
		return html[start:end]
	}

	return html
}

func parseBreadthCount(text string, pattern *regexp.Regexp) (int, bool) {
	if pattern == nil {
		return 0, false
	}
	m := pattern.FindStringSubmatch(text)
	if len(m) < 2 {
		return 0, false
	}
	return parseInt(normalizeCount(m[1]))
}

func normalizeCount(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.ReplaceAll(s, ",", "")
	s = strings.ReplaceAll(s, "，", "")
	return s
}

func isReasonableBreadth(up int, down int) bool {
	total := up + down
	return total >= minReasonableBreadthTotal
}

func parseInt(s string) (int, bool) {
	var v int
	_, err := fmt.Sscanf(strings.TrimSpace(s), "%d", &v)
	return v, err == nil
}

func extractByID(html, id string) string {
	re := regexp.MustCompile(`<div[^>]*id="` + regexp.QuoteMeta(id) + `"[^>]*>([\s\S]*?)</div>`)
	m := re.FindStringSubmatch(html)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

func extractByClass(html, class string) string {
	re := regexp.MustCompile(`<ul[^>]*class="[^"]*` + regexp.QuoteMeta(class) + `[^"]*"[^>]*>([\s\S]*?)</ul>`)
	m := re.FindStringSubmatch(html)
	if len(m) > 1 {
		return m[1]
	}
	return ""
}

func cleanHTML(text string) string {
	text = stripTags(text)
	text = strings.ReplaceAll(text, "&nbsp;", " ")
	text = strings.ReplaceAll(text, "\n", " ")
	text = strings.ReplaceAll(text, "\t", " ")
	text = strings.TrimSpace(text)
	for strings.Contains(text, "  ") {
		text = strings.ReplaceAll(text, "  ", " ")
	}
	return text
}

func stripTags(text string) string {
	re := regexp.MustCompile(`<[^>]+>`)
	return strings.TrimSpace(re.ReplaceAllString(text, " "))
}

func splitAndClean(text string) []string {
	fields := strings.Fields(text)
	results := []string{}
	for _, f := range fields {
		if f != "" {
			results = append(results, f)
		}
	}
	return results
}

func splitHighlights(text string) []string {
	if text == "" {
		return nil
	}
	parts := strings.Split(text, "。")
	results := []string{}
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		results = append(results, p+"。")
	}
	return results
}

func compareFupanSummary(summary string, report *FupanReport) (map[string]interface{}, error) {
	if report == nil {
		return nil, errors.New("missing_fupan_report")
	}
	features := strings.ToLower(cleanHTML(report.Summary))
	output := map[string]interface{}{
		"fupan_date": report.Date,
		"summary":    report.Summary,
		"highlights": report.Highlights,
		"indexes":    report.Indexes,
	}
	if summary == "" || features == "" {
		output["confidence"] = 0.2
		output["consistent"] = false
		output["diff"] = "缺少可比文本"
		return output, nil
	}
	matchCount := 0
	for _, word := range []string{"上涨", "下跌", "放量", "缩量", "成交额", "反弹", "回落"} {
		if strings.Contains(summary, word) && strings.Contains(features, word) {
			matchCount++
		}
	}
	confidence := 0.3 + float64(matchCount)*0.1
	if confidence > 0.9 {
		confidence = 0.9
	}
	output["confidence"] = confidence
	output["consistent"] = confidence >= 0.6
	output["diff"] = buildDiffHint(summary, report.Summary)
	return output, nil
}

func buildDiffHint(a, b string) string {
	if len(a) == 0 || len(b) == 0 {
		return "数据不足"
	}
	if len(a) > len(b) {
		return "模型摘要较长，可能包含更多细节"
	}
	return "同花顺摘要较长，模型可补充细节"
}

func init() {
	http.DefaultClient.Timeout = 10 * time.Second
}
