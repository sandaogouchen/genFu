package stockpicker

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"genFu/internal/tool"
)

const (
	strategySmallCapQuality    = "small_cap_quality"
	strategyTechnicalBreak     = "technical_breakout"
	strategyMomentumStrong     = "momentum_strong"
	strategyOversoldBounce     = "oversold_bounce"
	strategyMACrossoverTrend   = "ma_crossover_trend"
	strategyMACDSignalFollow   = "macd_signal_follow"
	strategyVolumeBreakout     = "volume_breakout"
	strategyPullbackInUptrend  = "pullback_in_uptrend"
	strategyTrendFollowingCore = "trend_following_core"
	strategyDefensiveConsol    = "defensive_consolidation"

	toolStrategySmallCapQuality    = "stock_strategy_small_cap_quality"
	toolStrategyTechnicalBreak     = "stock_strategy_technical_breakout"
	toolStrategyMomentumStrong     = "stock_strategy_momentum_strong"
	toolStrategyOversoldBounce     = "stock_strategy_oversold_bounce"
	toolStrategyMACrossoverTrend   = "stock_strategy_ma_crossover_trend"
	toolStrategyMACDSignalFollow   = "stock_strategy_macd_signal_follow"
	toolStrategyVolumeBreakout     = "stock_strategy_volume_breakout"
	toolStrategyPullbackInUptrend  = "stock_strategy_pullback_in_uptrend"
	toolStrategyTrendFollowingCore = "stock_strategy_trend_following_core"
	toolStrategyDefensiveConsol    = "stock_strategy_defensive_consolidation"
)

type strategyContext struct {
	UpCount         int
	DownCount       int
	LimitUp         int
	LimitDown       int
	UpRatio         float64
	MarketSentiment string
}

type strategyMeta struct {
	Name        string
	ToolName    string
	Description string
	Builder     func(ctx strategyContext) map[string]interface{}
	RiskNotes   func(ctx strategyContext) string
}

var strategyOrder = []string{
	strategySmallCapQuality,
	strategyTechnicalBreak,
	strategyMomentumStrong,
	strategyOversoldBounce,
	strategyMACrossoverTrend,
	strategyMACDSignalFollow,
	strategyVolumeBreakout,
	strategyPullbackInUptrend,
	strategyTrendFollowingCore,
	strategyDefensiveConsol,
}

