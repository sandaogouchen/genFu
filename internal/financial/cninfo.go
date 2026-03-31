package financial

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const (
	cninfoBaseURL    = "https://www.cninfo.com.cn"
	cninfoQueryURL   = "https://www.cninfo.com.cn/new/hisAnnouncement/query"
	cninfoPDFBaseURL = "https://static.cninfo.com.cn/"

	// 默认请求间隔（速率控制）
	cninfoMinInterval = 500 * time.Millisecond
)

type CNInfoClient struct {
	client      *http.Client
	mu          sync.Mutex
	lastRequest time.Time
}

func NewCNInfoClient() *CNInfoClient {
	return &CNInfoClient{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// QueryAnnouncements 查询公告列表（全参数版本）
func (c *CNInfoClient) QueryAnnouncements(ctx context.Context, query AnnouncementQuery) (*AnnouncementResult, error) {
	query.Defaults()

	// 速率控制
	c.rateLimit()

	// 解析分类码（支持中文别名）
	resolvedCategory := ResolveCategoryCode(query.Category)

	// 构建请求参数
	data := url.Values{}
	data.Set("pageNum", fmt.Sprintf("%d", query.PageNum))
	data.Set("pageSize", fmt.Sprintf("%d", query.PageSize))
	data.Set("tabName", "fulltext")
	data.Set("plate", "")
	data.Set("secid", "")

	// stock 参数
	if query.Symbol != "" {
		data.Set("stock", query.Symbol)
	} else {
		data.Set("stock", "")
	}

	// searchkey 参数——全文关键词
	data.Set("searchkey", query.SearchKey)

	// category 参数
	if resolvedCategory != "" {
		data.Set("category", resolvedCategory)
	} else {
		data.Set("category", "")
	}

	// column 参数——交易所
	col := resolveColumn(query.Column)
	data.Set("column", col)

	// seDate 参数——日期范围
	data.Set("seDate", fmt.Sprintf("%s~%s", query.StartDate, query.EndDate))

	// 排序
	data.Set("sortName", query.SortName)
	data.Set("sortType", query.SortType)

	// 高亮
	if query.Highlight {
		data.Set("isHLtitle", "true")
	} else {
		data.Set("isHLtitle", "false")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cninfoQueryURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", cninfoBaseURL)
	req.Header.Set("Accept", "application/json")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cninfo request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("cninfo returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result CNInfoQueryResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return result.ToResult(query.PageNum), nil
}

// QueryAnnouncementsLegacy 向后兼容的旧接口签名
// 行为与改造前完全一致：仅查深交所年报，回溯 2 年
func (c *CNInfoClient) QueryAnnouncementsLegacy(ctx context.Context, symbol string, pageNum, pageSize int) ([]Announcement, error) {
	now := time.Now()
	query := AnnouncementQuery{
		Symbol:    symbol,
		Category:  "category_ndbg_szsh",
		Column:    "szse",
		StartDate: now.AddDate(-2, 0, 0).Format("2006-01-02"),
		EndDate:   now.Format("2006-01-02"),
		PageNum:   pageNum,
		PageSize:  pageSize,
		Highlight: true,
	}
	result, err := c.QueryAnnouncements(ctx, query)
	if err != nil {
		return nil, err
	}
	return result.Announcements, nil
}

// DownloadPDF 下载PDF文件
func (c *CNInfoClient) DownloadPDF(ctx context.Context, relativeURL string) ([]byte, error) {
	fullURL := relativeURL
	if !strings.HasPrefix(relativeURL, "http") {
		fullURL = cninfoPDFBaseURL + relativeURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fullURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download failed with status: %d", resp.StatusCode)
	}

	return io.ReadAll(resp.Body)
}

// GetPDFDownloadURL 获取PDF下载URL
func (c *CNInfoClient) GetPDFDownloadURL(announcement *Announcement) string {
	if announcement == nil || announcement.PDFURL == "" {
		return ""
	}
	return announcement.FullPDFURL()
}

// rateLimit 线程安全的速率控制
func (c *CNInfoClient) rateLimit() {
	c.mu.Lock()
	defer c.mu.Unlock()
	elapsed := time.Since(c.lastRequest)
	if elapsed < cninfoMinInterval {
		time.Sleep(cninfoMinInterval - elapsed)
	}
	c.lastRequest = time.Now()
}

// resolveColumn 将交易所参数标准化
func resolveColumn(col string) string {
	switch strings.ToLower(strings.TrimSpace(col)) {
	case "sse":
		return "sse"
	case "szse":
		return "szse"
	case "bse":
		return "bse"
	case "all", "":
		return "" // 空 = 全部交易所
	default:
		return col
	}
}