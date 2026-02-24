package trade_signal

import (
	"context"
	"strings"
	"time"

	"genFu/internal/investment"
)

type Engine interface {
	Execute(ctx context.Context, signals []TradeSignal) ([]ExecutionResult, error)
}

type InvestmentEngine struct {
	service *investment.Service
}

func NewInvestmentEngine(service *investment.Service) *InvestmentEngine {
	return &InvestmentEngine{service: service}
}

func (e *InvestmentEngine) Execute(ctx context.Context, signals []TradeSignal) ([]ExecutionResult, error) {
	results := make([]ExecutionResult, 0, len(signals))
	for _, signal := range signals {
		action := strings.ToLower(strings.TrimSpace(signal.Action))
		if action == "hold" {
			results = append(results, ExecutionResult{Signal: signal, Status: "skipped"})
			continue
		}
		instrument, err := e.service.UpsertInstrument(ctx, signal.Symbol, signal.Name, signal.AssetType)
		if err != nil {
			results = append(results, ExecutionResult{Signal: signal, Status: "failed", Error: err.Error()})
			continue
		}
		trade, position, err := e.service.RecordTrade(ctx, signal.AccountID, instrument.ID, action, signal.Quantity, signal.Price, 0, time.Now(), signal.Reason)
		if err != nil {
			results = append(results, ExecutionResult{Signal: signal, Status: "failed", Error: err.Error()})
			continue
		}
		results = append(results, ExecutionResult{
			Signal:   signal,
			Trade:    &trade,
			Position: &position,
			Status:   "executed",
		})
	}
	return results, nil
}
