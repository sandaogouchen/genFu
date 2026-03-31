package financial

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"sync"
	"time"
)

const (
	sseQueryURL     = "http://query.sse.com.cn/security/stock/queryCompanyBulletin.do"
	sseReferer      = "http://www.sse.com.cn"
	sseMinInterval  = 1 * time.Second
)

// SSEClient 上海证券交易所公告查询客户端
type SSEClient struct {
	client      *http.Client
	mu          sync.Mutex
	lastRequest time.Time
}

func NewSSEClient() *SSEClient {
	return &SSEClient{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// QueryBulletins 查询上交所公告
func (c *SSEClient) QueryBulletins(ctx context.Context, query ExchangeQuery) (*AnnouncementResult, error) {
	query.Defaults()
	c.rateLimit()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sseQueryURL, nil)
	if err != nil {
		return nil, fmt.Errorf("build SSE request: %w", err)
	}

	// 构建查询参数
	q := req.URL.Query()
	q.Set("isPagination", "true")
	q.Set("securityType", "0101")
	q.Set("reportType", "ALL")
	q.Set("pageHelp.pageSize", fmt.Sprintf("%d", query.PageSize))
	q.Set("pageHelp.pageNo", fmt.Sprintf("%d", query.PageNum))

	if query.Symbol != "" {
		q.Set("productId", query.Symbol)
	}
	if query.Keyword != "" {
		q.Set("keyWord", query.Keyword)
	}
	if query.StartDate != "" {
		q.Set("startDate", query.StartDate)
	}
	if query.EndDate != "" {
		q.Set("endDate", query.EndDate)
	}
	req.URL.RawQuery = q.Encode()

	// 必须携带 Referer
	req.Header.Set("Referer", sseReferer)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("SSE request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SSE returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read SSE response: %w", err)
	}

	// 剥离 JSONP 包裹
	jsonData := stripJSONP(body)

	var sseResp sseQueryResponse
	if err := json.Unmarshal(jsonData, &sseResp); err != nil {
		return nil, fmt.Errorf("parse SSE response: %w", err)
	}

	return sseResp.toAnnouncementResult(query.PageNum), nil
}

// rateLimit 线程安全的速率控制
func (c *SSEClient) rateLimit() {
	c.mu.Lock()
	defer c.mu.Unlock()
	elapsed := time.Since(c.lastRequest)
	if elapsed < sseMinInterval {
		time.Sleep(sseMinInterval - elapsed)
	}
	c.lastRequest = time.Now()
}

// SSE 响应结构
type sseQueryResponse struct {
	Result []sseBulletin `json:"result"`
	PageHelp struct {
		Total    int `json:"total"`
		PageNo   int `json:"pageNo"`
		PageSize int `json:"pageSize"`
		PageCount int `json:"pageCount"`
	} `json:"pageHelp"`
}

type sseBulletin struct {
	SecurityCode string `json:"SECURITY_CODE"`
	SecurityName string `json:"SECURITY_ABBR_A"`
	Title        string `json:"TITLE"`
	BulletinDate string `json:"BULLETIN_DATE"` // YYYY-MM-DD
	BulletinType string `json:"BULLETIN_TYPE"`
	SSEURL       string `json:"URL"`           // 公告 PDF 相对路径
}

func (r *sseQueryResponse) toAnnouncementResult(currentPage int) *AnnouncementResult {
	result := &AnnouncementResult{
		TotalRecords: r.PageHelp.Total,
		TotalPages:   r.PageHelp.PageCount,
		CurrentPage:  currentPage,
		HasMore:      currentPage < r.PageHelp.PageCount,
	}

	for _, b := range r.Result {
		ann := Announcement{
			SecCode: b.SecurityCode,
			SecName: b.SecurityName,
			Title:   b.Title,
			Column:  "sse",
			PDFURL:  b.SSEURL,
		}
		// 尝试解析日期
		if t, err := time.Parse("2006-01-02", b.BulletinDate); err == nil {
			ann.AnnouncementTime = t.Unix() * 1000
		}
		result.Announcements = append(result.Announcements, ann)
	}
	if result.Announcements == nil {
		result.Announcements = []Announcement{}
	}
	return result
}

// stripJSONP 剥离 JSONP 回调函数包裹
// 输入: jsonpCallbackXXXX({...})
// 输出: {...}
func stripJSONP(data []byte) []byte {
	re := regexp.MustCompile(`^\w+\(`)
	loc := re.FindIndex(data)
	if loc == nil {
		return data // 不是 JSONP 格式，原样返回
	}
	// 去除开头的 callback( 和末尾的 )
	start := loc[1]
	end := len(data) - 1
	for end > start && (data[end] == ')' || data[end] == ';' || data[end] == '\n' || data[end] == '\r' || data[end] == ' ') {
		end--
	}
	return data[start : end+1]
}