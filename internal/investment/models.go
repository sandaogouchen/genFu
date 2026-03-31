package investment

import "time"

type User struct {
	ID        int64     `json:"id"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type Account struct {
	ID           int64     `json:"id"`
	UserID       int64     `json:"user_id"`
	Name         string    `json:"name"`
	BaseCurrency string    `json:"base_currency"`
	CreatedAt    time.Time `json:"created_at"`
}

type Instrument struct {
	ID          int64     `json:"id"`
	Symbol      string    `json:"symbol"`
	Name        string    `json:"name"`
	AssetType   string    `json:"asset_type"`
	Industry    string    `json:"industry,omitempty"`
	Products    []string  `json:"products,omitempty"`
	Competitors []string  `json:"competitors,omitempty"`
	SupplyChain []string  `json:"supply_chain,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
}

type Position struct {
	ID               int64      `json:"id"`
	AccountID        int64      `json:"account_id"`
	Instrument       Instrument `json:"instrument"`
	Quantity         float64    `json:"quantity"`
	AvgCost          float64    `json:"avg_cost"`
	MarketPrice      *float64   `json:"market_price,omitempty"`
	OperationGuideID *int64     `json:"operation_guide_id,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
}

type Trade struct {
	ID         int64      `json:"id"`
	AccountID  int64      `json:"account_id"`
	Instrument Instrument `json:"instrument"`
	Side       string     `json:"side"`
	Quantity   float64    `json:"quantity"`
	Price      float64    `json:"price"`
	Fee        float64    `json:"fee"`
	TradeAt    time.Time  `json:"trade_at"`
	Note       string     `json:"note"`
}

type CashFlow struct {
	ID        int64     `json:"id"`
	AccountID int64     `json:"account_id"`
	Amount    float64   `json:"amount"`
	Currency  string    `json:"currency"`
	FlowType  string    `json:"flow_type"`
	FlowAt    time.Time `json:"flow_at"`
	Note      string    `json:"note"`
}

type Valuation struct {
	ID          int64     `json:"id"`
	AccountID   int64     `json:"account_id"`
	TotalValue  float64   `json:"total_value"`
	TotalCost   float64   `json:"total_cost"`
	TotalPnL    float64   `json:"total_pnl"`
	ValuationAt time.Time `json:"valuation_at"`
}

type PortfolioSummary struct {
	AccountID     int64      `json:"account_id"`
	PositionCount int64      `json:"position_count"`
	TradeCount    int64      `json:"trade_count"`
	TotalValue    float64    `json:"total_value"`
	TotalCost     float64    `json:"total_cost"`
	TotalPnL      float64    `json:"total_pnl"`
	ValuationAt   *time.Time `json:"valuation_at,omitempty"`
}

type PositionPnL struct {
	AccountID    int64   `json:"account_id"`
	InstrumentID int64   `json:"instrument_id"`
	Symbol       string  `json:"symbol"`
	Name         string  `json:"name"`
	Quantity     float64 `json:"quantity"`
	AvgCost      float64 `json:"avg_cost"`
	MarketPrice  float64 `json:"market_price"`
	Cost         float64 `json:"cost"`
	Value        float64 `json:"value"`
	PnL          float64 `json:"pnl"`
	PnLPct       float64 `json:"pnl_pct"`
}

type AccountPnL struct {
	AccountID  int64         `json:"account_id"`
	Positions  []PositionPnL `json:"positions"`
	TotalCost  float64       `json:"total_cost"`
	TotalValue float64       `json:"total_value"`
	TotalPnL   float64       `json:"total_pnl"`
}
