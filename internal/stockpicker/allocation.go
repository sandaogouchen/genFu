package stockpicker

import (
	"math"

	"genFu/internal/tool"
)

// AllocationService 资产配置服务
type AllocationService struct{}

// NewAllocationService 创建资产配置服务
func NewAllocationService() *AllocationService {
	return &AllocationService{}
}

// CalculateAllocation 计算资产配置建议
func (s *AllocationService) CalculateAllocation(
	candidate *StockPick,
	holdings []Position,
	allStocks []tool.MarketItem,
) Allocation {
	allocation := Allocation{
		SuggestedWeight:        0.1, // 默认10%
		IndustryDiversity:      s.calculateIndustryDiversity(candidate.Industry, holdings),
		RiskExposure:           s.calculateRiskExposure(candidate),
		LiquidityScore:         s.calculateLiquidityScore(candidate.Symbol, allStocks),
		CorrelationWithHolding: s.calculateCorrelation(candidate.Symbol, holdings),
	}

	// 根据各项指标调整权重
	allocation.SuggestedWeight = s.adjustWeight(allocation)

	return allocation
}

// calculateIndustryDiversity 计算行业分散度
func (s *AllocationService) calculateIndustryDiversity(industry string, holdings []Position) float64 {
	if len(holdings) == 0 {
		return 1.0 // 无持仓，分散度最高
	}

	// 统计各行业权重
	industryMap := make(map[string]float64)
	totalValue := 0.0
	for _, h := range holdings {
		industryMap[h.Industry] += h.Value
		totalValue += h.Value
	}

	if totalValue == 0 {
		return 1.0
	}

	// 计算目标行业占比
	targetIndustryRatio := industryMap[industry] / totalValue

	// 分散度: 目标行业占比越低，分散度越高
	diversity := 1.0 - targetIndustryRatio
	return math.Max(0.1, math.Min(1.0, diversity))
}

// calculateRiskExposure 计算风险敞口
func (s *AllocationService) calculateRiskExposure(candidate *StockPick) float64 {
	// 基于技术面风险评估
	riskScore := 0.3 // 基础风险

	// 根据confidence调整
	if candidate.Confidence > 0.8 {
		riskScore += 0.1 // 高置信度可能过度自信
	} else if candidate.Confidence < 0.5 {
		riskScore += 0.3 // 低置信度风险更高
	}

	// 根据风险等级调整
	switch candidate.RiskLevel {
	case "high":
		riskScore += 0.3
	case "medium":
		riskScore += 0.1
	case "low":
		// 不增加
	}

	return math.Min(1.0, riskScore)
}

// calculateLiquidityScore 计算流动性评分
func (s *AllocationService) calculateLiquidityScore(symbol string, allStocks []tool.MarketItem) float64 {
	// 查找目标股票的成交额
	for _, stock := range allStocks {
		if stock.Code == symbol {
			// 成交额大于1亿: 1.0
			// 成交额5000万-1亿: 0.8
			// 成交额1000万-5000万: 0.6
			// 成交额小于1000万: 0.3
			amount := stock.Amount
			if amount >= 1e8 {
				return 1.0
			} else if amount >= 5e7 {
				return 0.8
			} else if amount >= 1e7 {
				return 0.6
			}
			return 0.3
		}
	}
	return 0.0 // 未找到
}

// calculateCorrelation 计算与持仓的相关性
func (s *AllocationService) calculateCorrelation(symbol string, holdings []Position) float64 {
	// 简化实现: 检查是否已持有
	for _, h := range holdings {
		if h.Symbol == symbol {
			return 1.0 // 已持有，相关性最高
		}
	}

	// TODO: 可以扩展为基于历史价格序列计算相关系数
	return 0.0
}

// adjustWeight 根据各项指标调整权重
func (s *AllocationService) adjustWeight(allocation Allocation) float64 {
	weight := allocation.SuggestedWeight

	// 行业分散度高，可以增加权重
	if allocation.IndustryDiversity > 0.8 {
		weight *= 1.2
	} else if allocation.IndustryDiversity < 0.3 {
		weight *= 0.7
	}

	// 风险敞口高，降低权重
	if allocation.RiskExposure > 0.7 {
		weight *= 0.6
	}

	// 流动性差，降低权重
	if allocation.LiquidityScore < 0.5 {
		weight *= 0.5
	}

	// 与持仓相关高，降低权重
	if allocation.CorrelationWithHolding > 0.7 {
		weight *= 0.6
	}

	// 限制在5%-20%之间
	return math.Max(0.05, math.Min(0.2, weight))
}
