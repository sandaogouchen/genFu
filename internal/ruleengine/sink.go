package ruleengine

import (
	"context"
	"fmt"
	"log"
)

// ---------------------------------------------------------------------------
// LogSignalSink -- simple logging sink
// ---------------------------------------------------------------------------

// LogSignalSink writes every received signal to the standard logger.
// It is mainly useful during development and debugging.
type LogSignalSink struct{}

// NewLogSignalSink returns a ready-to-use LogSignalSink.
func NewLogSignalSink() *LogSignalSink {
	return &LogSignalSink{}
}

// EmitSignal logs the signal details and always returns nil.
func (s *LogSignalSink) EmitSignal(ctx context.Context, signal TriggerSignal) error {
	log.Printf("[RuleEngine] Signal: %s %s %s action=%s price=%.2f reason=%s",
		signal.RuleType, signal.Symbol, signal.RuleName, signal.Action.ActionType,
		signal.MarketPrice, signal.Reason)
	return nil
}

// ---------------------------------------------------------------------------
// TradeSignalSink -- persists trigger events then optionally chains
// ---------------------------------------------------------------------------

// TradeSignalSink converts rule-engine triggers into persisted trigger events
// and forwards them to an optional inner SignalSink.  This is the primary sink
// used in production: it makes sure every trigger is recorded in the store
// before the signal is handed to downstream consumers (e.g. a decision
// pipeline or order executor).
type TradeSignalSink struct {
	store RuleStore
	inner SignalSink // optional chained sink
}

// NewTradeSignalSink creates a TradeSignalSink backed by the given store.
// If inner is non-nil every successfully logged signal is also forwarded to it.
func NewTradeSignalSink(store RuleStore, inner SignalSink) *TradeSignalSink {
	return &TradeSignalSink{
		store: store,
		inner: inner,
	}
}

// EmitSignal persists a TriggerEvent derived from the signal, then optionally
// forwards the signal to the chained inner sink.
func (s *TradeSignalSink) EmitSignal(ctx context.Context, signal TriggerSignal) error {
	event := TriggerEvent{
		RuleID:       signal.RuleID,
		AccountID:    signal.AccountID,
		Symbol:       signal.Symbol,
		TriggerPrice: signal.TriggerPrice,
		MarketPrice:  signal.MarketPrice,
		PnLPct:       signal.PnLPct,
		Action:       signal.Action,
		Status:       "triggered",
		Reason:       signal.Reason,
		TriggeredAt:  signal.TriggeredAt,
	}
	if err := s.store.LogTrigger(ctx, &event); err != nil {
		return fmt.Errorf("log trigger: %w", err)
	}

	// Chain to inner sink if present.
	if s.inner != nil {
		return s.inner.EmitSignal(ctx, signal)
	}
	return nil
}
