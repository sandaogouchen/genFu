package decision

import (
	"context"

	"genFu/internal/tool"
	"genFu/internal/trade_signal"
)

type DecisionRequest struct {
	AccountID       int64             `json:"account_id,omitempty"`
	ReportIDs       []int64           `json:"report_ids,omitempty"`
	GuideSelections []GuideSelection  `json:"guide_selections,omitempty"`
	Meta            map[string]string `json:"meta,omitempty"`
	SessionID       string            `json:"session_id,omitempty"`
	SessionTitle    string            `json:"session_title,omitempty"`
	Prompt          string            `json:"prompt,omitempty"`
}

type GuideSelection struct {
	Symbol  string `json:"symbol"`
	GuideID int64  `json:"guide_id"`
}

type RiskBudget struct {
	MaxSingleOrderRatio    float64 `json:"max_single_order_ratio"`
	MaxSymbolExposureRatio float64 `json:"max_symbol_exposure_ratio"`
	MaxDailyTradeRatio     float64 `json:"max_daily_trade_ratio"`
	MinConfidence          float64 `json:"min_confidence"`
}

func DefaultRiskBudget() RiskBudget {
	return RiskBudget{
		MaxSingleOrderRatio:    0.20,
		MaxSymbolExposureRatio: 0.30,
		MaxDailyTradeRatio:     0.40,
		MinConfidence:          0.55,
	}
}

func (r RiskBudget) Normalize() RiskBudget {
	out := r
	def := DefaultRiskBudget()
	if out.MaxSingleOrderRatio <= 0 || out.MaxSingleOrderRatio > 1 {
		out.MaxSingleOrderRatio = def.MaxSingleOrderRatio
	}
	if out.MaxSymbolExposureRatio <= 0 || out.MaxSymbolExposureRatio > 1 {
		out.MaxSymbolExposureRatio = def.MaxSymbolExposureRatio
	}
	if out.MaxDailyTradeRatio <= 0 || out.MaxDailyTradeRatio > 1 {
		out.MaxDailyTradeRatio = def.MaxDailyTradeRatio
	}
	if out.MinConfidence <= 0 || out.MinConfidence > 1 {
		out.MinConfidence = def.MinConfidence
	}
	return out
}

type PlannedOrder struct {
	OrderID        string  `json:"order_id"`
	AccountID      int64   `json:"account_id"`
	Symbol         string  `json:"symbol"`
	Name           string  `json:"name"`
	AssetType      string  `json:"asset_type"`
	Action         string  `json:"action"`
	Quantity       float64 `json:"quantity"`
	Price          float64 `json:"price"`
	Notional       float64 `json:"notional"`
	Confidence     float64 `json:"confidence"`
	DecisionID     string  `json:"decision_id"`
	PlanningReason string  `json:"planning_reason"`
}

type GuardedOrder struct {
	PlannedOrder
	GuardStatus     string `json:"guard_status"`
	GuardReason     string `json:"guard_reason,omitempty"`
	ExecutionStatus string `json:"execution_status,omitempty"`
	ExecutionError  string `json:"execution_error,omitempty"`
	TradeID         int64  `json:"trade_id,omitempty"`
}

type ReviewAttribution struct {
	OrderID string `json:"order_id,omitempty"`
	Title   string `json:"title,omitempty"`
	Detail  string `json:"detail,omitempty"`
}

type PostTradeReview struct {
	Summary        string              `json:"summary"`
	Attributions   []ReviewAttribution `json:"attributions,omitempty"`
	LearningPoints []string            `json:"learning_points,omitempty"`
}

type DecisionResponse struct {
	Decision      trade_signal.DecisionOutput    `json:"decision"`
	Raw           string                         `json:"raw"`
	Signals       []trade_signal.TradeSignal     `json:"signals"`
	Executions    []trade_signal.ExecutionResult `json:"executions"`
	ToolResults   []tool.ToolResult              `json:"tool_results,omitempty"`
	ReportID      int64                          `json:"report_id,omitempty"`
	RunID         string                         `json:"run_id,omitempty"`
	RiskBudget    RiskBudget                     `json:"risk_budget,omitempty"`
	PlannedOrders []PlannedOrder                 `json:"planned_orders,omitempty"`
	GuardedOrders []GuardedOrder                 `json:"guarded_orders,omitempty"`
	Review        *PostTradeReview               `json:"review,omitempty"`
	Warnings      []string                       `json:"warnings,omitempty"`
}

type GuardRequest struct {
	AccountID     int64
	RiskBudget    RiskBudget
	PlannedOrders []PlannedOrder
}

type PolicyGuard interface {
	Guard(ctx context.Context, req GuardRequest) ([]GuardedOrder, error)
}
