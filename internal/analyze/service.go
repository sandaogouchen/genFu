package analyze

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sandaogouchen/genFu/internal/agent/technical"
	"github.com/sandaogouchen/genFu/internal/tool"
)

// Agent 接口（与 internal/agent 包的接口一致）
type Agent interface {
	Run(ctx context.Context, prompt string) (string, error)
}

// Service 分析服务
type Service struct {
	agent     Agent
	technical *technical.TechnicalAgent // 可选，为 nil 时跳过技术分析
	registry  *tool.Registry
}

// NewService 创建分析服务（向后兼容：不含 technical Agent）
func NewService(a Agent) *Service {
	return &Service{
		agent:    a,
		registry: tool.GetRegistry(),
	}
}

// NewServiceWithTechnical 创建包含技术分析 Agent 的分析服务
func NewServiceWithTechnical(a Agent, ta *technical.TechnicalAgent) *Service {
	return &Service{
		agent:     a,
		technical: ta,
		registry:  tool.GetRegistry(),
	}
}

// Analyze 执行分析。
// 改进流程：
// 1. 自动获取 K 线并计算技术指标（EnrichWithIndicators）
// 2. 如果有 technical Agent，执行技术分析
// 3. 将指标数据和技术分析结果注入 prompt
// 4. 调用主 Agent 进行综合分析
func (s *Service) Analyze(ctx context.Context, req AnalyzeRequest) (*AnalyzeResponse, error) {
	if req.Symbol == "" {
		return nil, fmt.Errorf("symbol is required")
	}
	if req.Period == "" {
		req.Period = "1d"
	}

	// Step 1: 自动计算技术指标（非阻断）
	EnrichWithIndicators(&req, s.registry)

	// Step 2: 技术分析 Agent（非阻断）
	var techAnalysis string
	if s.technical != nil && req.Meta["indicators"] != "" {
		result, err := s.technical.Run(ctx, req.Meta["indicators"])
		if err != nil {
			// 技术分析失败不阻断
			if req.Meta == nil {
				req.Meta = make(map[string]string)
			}
			req.Meta["technical_error"] = err.Error()
		} else {
			techAnalysis = result
		}
	}

	// Step 3: 构建增强后的分析 prompt
	prompt := buildAnalyzePrompt(req, techAnalysis)

	// Step 4: 调用主 Agent
	result, err := s.agent.Run(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("agent run failed: %w", err)
	}

	return &AnalyzeResponse{
		Symbol:            req.Symbol,
		Period:            req.Period,
		Analysis:          result,
		TechnicalAnalysis: techAnalysis,
	}, nil
}

// buildAnalyzePrompt 构建分析 prompt，注入技术指标数据
func buildAnalyzePrompt(req AnalyzeRequest, techAnalysis string) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("请分析 %s 的行情数据", req.Symbol))

	if req.Period != "" {
		sb.WriteString(fmt.Sprintf("，K线周期为 %s", req.Period))
	}

	if req.Indicators != nil && len(req.Indicators) > 0 {
		sb.WriteString("，需要计算以下技术指标：")
		for i, ind := range req.Indicators {
			if i > 0 {
				sb.WriteString("、")
			}
			sb.WriteString(ind)
		}
	}

	sb.WriteString("。请给出详细的技术分析结果。")

	// 注入技术指标最新快照
	if latest := req.Meta["indicators_latest"]; latest != "" {
		sb.WriteString("\n\n## 技术指标快照（已计算）\n")
		// 格式化 JSON
		var latestMap map[string]interface{}
		if json.Unmarshal([]byte(latest), &latestMap) == nil {
			formatted, _ := json.MarshalIndent(latestMap, "", "  ")
			sb.WriteString("```json\n")
			sb.Write(formatted)
			sb.WriteString("\n```\n")
		}
	}

	// 注入信号事件
	if signals := req.Meta["indicators_signals"]; signals != "" {
		sb.WriteString("\n## 近期信号事件\n")
		sb.WriteString("```json\n")
		sb.WriteString(signals)
		sb.WriteString("\n```\n")
	}

	// 注入技术分析结果
	if techAnalysis != "" {
		sb.WriteString("\n## 技术分析 Agent 研判\n")
		sb.WriteString(techAnalysis)
		sb.WriteString("\n")
	}

	return sb.String()
}

// GetAvailableTools 返回当前可用的工具列表
func (s *Service) GetAvailableTools() []tool.ToolInfo {
	return s.registry.ListTools()
}
