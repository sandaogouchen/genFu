package ruleengine

import (
	"context"
	"fmt"
	"log"
	"sync/atomic"
	"time"
)

// MarketDataProvider abstracts real-time market data retrieval.
type MarketDataProvider interface {
	GetLatestPrice(ctx context.Context, symbol string) (float64, time.Time, error)
	GetDailyChange(ctx context.Context, symbol string) (float64, error)
}

// SignalSink receives trigger signals produced by the monitor's evaluation loop.
type SignalSink interface {
	EmitSignal(ctx context.Context, signal TriggerSignal) error
}

// TradingHours defines the intraday windows during which the monitor is active.
type TradingHours struct {
	MorningOpen    string // e.g. "09:30"
	MorningClose   string // e.g. "11:30"
	AfternoonOpen  string // e.g. "13:00"
	AfternoonClose string // e.g. "15:00"
	Timezone       string // IANA timezone, e.g. "Asia/Shanghai"
}

// Monitor periodically polls positions, refreshes market data, evaluates rules
// through the Engine, and dispatches resulting signals.
type Monitor struct {
	engine       *Engine
	tracker      PositionTracker
	store        RuleStore
	marketData   MarketDataProvider
	signalSink   SignalSink
	pollInterval time.Duration
	tradingHours TradingHours
	running      atomic.Bool
	stopCh       chan struct{}
	accountID    int64
}

// NewMonitor creates a Monitor wired to the given engine, tracker and store.
// Functional options customise polling interval, market-data source, etc.
func NewMonitor(engine *Engine, tracker PositionTracker, store RuleStore, opts ...MonitorOption) *Monitor {
	m := &Monitor{
		engine:       engine,
		tracker:      tracker,
		store:        store,
		pollInterval: 5 * time.Second, // sensible default
		stopCh:       make(chan struct{}),
	}
	for _, o := range opts {
		o(m)
	}
	return m
}

// MonitorOption is a functional option for NewMonitor.
type MonitorOption func(*Monitor)

// WithPollInterval sets how often the monitor runs a tick cycle.
func WithPollInterval(d time.Duration) MonitorOption {
	return func(m *Monitor) { m.pollInterval = d }
}

// WithMarketData plugs in a live-price provider so the monitor can refresh
// quotes before evaluating rules.
func WithMarketData(md MarketDataProvider) MonitorOption {
	return func(m *Monitor) { m.marketData = md }
}

// WithSignalSink sets the destination for triggered signals.
func WithSignalSink(sink SignalSink) MonitorOption {
	return func(m *Monitor) { m.signalSink = sink }
}

// WithTradingHours restricts monitoring to configured market sessions.
func WithTradingHours(th TradingHours) MonitorOption {
	return func(m *Monitor) { m.tradingHours = th }
}

// WithAccountID scopes the monitor to a single account.
func WithAccountID(id int64) MonitorOption {
	return func(m *Monitor) { m.accountID = id }
}

// Start launches the background polling goroutine. It returns an error if the
// monitor is already running.
func (m *Monitor) Start(ctx context.Context) error {
	if m.running.Load() {
		return fmt.Errorf("monitor already running")
	}
	m.running.Store(true)

	go func() {
		ticker := time.NewTicker(m.pollInterval)
		defer ticker.Stop()
		defer m.running.Store(false)

		// Run an initial tick immediately.
		if err := m.tick(ctx); err != nil {
			log.Printf("[RuleEngine] tick error: %v", err)
		}

		for {
			select {
			case <-ctx.Done():
				log.Printf("[RuleEngine] monitor stopping: context cancelled")
				return
			case <-m.stopCh:
				log.Printf("[RuleEngine] monitor stopping: stop requested")
				return
			case <-ticker.C:
				if err := m.tick(ctx); err != nil {
					log.Printf("[RuleEngine] tick error: %v", err)
				}
			}
		}
	}()

	return nil
}

// Stop signals the polling goroutine to exit.
func (m *Monitor) Stop() error {
	if !m.running.Load() {
		return fmt.Errorf("monitor is not running")
	}
	close(m.stopCh)
	return nil
}