var strategyMetaMap = map[string]strategyMeta{
	strategySmallCapQuality: {
		Name:        strategySmallCapQuality,
		ToolName:    toolStrategySmallCapQuality,
		Description: "小盘均衡策略（规模+流动性）",
		Builder: func(ctx strategyContext) map[string]interface{} {
			changeMax := 5.0
			if ctx.UpRatio < 0.4 {
				changeMax = 4.0
			}
			return map[string]interface{}{
				"amount_min":      5e7,
				"amount_max":      5e8,
				"change_rate_min": -3.0,
				"change_rate_max": changeMax,
				"ma5_above_ma20":  true,
			}
		},
		RiskNotes: func(ctx strategyContext) string {
			return "关注中小盘流动性分化，避免追高。"
		},
	},
	strategyTechnicalBreak: {
		Name:        strategyTechnicalBreak,
		ToolName:    toolStrategyTechnicalBreak,
		Description: "技术面突破策略（Breakout）",
		Builder: func(ctx strategyContext) map[string]interface{} {
			changeMin := 2.0
			if ctx.UpRatio > 0.65 {
				changeMin = 2.5
			}
			return map[string]interface{}{
				"amount_min":      1e8,
				"change_rate_min": changeMin,
				"change_rate_max": 9.0,
				"macd_golden":     true,
				"volume_spike":    true,
			}
		},
		RiskNotes: func(ctx strategyContext) string {
			return "突破策略需确认量能延续，防止假突破。"
		},
	},
	strategyMomentumStrong: {
		Name:        strategyMomentumStrong,
		ToolName:    toolStrategyMomentumStrong,
		Description: "强势动量策略（Momentum）",
		Builder: func(ctx strategyContext) map[string]interface{} {
			changeMin := 3.0
			if ctx.UpRatio > 0.8 {
				changeMin = 4.0
			}
			return map[string]interface{}{
				"amount_min":      2e8,
				"change_rate_min": changeMin,
				"change_rate_max": 9.0,
				"ma20_rising":     true,
			}
		},
		RiskNotes: func(ctx strategyContext) string {
			if ctx.UpRatio > 0.8 {
				return "情绪过热阶段严格止盈，防范冲高回落。"
			}
			return "动量衰减时需快速减仓。"
		},
	},
	strategyOversoldBounce: {
		Name:        strategyOversoldBounce,
		ToolName:    toolStrategyOversoldBounce,
		Description: "超跌反弹策略（Mean Reversion）",
		Builder: func(ctx strategyContext) map[string]interface{} {
			changeMax := 4.0
			if ctx.UpRatio < 0.2 {
				changeMax = 3.0
			}
			return map[string]interface{}{
				"amount_min":      5e7,
				"change_rate_min": -6.0,
				"change_rate_max": changeMax,
				"rsi_oversold":    true,
			}
		},
		RiskNotes: func(ctx strategyContext) string {
			return "仅适合轻仓反弹博弈，止损必须前置。"
		},
	},
	strategyMACrossoverTrend: {
		Name:        strategyMACrossoverTrend,
		ToolName:    toolStrategyMACrossoverTrend,
		Description: "均线金叉趋势策略（MA Crossover）",
		Builder: func(ctx strategyContext) map[string]interface{} {
			return map[string]interface{}{
				"amount_min":      8e7,
				"change_rate_min": -1.0,
				"change_rate_max": 6.0,
				"ma5_above_ma20":  true,
				"ma20_rising":     true,
			}
		},
		RiskNotes: func(ctx strategyContext) string {
			return "均线策略对震荡行情敏感，需结合成交量过滤。"
		},
	},
	strategyMACDSignalFollow: {
		Name:        strategyMACDSignalFollow,
		ToolName:    toolStrategyMACDSignalFollow,
		Description: "MACD信号跟随策略（MACD Crossover）",
		Builder: func(ctx strategyContext) map[string]interface{} {
			return map[string]interface{}{
				"amount_min":      1.2e8,
				"change_rate_min": 0.0,
				"change_rate_max": 8.0,
				"macd_golden":     true,
				"ma20_rising":     true,
			}
		},
		RiskNotes: func(ctx strategyContext) string {
			return "MACD滞后性较强，需配合止损与仓位控制。"
		},
	},
	strategyVolumeBreakout: {
		Name:        strategyVolumeBreakout,
		ToolName:    toolStrategyVolumeBreakout,
		Description: "量价突破策略（Volume Breakout）",
		Builder: func(ctx strategyContext) map[string]interface{} {
			return map[string]interface{}{
				"amount_min":      1.5e8,
				"change_rate_min": 1.5,
				"change_rate_max": 9.0,
				"volume_spike":    true,
				"ma5_above_ma20":  true,
			}
		},
		RiskNotes: func(ctx strategyContext) string {
			return "放量突破后若缩量回落，需警惕失败形态。"
		},
	},
	strategyPullbackInUptrend: {
		Name:        strategyPullbackInUptrend,
		ToolName:    toolStrategyPullbackInUptrend,
		Description: "上升趋势回踩策略（Pullback）",
		Builder: func(ctx strategyContext) map[string]interface{} {
			return map[string]interface{}{
				"amount_min":      8e7,
				"change_rate_min": -2.0,
				"change_rate_max": 3.0,
				"ma20_rising":     true,
			}
		},
		RiskNotes: func(ctx strategyContext) string {
			return "回踩失败会快速转弱，建议设置硬止损。"
		},
	},
	strategyTrendFollowingCore: {
		Name:        strategyTrendFollowingCore,
		ToolName:    toolStrategyTrendFollowingCore,
		Description: "核心趋势跟随策略（Trend Following）",
		Builder: func(ctx strategyContext) map[string]interface{} {
			return map[string]interface{}{
				"amount_min":      1e8,
				"change_rate_min": 0.5,
				"change_rate_max": 7.0,
				"ma20_rising":     true,
				"ma5_above_ma20":  true,
			}
		},
		RiskNotes: func(ctx strategyContext) string {
			return "趋势跟随要接受回撤，避免频繁反向交易。"
		},
	},
	strategyDefensiveConsol: {
		Name:        strategyDefensiveConsol,
		ToolName:    toolStrategyDefensiveConsol,
		Description: "防御性震荡策略（Defensive Range）",
		Builder: func(ctx strategyContext) map[string]interface{} {
			return map[string]interface{}{
				"amount_min":      5e7,
				"amount_max":      3e8,
				"change_rate_min": -2.0,
				"change_rate_max": 2.0,
			}
		},
		RiskNotes: func(ctx strategyContext) string {
			return "防御策略收益弹性较低，适合弱市控回撤。"
		},
	},
}

// StockStrategyRouterTool 根据市场涨跌家数路由到策略工具
type StockStrategyRouterTool struct{}

func NewStockStrategyRouterTool() *StockStrategyRouterTool {
	return &StockStrategyRouterTool{}
}

