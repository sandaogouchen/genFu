package analyze

import "genFu/internal/tool"

type AnalyzeRequest struct {
	Symbol       string            `json:"symbol"`
	Name         string            `json:"name,omitempty"`
	Type         string            `json:"type"`
	Kline        string            `json:"kline,omitempty"`
	Manager      string            `json:"manager,omitempty"`
	Meta         map[string]string `json:"meta,omitempty"`
	SessionID    string            `json:"session_id,omitempty"`
	SessionTitle string            `json:"session_title,omitempty"`
	Prompt       string            `json:"prompt,omitempty"`
}

type AnalyzeStep struct {
	Name        string            `json:"name"`
	Input       string            `json:"input"`
	Output      string            `json:"output"`
	ToolResults []tool.ToolResult `json:"tool_results,omitempty"`
}

type AnalyzeResponse struct {
	Type     string        `json:"type"`
	Symbol   string        `json:"symbol"`
	Name     string        `json:"name,omitempty"`
	Steps    []AnalyzeStep `json:"steps"`
	Summary  string        `json:"summary"`
	ReportID int64         `json:"report_id,omitempty"`
}