// IsRunning reports whether the polling goroutine is active.
func (m *Monitor) IsRunning() bool {
	return m.running.Load()
}

// tick executes a single poll cycle:
//  1. Gate on trading hours (if configured).
//  2. Refresh prices from market-data provider.
//  3. Evaluate all rules via engine.CheckAll.
//  4. Dispatch triggered signals.
func (m *Monitor) tick(ctx context.Context) error {
	// 1. Trading-hours gate.
	if !m.isWithinTradingHours() {
		return nil
	}

	// 2. Fetch current position snapshots.
	snapshots, err := m.tracker.GetAllSnapshots(ctx, m.accountID)
	if err != nil {
		return fmt.Errorf("get snapshots: %w", err)
	}

	// 3. Refresh market data for every position.
	now := time.Now()
	for i := range snapshots {
		snap := &snapshots[i]
		if m.marketData != nil {
			price, _, priceErr := m.marketData.GetLatestPrice(ctx, snap.Symbol)
			if priceErr != nil {
				log.Printf("[RuleEngine] price fetch failed for %s: %v", snap.Symbol, priceErr)
				continue
			}
			if updateErr := m.tracker.UpdatePrice(ctx, m.accountID, snap.Symbol, price, now); updateErr != nil {
				log.Printf("[RuleEngine] price update failed for %s: %v", snap.Symbol, updateErr)
				continue
			}
			snap.CurrentPrice = price
			snap.MarketPrice = price

			dailyChange, dcErr := m.marketData.GetDailyChange(ctx, snap.Symbol)
			if dcErr == nil {
				snap.DailyChangePct = dailyChange
			}
		}
	}

	// 4. Evaluate all rules.
	results, err := m.engine.CheckAll(ctx, m.accountID)
	if err != nil {
		return fmt.Errorf("check all: %w", err)
	}

	// 5. Dispatch triggered results.
	var firstErr error
	for _, res := range results {
		if !res.Triggered {
			continue
		}

		signal := TriggerSignal{
			RuleID:       res.RuleID,
			RuleName:     res.RuleName,
			RuleType:     res.RuleType,
			AccountID:    m.accountID,
			Symbol:       res.Symbol,
			TriggerPrice: res.TriggerPrice,
			MarketPrice:  res.MarketPrice,
			PnLPct:       res.PnLPct,
			Action:       res.Action,
			Reason:       res.Reason,
			TriggeredAt:  time.Now(),
		}

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
		if logErr := m.store.LogTrigger(ctx, &event); logErr != nil {
			log.Printf("[RuleEngine] log trigger failed: %v", logErr)
			if firstErr == nil {
				firstErr = logErr
			}
		}

		if m.signalSink != nil {
			if emitErr := m.signalSink.EmitSignal(ctx, signal); emitErr != nil {
				log.Printf("[RuleEngine] emit signal failed: %v", emitErr)
				if firstErr == nil {
					firstErr = emitErr
				}
			}
		}
	}

	return firstErr
}

// isWithinTradingHours returns true when the current wall-clock time falls
// inside one of the configured trading sessions. If no timezone is set the
// method is a no-op and always returns true.
func (m *Monitor) isWithinTradingHours() bool {
	tz := m.tradingHours.Timezone
	if tz == "" {
		return true
	}

	loc, err := time.LoadLocation(tz)
	if err != nil {
		log.Printf("[RuleEngine] invalid timezone %q, allowing tick: %v", tz, err)
		return true
	}
	now := time.Now().In(loc)
	hhmm := now.Format("15:04")

	inMorning := m.tradingHours.MorningOpen != "" &&
		m.tradingHours.MorningClose != "" &&
		hhmm >= m.tradingHours.MorningOpen &&
		hhmm <= m.tradingHours.MorningClose

	inAfternoon := m.tradingHours.AfternoonOpen != "" &&
		m.tradingHours.AfternoonClose != "" &&
		hhmm >= m.tradingHours.AfternoonOpen &&
		hhmm <= m.tradingHours.AfternoonClose

	return inMorning || inAfternoon
}