func (t *StockStrategyRouterTool) Spec() tool.ToolSpec {
	return tool.ToolSpec{
		Name:        "stock_strategy_router",
		Description: "route market breadth data to the best stock strategy tool",
		Params: map[string]string{
			"action":           "string",
			"up_count":         "number",
			"down_count":       "number",
			"limit_up":         "number",
			"limit_down":       "number",
			"market_sentiment": "string",
		},
		Required: []string{"action"},
	}
}

func (t *StockStrategyRouterTool) Execute(ctx context.Context, args map[string]interface{}) (tool.ToolResult, error) {
	_ = ctx
	action, err := requireStringArg(args, "action")
	if err != nil {
		return tool.ToolResult{Name: "stock_strategy_router", Error: err.Error()}, err
	}

	switch strings.ToLower(strings.TrimSpace(action)) {
	case "find_tool":
		return t.route(args)
	case "list_tools":
		return t.listTools(), nil
	default:
		err := errors.New("unsupported_action")
		return tool.ToolResult{Name: "stock_strategy_router", Error: err.Error()}, err
	}
}

func (t *StockStrategyRouterTool) route(args map[string]interface{}) (tool.ToolResult, error) {
	ctx := parseStrategyContext(args)
	meta, marketState, reason := chooseStrategy(ctx)
	if meta.ToolName == "" {
		meta = strategyMetaMap[strategySmallCapQuality]
		marketState = "震荡盘整"
		reason = "缺少有效涨跌家数，使用默认均衡策略"
	}

	return tool.ToolResult{
		Name: "stock_strategy_router",
		Output: map[string]interface{}{
			"strategy_name":        meta.Name,
			"strategy_tool":        meta.ToolName,
			"strategy_description": meta.Description,
			"market_state":         marketState,
			"up_ratio":             ctx.UpRatio,
			"up_count":             ctx.UpCount,
			"down_count":           ctx.DownCount,
			"limit_up":             ctx.LimitUp,
			"limit_down":           ctx.LimitDown,
			"routing_reason":       reason,
		},
	}, nil
}

func (t *StockStrategyRouterTool) listTools() tool.ToolResult {
	strategies := make([]map[string]interface{}, 0, len(strategyOrder))
	for _, name := range strategyOrder {
		meta, ok := strategyMetaMap[name]
		if !ok {
			continue
		}
		strategies = append(strategies, map[string]interface{}{
			"strategy_name":        meta.Name,
			"strategy_tool":        meta.ToolName,
			"strategy_description": meta.Description,
		})
	}
	return tool.ToolResult{
		Name:   "stock_strategy_router",
		Output: map[string]interface{}{"strategies": strategies},
	}
}

// StockStrategyTool 具体策略工具，接收涨跌家数并给出筛选条件
type StockStrategyTool struct {
	meta strategyMeta
}

func NewStockStrategySmallCapQualityTool() *StockStrategyTool {
	return newStockStrategyTool(strategySmallCapQuality)
}

func NewStockStrategyTechnicalBreakoutTool() *StockStrategyTool {
	return newStockStrategyTool(strategyTechnicalBreak)
}

func NewStockStrategyMomentumStrongTool() *StockStrategyTool {
	return newStockStrategyTool(strategyMomentumStrong)
}

func NewStockStrategyOversoldBounceTool() *StockStrategyTool {
	return newStockStrategyTool(strategyOversoldBounce)
}

func newStockStrategyTool(strategyName string) *StockStrategyTool {
	meta, ok := strategyMetaMap[strategyName]
	if !ok {
		return &StockStrategyTool{}
	}
	return &StockStrategyTool{meta: meta}
}

func (t *StockStrategyTool) Spec() tool.ToolSpec {
	return tool.ToolSpec{
		Name:        t.meta.ToolName,
		Description: t.meta.Description,
		Params: map[string]string{
			"up_count":         "number",
			"down_count":       "number",
			"limit_up":         "number",
			"limit_down":       "number",
			"market_sentiment": "string",
			"limit":            "number",
		},
	}
}

func (t *StockStrategyTool) Execute(ctx context.Context, args map[string]interface{}) (tool.ToolResult, error) {
	_ = ctx
	if t.meta.Name == "" || t.meta.Builder == nil {
		err := errors.New("invalid_strategy_tool")
		return tool.ToolResult{Name: "stock_strategy_tool", Error: err.Error()}, err
	}

	strategyCtx := parseStrategyContext(args)
	limit := 50
	if parsed, ok := toInt(args["limit"]); ok && parsed > 0 {
		limit = parsed
	}

	conditions := t.meta.Builder(strategyCtx)
	conditions["strategy_type"] = t.meta.Name
	conditions["limit"] = limit

	riskNotes := "注意仓位与止损"
	if t.meta.RiskNotes != nil {
		riskNotes = t.meta.RiskNotes(strategyCtx)
	}

	return tool.ToolResult{
		Name: t.meta.ToolName,
		Output: map[string]interface{}{
			"strategy_name":        t.meta.Name,
			"strategy_description": t.meta.Description,
			"screening_conditions": conditions,
			"market_context":       fmt.Sprintf("上涨%d家，下跌%d家，上涨占比%.2f", strategyCtx.UpCount, strategyCtx.DownCount, strategyCtx.UpRatio),
			"risk_notes":           riskNotes,
		},
	}, nil
}

