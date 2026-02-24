package decision

import (
	"genFu/internal/tool"
	"genFu/internal/trade_signal"
)

type DecisionRequest struct {
	AccountID int64             `json:"account_id,omitempty"`
	ReportIDs []int64           `json:"report_ids,omitempty"`
	Meta      map[string]string `json:"meta,omitempty"`
}

type DecisionResponse struct {
	Decision    trade_signal.DecisionOutput    `json:"decision"`
	Raw         string                         `json:"raw"`
	Signals     []trade_signal.TradeSignal     `json:"signals"`
	Executions  []trade_signal.ExecutionResult `json:"executions"`
	ToolResults []tool.ToolResult              `json:"tool_results,omitempty"`
	ReportID    int64                          `json:"report_id,omitempty"`
}
