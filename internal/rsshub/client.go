package rsshub

import (
	"encoding/xml"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"
)

type Item struct {
	Title       string     `json:"title"`
	Link        string     `json:"link"`
	GUID        string     `json:"guid"`
	PublishedAt *time.Time `json:"published_at"`
	Description string     `json:"description"`
}

type Client struct {
	baseURL string
	client  *http.Client
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	if strings.TrimSpace(baseURL) == "" {
		baseURL = "https://rsshub.app"
	}
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		client:  &http.Client{},
	}
}

func (c *Client) Fetch(route string, limit int) ([]Item, error) {
	if c == nil {
		return nil, errors.New("rsshub_client_not_initialized")
	}
	route = strings.TrimSpace(route)
	if route == "" {
		return nil, errors.New("empty_route")
	}
	url := c.baseURL + "/" + strings.TrimLeft(route, "/")
	resp, err := c.client.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		return nil, errors.New("rsshub_request_failed")
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	items, err := parseRSS(body)
	if err != nil {
		return nil, err
	}
	if limit > 0 && len(items) > limit {
		items = items[:limit]
	}
	return items, nil
}

type rssFeed struct {
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Items []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	GUID        string `xml:"guid"`
	PubDate     string `xml:"pubDate"`
	Description string `xml:"description"`
}

func parseRSS(data []byte) ([]Item, error) {
	var feed rssFeed
	if err := xml.Unmarshal(data, &feed); err != nil {
		return nil, err
	}
	results := make([]Item, 0, len(feed.Channel.Items))
	for _, item := range feed.Channel.Items {
		parsed := Item{
			Title:       strings.TrimSpace(item.Title),
			Link:        strings.TrimSpace(item.Link),
			GUID:        strings.TrimSpace(item.GUID),
			Description: strings.TrimSpace(item.Description),
		}
		if parsed.GUID == "" {
			parsed.GUID = parsed.Link
		}
		if t, ok := parseTime(item.PubDate); ok {
			parsed.PublishedAt = &t
		}
		results = append(results, parsed)
	}
	return results, nil
}

func parseTime(value string) (time.Time, bool) {
	value = strings.TrimSpace(value)
	if value == "" {
		return time.Time{}, false
	}
	layouts := []string{
		time.RFC1123Z,
		time.RFC1123,
		time.RFC822Z,
		time.RFC822,
		time.RFC3339,
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, value); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}
