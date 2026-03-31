package financial

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

const (
	szseAnnListURL  = "http://www.szse.cn/api/disc/announcement/annList"
	szseMinInterval = 1 * time.Second
)

// SZSEClient 深圳证券交易所公告查询客户端
type SZSEClient struct {
	client      *http.Client
	mu          sync.Mutex
	lastRequest time.Time
}

func NewSZSEClient() *SZSEClient {
	return &SZSEClient{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

// QueryAnnouncements 查询深交所公告
func (c *SZSEClient) QueryAnnouncements(ctx context.Context, query ExchangeQuery) (*AnnouncementResult, error) {
	query.Defaults()
	c.rateLimit()

	reqBody := szseRequest{
		SeDate:      []string{query.StartDate, query.EndDate},
		ChannelCode: []string{"listedNotice_disc"},
		PageSize:    query.PageSize,
		PageNum:     query.PageNum,
	}

	if query.Symbol != "" {
		reqBody.Stock = []string{query.Symbol}
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal SZSE request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, szseAnnListURL, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("build SZSE request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("SZSE request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("SZSE returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read SZSE response: %w", err)
	}

	var szseResp szseResponse
	if err := json.Unmarshal(body, &szseResp); err != nil {
		return nil, fmt.Errorf("parse SZSE response: %w", err)
	}

	return szseResp.toAnnouncementResult(query.PageNum, query.PageSize), nil
}

// rateLimit 线程安全的速率控制
func (c *SZSEClient) rateLimit() {
	c.mu.Lock()
	defer c.mu.Unlock()
	elapsed := time.Since(c.lastRequest)
	if elapsed < szseMinInterval {
		time.Sleep(szseMinInterval - elapsed)
	}
	c.lastRequest = time.Now()
}

// SZSE 请求结构
type szseRequest struct {
	SeDate      []string `json:"seDate"`
	Stock       []string `json:"stock,omitempty"`
	ChannelCode []string `json:"channelCode"`
	PageSize    int      `json:"pageSize"`
	PageNum     int      `json:"pageNum"`
}

// SZSE 响应结构
type szseResponse struct {
	Data           []szseAnnouncement `json:"data"`
	Announce_Count int                `json:"announceCount"`
}

type szseAnnouncement struct {
	ID          string `json:"id"`
	Title       string `json:"title"`
	PublishDate string `json:"publishDate"` // YYYY-MM-DD HH:MM
	SecCode     string `json:"secCode"`
	SecName     string `json:"secName"`
	AttachPath  string `json:"attachPath"` // PDF 路径
	AttachSize  int    `json:"attachSize"`
}

func (r *szseResponse) toAnnouncementResult(currentPage, pageSize int) *AnnouncementResult {
	totalRecords := r.Announce_Count
	if pageSize <= 0 {
		pageSize = 30
	}
	totalPages := totalRecords / pageSize
	if totalRecords%pageSize > 0 {
		totalPages++
	}

	result := &AnnouncementResult{
		TotalRecords: totalRecords,
		TotalPages:   totalPages,
		CurrentPage:  currentPage,
		HasMore:      currentPage < totalPages,
	}

	for _, a := range r.Data {
		ann := Announcement{
			ID:      a.ID,
			SecCode: a.SecCode,
			SecName: a.SecName,
			Title:   a.Title,
			Column:  "szse",
			PDFURL:  a.AttachPath,
			PDFSize: a.AttachSize,
		}
		// 尝试解析日期
		if t, err := time.Parse("2006-01-02 15:04", a.PublishDate); err == nil {
			ann.AnnouncementTime = t.Unix() * 1000
		} else if t, err := time.Parse("2006-01-02", a.PublishDate); err == nil {
			ann.AnnouncementTime = t.Unix() * 1000
		}
		result.Announcements = append(result.Announcements, ann)
	}
	if result.Announcements == nil {
		result.Announcements = []Announcement{}
	}
	return result
}