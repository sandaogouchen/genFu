package trade_signal

import "genFu/internal/investment"

type DecisionOutput struct {
	DecisionID string         `json:"decision_id"`
	MarketView string         `json:"market_view"`
	RiskNotes  string         `json:"risk_notes"`
	Decisions  []DecisionItem `json:"decisions"`
}

type DecisionItem struct {
	AccountID  int64   `json:"account_id"`
	Symbol     string  `json:"symbol"`
	Name       string  `json:"name"`
	AssetType  string  `json:"asset_type"`
	Action     string  `json:"action"`
	Quantity   float64 `json:"quantity"`
	Price      float64 `json:"price"`
	Confidence float64 `json:"confidence"`
	ValidUntil string  `json:"valid_until"`
	Reason     string  `json:"reason"`
}

type TradeSignal struct {
	AccountID  int64   `json:"account_id"`
	Symbol     string  `json:"symbol"`
	Name       string  `json:"name"`
	AssetType  string  `json:"asset_type"`
	Action     string  `json:"action"`
	Quantity   float64 `json:"quantity"`
	Price      float64 `json:"price"`
	Confidence float64 `json:"confidence"`
	ValidUntil string  `json:"valid_until"`
	Reason     string  `json:"reason"`
	DecisionID string  `json:"decision_id"`
}

type ExecutionResult struct {
	Signal   TradeSignal          `json:"signal"`
	Trade    *investment.Trade    `json:"trade,omitempty"`
	Position *investment.Position `json:"position,omitempty"`
	Error    string               `json:"error,omitempty"`
	Status   string               `json:"status"`
}
