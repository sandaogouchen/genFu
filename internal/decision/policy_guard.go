package decision

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	"genFu/internal/investment"
)

type PolicyGuardAgent struct {
	holdings *investment.Repository
}

func NewPolicyGuardAgent(holdings *investment.Repository) *PolicyGuardAgent {
	return &PolicyGuardAgent{holdings: holdings}
}

func (g *PolicyGuardAgent) Guard(ctx context.Context, req GuardRequest) ([]GuardedOrder, error) {
	orders := req.PlannedOrders
	if len(orders) == 0 {
		return []GuardedOrder{}, nil
	}
	budget := req.RiskBudget.Normalize()

	positions := []investment.Position{}
	summary := investment.PortfolioSummary{}
	availableCash := 0.0
	todayNotional := 0.0
	snapshotReady := false

	if g != nil && g.holdings != nil && req.AccountID > 0 {
		var err error
		positions, err = g.holdings.ListPositions(ctx, req.AccountID)
		if err != nil {
			return nil, err
		}
		summary, err = g.holdings.GetPortfolioSummary(ctx, req.AccountID)
		if err != nil {
			return nil, err
		}
		availableCash, err = g.holdings.EstimateAvailableCash(ctx, req.AccountID)
		if err != nil {
			return nil, err
		}
		trades, err := g.holdings.ListTrades(ctx, req.AccountID, 5000, 0)
		if err != nil {
			return nil, err
		}
		todayNotional = calcSameDayTradeNotional(trades, time.Now())
		snapshotReady = true
	}

	symbolQty := map[string]float64{}
	symbolValue := map[string]float64{}
	for _, pos := range positions {
		key := normalizeSymbol(pos.Instrument.Symbol)
		symbolQty[key] += pos.Quantity
		market := pos.AvgCost
		if pos.MarketPrice != nil && *pos.MarketPrice > 0 {
			market = *pos.MarketPrice
		}
		symbolValue[key] += pos.Quantity * market
	}

	totalAssetBase := summary.TotalValue + math.Max(availableCash, 0)
	if totalAssetBase <= 0 {
		totalAssetBase = 1
	}

	runningCash := availableCash
	runningDayNotional := todayNotional
	out := make([]GuardedOrder, 0, len(orders))
	for _, order := range orders {
		item := GuardedOrder{
			PlannedOrder:    order,
			GuardStatus:     "approved",
			ExecutionStatus: "pending",
		}
		action := strings.ToLower(strings.TrimSpace(order.Action))
		notional := order.Notional
		if notional <= 0 {
			notional = order.Quantity * order.Price
		}

		if action != "buy" && action != "sell" {
			blockOrder(&item, "invalid_action")
		}
		if order.Quantity <= 0 || order.Price <= 0 || notional <= 0 {
			blockOrder(&item, "invalid_quantity_or_price")
		}
		if order.Confidence < budget.MinConfidence {
			blockOrder(&item, fmt.Sprintf("confidence_below_threshold: %.4f < %.4f", order.Confidence, budget.MinConfidence))
		}

		key := normalizeSymbol(order.Symbol)
		if item.GuardStatus == "approved" && snapshotReady && notional/totalAssetBase > budget.MaxSingleOrderRatio {
			blockOrder(&item, fmt.Sprintf("single_order_ratio_exceeded: %.4f > %.4f", notional/totalAssetBase, budget.MaxSingleOrderRatio))
		}

		projectedDayNotional := runningDayNotional + notional
		if item.GuardStatus == "approved" && snapshotReady && projectedDayNotional/totalAssetBase > budget.MaxDailyTradeRatio {
			blockOrder(&item, fmt.Sprintf("daily_trade_ratio_exceeded: %.4f > %.4f", projectedDayNotional/totalAssetBase, budget.MaxDailyTradeRatio))
		}

		projectedSymbolValue := symbolValue[key]
		switch action {
		case "buy":
			projectedSymbolValue += notional
		case "sell":
			projectedSymbolValue -= notional
			if projectedSymbolValue < 0 {
				projectedSymbolValue = 0
			}
		}
		if item.GuardStatus == "approved" && snapshotReady && projectedSymbolValue/totalAssetBase > budget.MaxSymbolExposureRatio {
			blockOrder(&item, fmt.Sprintf("symbol_exposure_ratio_exceeded: %.4f > %.4f", projectedSymbolValue/totalAssetBase, budget.MaxSymbolExposureRatio))
		}

		if item.GuardStatus == "approved" && snapshotReady && action == "sell" {
			if order.Quantity > symbolQty[key]+1e-9 {
				blockOrder(&item, fmt.Sprintf("insufficient_sellable_quantity: %.4f > %.4f", order.Quantity, symbolQty[key]))
			}
		}
		if item.GuardStatus == "approved" && snapshotReady && action == "buy" {
			if notional > runningCash+1e-9 {
				blockOrder(&item, fmt.Sprintf("insufficient_available_cash: %.4f > %.4f", notional, runningCash))
			}
		}

		if item.GuardStatus == "approved" {
			switch action {
			case "buy":
				runningCash -= notional
				symbolQty[key] += order.Quantity
				symbolValue[key] += notional
			case "sell":
				runningCash += notional
				symbolQty[key] -= order.Quantity
				if symbolQty[key] < 0 {
					symbolQty[key] = 0
				}
				symbolValue[key] -= notional
				if symbolValue[key] < 0 {
					symbolValue[key] = 0
				}
			}
			runningDayNotional = projectedDayNotional
		}

		out = append(out, item)
	}
	return out, nil
}

func calcSameDayTradeNotional(trades []investment.Trade, now time.Time) float64 {
	total := 0.0
	y, m, d := now.Date()
	for _, trade := range trades {
		ty, tm, td := trade.TradeAt.Date()
		if y == ty && m == tm && d == td {
			total += math.Abs(trade.Quantity * trade.Price)
		}
	}
	return total
}

func blockOrder(item *GuardedOrder, reason string) {
	if item == nil {
		return
	}
	item.GuardStatus = "blocked"
	item.GuardReason = reason
	item.ExecutionStatus = "blocked"
	item.ExecutionError = reason
}

func normalizeSymbol(symbol string) string {
	return strings.ToUpper(strings.TrimSpace(symbol))
}
