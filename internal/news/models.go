package news

import "time"

type NewsItem struct {
	ID          int64
	Source      string
	Title       string
	Link        string
	GUID        string
	PublishedAt *time.Time
	Content     string
	FetchedAt   time.Time
}

type NewsBrief struct {
	ID        int64
	ItemID    int64
	Sentiment string
	Brief     string
	Keywords  []string
	CreatedAt time.Time
}

type BriefView struct {
	ItemID      int64    `json:"item_id"`
	Title       string   `json:"title"`
	Link        string   `json:"link"`
	PublishedAt *time.Time `json:"published_at"`
	Sentiment   string   `json:"sentiment"`
	Brief       string   `json:"brief"`
	Keywords    []string `json:"keywords"`
	CreatedAt   time.Time `json:"created_at"`
}

type RSSItem struct {
	Title       string
	Link        string
	GUID        string
	PublishedAt *time.Time
	Description string
}
