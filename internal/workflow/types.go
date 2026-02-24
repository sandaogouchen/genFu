package workflow

import "genFu/internal/rsshub"

type StockWorkflowInput struct {
	AccountID           int64    `json:"account_id"`
	Symbol              string   `json:"symbol"`
	Name                string   `json:"name"`
	StockNewsRoutes      []string `json:"stock_news_routes"`
	IndustryNewsRoutes   []string `json:"industry_news_routes"`
	NewsLimit            int      `json:"news_limit"`
}

type HoldingPosition struct {
	Symbol      string  `json:"symbol"`
	Name        string  `json:"name"`
	Quantity    float64 `json:"quantity"`
	AvgCost     float64 `json:"avg_cost"`
	MarketPrice float64 `json:"market_price"`
	Value       float64 `json:"value"`
	Ratio       float64 `json:"ratio"`
}

type HoldingsOutput struct {
	Positions  []HoldingPosition `json:"positions"`
	TotalValue float64           `json:"total_value"`
}

type MarketMove struct {
	Symbol     string  `json:"symbol"`
	Name       string  `json:"name"`
	Price      float64 `json:"price"`
	Change     float64 `json:"change"`
	ChangeRate float64 `json:"change_rate"`
	Error      string  `json:"error,omitempty"`
}

type NewsSummaryOutput struct {
	Items     []rsshub.Item `json:"items"`
	Summary   string        `json:"summary"`
	Sentiment string        `json:"sentiment"`
}

type StockWorkflowOutput struct {
	Holdings         HoldingsOutput   `json:"holdings"`
	HoldingsMarket   []MarketMove     `json:"holdings_market"`
	TargetMarket     MarketMove       `json:"target_market"`
	News             NewsSummaryOutput `json:"news"`
	BullAnalysis     string           `json:"bull_analysis"`
	BearAnalysis     string           `json:"bear_analysis"`
	DebateAnalysis   string           `json:"debate_analysis"`
	Summary          string           `json:"summary"`
}
