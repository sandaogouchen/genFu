package technical

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/sandaogouchen/genFu/internal/indicator"
)

// TechnicalAgent 技术分析 Agent，基于计算后的指标数据输出专业解读。
// 实现 agent.Agent 接口。
type TechnicalAgent struct{}

// New 创建技术分析 Agent
func New() *TechnicalAgent {
	return &TechnicalAgent{}
}

// Run 执行技术分析。prompt 应包含 IndicatorResult 的 JSON 数据。
func (a *TechnicalAgent) Run(ctx context.Context, prompt string) (string, error) {
	// 尝试从 prompt 中提取 IndicatorResult JSON
	var result indicator.IndicatorResult
	if err := json.Unmarshal([]byte(prompt), &result); err != nil {
		// 如果不是纯 JSON，尝试从文本中提取 JSON 对象
		jsonStart := strings.Index(prompt, "{")
		jsonEnd := -1
		if jsonStart >= 0 {
			// 使用深度匹配找到正确的结束花括号
			depth := 0
			for i := jsonStart; i < len(prompt); i++ {
				switch prompt[i] {
				case '{':
					depth++
				case '}':
					depth--
					if depth == 0 {
						jsonEnd = i
						break
					}
				}
				if jsonEnd >= 0 {
					break
				}
			}
		}
		if jsonStart >= 0 && jsonEnd > jsonStart {
			if err2 := json.Unmarshal([]byte(prompt[jsonStart:jsonEnd+1]), &result); err2 != nil {
				return "", fmt.Errorf("无法解析技术指标数据: %w", err2)
			}
		} else {
			return "", fmt.Errorf("输入中未找到有效的技术指标 JSON 数据")
		}
	}

	// 生成技术分析报告
	return a.analyze(result), nil
}

