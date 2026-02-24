package financial

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	cninfoBaseURL    = "https://www.cninfo.com.cn"
	cninfoQueryURL   = "https://www.cninfo.com.cn/new/hisAnnouncement/query"
	cninfoPDFBaseURL = "https://static.cninfo.com.cn/"
)

type CNInfoClient struct {
	client *http.Client
}

func NewCNInfoClient() *CNInfoClient {
	return &CNInfoClient{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// QueryAnnouncements 查询公告列表
func (c *CNInfoClient) QueryAnnouncements(ctx context.Context, symbol string, pageNum, pageSize int) ([]Announcement, error) {
	// 构建请求参数
	data := url.Values{}
	data.Set("pageNum", fmt.Sprintf("%d", pageNum))
	data.Set("pageSize", fmt.Sprintf("%d", pageSize))
	data.Set("column", "szse")
	data.Set("tabName", "fulltext")
	data.Set("plate", "")
	data.Set("stock", symbol)
	data.Set("searchkey", "")
	data.Set("secid", "")
	data.Set("category", "category_ndbg_szsh") // 年度报告类别
	data.Set("seDate", c.buildDateRange())
	data.Set("sortName", "")
	data.Set("sortType", "")
	data.Set("isHLtitle", "true")

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, cninfoQueryURL, strings.NewReader(data.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Referer", cninfoBaseURL)

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result CNInfoQueryResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	return result.Announcements, nil
}

// DownloadPDF 下载PDF文件
func (c *CNInfoClient) DownloadPDF(ctx context.Context, relativeURL string) ([]byte, error) {
	// 确保URL格式正确
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
	if strings.HasPrefix(announcement.PDFURL, "http") {
		return announcement.PDFURL
	}
	return cninfoPDFBaseURL + announcement.PDFURL
}

func (c *CNInfoClient) buildDateRange() string {
	now := time.Now()
	from := now.AddDate(-2, 0, 0) // 最近两年
	return fmt.Sprintf("%s ~ %s", from.Format("2006-01-02"), now.Format("2006-01-02"))
}