func RegisterStockStrategyTools(registry *tool.Registry) {
	if registry == nil {
		return
	}
	registry.Register(NewStockStrategyRouterTool())
	for _, name := range strategyOrder {
		registry.Register(newStockStrategyTool(name))
	}
}

func parseStrategyContext(args map[string]interface{}) strategyContext {
	up, _ := toInt(args["up_count"])
	down, _ := toInt(args["down_count"])
	limitUp, _ := toInt(args["limit_up"])
	limitDown, _ := toInt(args["limit_down"])
	sentiment, _ := args["market_sentiment"].(string)

	upRatio, hasBreadth := marketUpRatio(up, down)
	if !hasBreadth {
		upRatio = upRatioFromSentiment(sentiment)
	}

	return strategyContext{
		UpCount:         up,
		DownCount:       down,
		LimitUp:         limitUp,
		LimitDown:       limitDown,
		UpRatio:         upRatio,
		MarketSentiment: sentiment,
	}
}

func marketUpRatio(up int, down int) (float64, bool) {
	total := up + down
	if total <= 0 {
		return 0.5, false
	}
	return float64(up) / float64(total), true
}

func upRatioFromSentiment(sentiment string) float64 {
	switch strings.ToLower(strings.TrimSpace(sentiment)) {
	case "very_bullish":
		return 0.75
	case "bullish":
		return 0.62
	case "neutral":
		return 0.5
	case "bearish":
		return 0.38
	case "very_bearish":
		return 0.25
	default:
		return 0.5
	}
}

func chooseStrategy(ctx strategyContext) (strategyMeta, string, string) {
	bearGap := ctx.DownCount - ctx.UpCount
	bullGap := ctx.UpCount - ctx.DownCount

	switch {
	case ctx.UpRatio >= 0.75:
		return strategyMetaMap[strategyMomentumStrong], "强势普涨", "上涨占比>=75%，动量策略优先"
	case ctx.UpRatio >= 0.68:
		if ctx.LimitUp >= 40 || bullGap >= 1200 {
			return strategyMetaMap[strategyVolumeBreakout], "高热上行", "涨停/强势股扩散，采用量价突破策略"
		}
		return strategyMetaMap[strategyTrendFollowingCore], "趋势上行", "上涨占比高，采用趋势跟随策略"
	case ctx.UpRatio >= 0.60:
		return strategyMetaMap[strategyTechnicalBreak], "震荡上行", "上涨占比60%-68%，采用技术突破策略"
	case ctx.UpRatio >= 0.54:
		return strategyMetaMap[strategyMACDSignalFollow], "温和上行", "上涨占比较高，采用MACD信号跟随"
	case ctx.UpRatio >= 0.48:
		if strings.EqualFold(strings.TrimSpace(ctx.MarketSentiment), "bullish") {
			return strategyMetaMap[strategyPullbackInUptrend], "结构性上涨", "多头结构中优先回踩买入"
		}
		return strategyMetaMap[strategyMACrossoverTrend], "均衡震荡", "上涨占比接近中位，采用均线趋势策略"
	case ctx.UpRatio >= 0.42:
		return strategyMetaMap[strategySmallCapQuality], "震荡分化", "中性偏弱环境，采用质量与流动性约束"
	case ctx.UpRatio >= 0.34:
		return strategyMetaMap[strategyDefensiveConsol], "弱势防御", "上涨占比偏低，采用防御性震荡策略"
	default:
		if bearGap >= 1200 || ctx.LimitDown >= 40 {
			return strategyMetaMap[strategyOversoldBounce], "恐慌下跌", "跌幅扩散明显，采用超跌反弹策略"
		}
		return strategyMetaMap[strategyPullbackInUptrend], "超跌修复", "弱市修复阶段优先回踩确认策略"
	}
}

func requireStringArg(args map[string]interface{}, key string) (string, error) {
	if args == nil {
		return "", errors.New("missing_args")
	}
	raw, ok := args[key]
	if !ok {
		return "", fmt.Errorf("missing_%s", key)
	}
	s, ok := raw.(string)
	if !ok || strings.TrimSpace(s) == "" {
		return "", fmt.Errorf("invalid_%s", key)
	}
	return s, nil
}