// analyze 基于指标数据生成结构化分析
func (a *TechnicalAgent) analyze(result indicator.IndicatorResult) string {
	var sb strings.Builder

	sb.WriteString(fmt.Sprintf("【%s 技术指标分析】\n", result.Symbol))
	sb.WriteString(fmt.Sprintf("数据范围: %s | K线数量: %d\n\n", result.DataRange, result.Count))

	latest := result.Latest

	// 1. MACD 分析
	if len(result.MACD) > 0 {
		sb.WriteString("━━━ MACD 分析 ━━━\n")
		sb.WriteString(fmt.Sprintf("• DIF = %.4f, DEA = %.4f, 柱状图 = %.4f\n", latest.MACD_DIF, latest.MACD_DEA, latest.MACD_Hist))

		if latest.MACD_DIF > latest.MACD_DEA {
			sb.WriteString("• 状态: DIF 在 DEA 上方，短期多头动能\n")
		} else {
			sb.WriteString("• 状态: DIF 在 DEA 下方，短期空头动能\n")
		}

		if latest.MACD_Hist > 0 {
			sb.WriteString("• 柱状图为正且")
			if len(result.MACD) >= 2 {
				prevHist := result.MACD[len(result.MACD)-2].Histogram
				if latest.MACD_Hist > prevHist {
					sb.WriteString("递增，多头动能增强\n")
				} else {
					sb.WriteString("递减，多头动能减弱\n")
				}
			} else {
				sb.WriteString("多头格局\n")
			}
		} else {
			sb.WriteString("• 柱状图为负且")
			if len(result.MACD) >= 2 {
				prevHist := result.MACD[len(result.MACD)-2].Histogram
				if latest.MACD_Hist < prevHist {
					sb.WriteString("递减，空头动能增强\n")
				} else {
					sb.WriteString("递增，空头动能减弱\n")
				}
			} else {
				sb.WriteString("空头格局\n")
			}
		}
		sb.WriteString("\n")
	}

	// 2. RSI 分析
	if len(result.RSI) > 0 {
		sb.WriteString("━━━ RSI 分析 ━━━\n")
		sb.WriteString(fmt.Sprintf("• RSI = %.2f, 区间: %s\n", latest.RSI, zoneLabel(latest.RSI_Zone)))

		switch latest.RSI_Zone {
		case "overbought":
			sb.WriteString("• 处于超买区间（>70），需警惕回调风险\n")
		case "oversold":
			sb.WriteString("• 处于超卖区间（<30），可能存在反弹机会\n")
		default:
			if latest.RSI > 50 {
				sb.WriteString("• 处于中性偏强区间，多头略占优\n")
			} else {
				sb.WriteString("• 处于中性偏弱区间，空头略占优\n")
			}
		}

		// RSI 趋势
		if len(result.RSI) >= 5 {
			rsi5ago := result.RSI[len(result.RSI)-5].Value
			if rsi5ago > 0 {
				if latest.RSI > rsi5ago {
					sb.WriteString(fmt.Sprintf("• 近 5 根 K 线 RSI 从 %.2f 上升至 %.2f，动能向上\n", rsi5ago, latest.RSI))
				} else {
					sb.WriteString(fmt.Sprintf("• 近 5 根 K 线 RSI 从 %.2f 下降至 %.2f，动能向下\n", rsi5ago, latest.RSI))
				}
			}
		}
		sb.WriteString("\n")
	}

	// 3. 布林带分析
	if len(result.Bollinger) > 0 {
		sb.WriteString("━━━ 布林带分析 ━━━\n")
		sb.WriteString(fmt.Sprintf("• 上轨 = %.4f, 中轨 = %.4f, 下轨 = %.4f\n", latest.BB_Upper, latest.BB_Middle, latest.BB_Lower))
		sb.WriteString(fmt.Sprintf("• 当前价格 = %.4f, %%B = %.4f\n", latest.Close, latest.BB_PercentB))

		if latest.BB_PercentB > 1 {
			sb.WriteString("• 价格突破上轨，处于强势区间\n")
		} else if latest.BB_PercentB < 0 {
			sb.WriteString("• 价格跌破下轨，处于弱势区间\n")
		} else if latest.BB_PercentB > 0.8 {
			sb.WriteString("• 价格靠近上轨，注意回调压力\n")
		} else if latest.BB_PercentB < 0.2 {
			sb.WriteString("• 价格靠近下轨，注意反弹机会\n")
		} else {
			sb.WriteString("• 价格处于中轨附近，方向不明确\n")
		}

		// 支撑阻力
		sb.WriteString(fmt.Sprintf("• 动态支撑: %.4f (下轨) | 动态阻力: %.4f (上轨)\n", latest.BB_Lower, latest.BB_Upper))
		sb.WriteString("\n")
	}

	// 4. 多指标共振分析
	sb.WriteString("━━━ 综合研判 ━━━\n")
	bullCount, bearCount := 0, 0

	if len(result.MACD) > 0 {
		if latest.MACD_DIF > latest.MACD_DEA {
			bullCount++
		} else {
			bearCount++
		}
	}

	if len(result.RSI) > 0 {
		if latest.RSI_Zone == "oversold" || (latest.RSI > 40 && latest.RSI < 70) {
			bullCount++
		}
		if latest.RSI_Zone == "overbought" {
			bearCount++
		}
	}

	if len(result.Bollinger) > 0 {
		if latest.BB_PercentB < 0.2 {
			bullCount++ // 超跌反弹机会
		}
		if latest.BB_PercentB > 0.8 {
			bearCount++ // 超涨回调压力
		}
	}

	if bullCount > bearCount {
		sb.WriteString("• 共振方向: 偏多\n")
		sb.WriteString(fmt.Sprintf("• 多头信号: %d | 空头信号: %d\n", bullCount, bearCount))
	} else if bearCount > bullCount {
		sb.WriteString("• 共振方向: 偏空\n")
		sb.WriteString(fmt.Sprintf("• 多头信号: %d | 空头信号: %d\n", bullCount, bearCount))
	} else {
		sb.WriteString("• 共振方向: 指标分歧，方向不明\n")
	}

	// 5. 信号事件
	if len(result.Signals) > 0 {
		sb.WriteString("\n━━━ 近期信号事件 ━━━\n")
		// 只显示最近 10 个信号
		start := len(result.Signals) - 10
		if start < 0 {
			start = 0
		}
		for _, sig := range result.Signals[start:] {
			sb.WriteString(fmt.Sprintf("• [%s] %s: %s\n", sig.Time, sig.Indicator, sig.Detail))
		}
	}

	return sb.String()
}

func zoneLabel(zone string) string {
	switch zone {
	case "overbought":
		return "超买"
	case "oversold":
		return "超卖"
	default:
		return "中性"
	}
}
