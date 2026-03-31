package tool

import (
	"context"
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"genFu/internal/db"
	"genFu/internal/investment"
	"genFu/internal/testutil"
)

type testQuoteResult struct {
	price float64
	err   error
}

type stubPortfolioPriceResolver struct {
	stock map[string]testQuoteResult
	fund  map[string]testQuoteResult
}

type stubInstrumentSearchProvider struct {
	searchResults []SearchItem
	searchErr     error
	stockResult   *SearchItem
	stockErr      error
}

func (s stubInstrumentSearchProvider) SearchInstruments(_ context.Context, _ string, _ int) ([]SearchItem, error) {
	if s.searchErr != nil {
		return nil, s.searchErr
	}
	return append([]SearchItem(nil), s.searchResults...), nil
}

func (s stubInstrumentSearchProvider) SearchStockByCode(_ context.Context, _ string) (*SearchItem, error) {
	if s.stockErr != nil {
		return nil, s.stockErr
	}
	if s.stockResult == nil {
		return nil, nil
	}
	item := *s.stockResult
	return &item, nil
}

type stubMarketDataExecutor struct {
	mu      sync.Mutex
	calls   []string
	results map[string]ToolResult
	errs    map[string]error
}

func (s *stubMarketDataExecutor) Execute(_ context.Context, args map[string]interface{}) (ToolResult, error) {
	action, _ := args["action"].(string)
	code, _ := args["code"].(string)
	key := action + ":" + code

	s.mu.Lock()
	s.calls = append(s.calls, key)
	s.mu.Unlock()

	if err, ok := s.errs[key]; ok && err != nil {
		return ToolResult{Name: "marketdata"}, err
	}
	if result, ok := s.results[key]; ok {
		return result, nil
	}
	return ToolResult{Name: "marketdata", Error: "not_found"}, nil
}

func (s *stubMarketDataExecutor) CalledKeys() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	copied := make([]string, len(s.calls))
	copy(copied, s.calls)
	return copied
}

func (s stubPortfolioPriceResolver) ResolveStockPrice(_ context.Context, symbol string) (float64, error) {
	if s.stock == nil {
		return 0, errors.New("stock_quote_not_configured")
	}
	if v, ok := s.stock[symbol]; ok {
		return v.price, v.err
	}
	return 0, errors.New("stock_quote_not_found")
}

func (s stubPortfolioPriceResolver) ResolveFundPrice(_ context.Context, symbol string) (float64, error) {
	if s.fund == nil {
		return 0, errors.New("fund_quote_not_configured")
	}
	if v, ok := s.fund[symbol]; ok {
		return v.price, v.err
	}
	return 0, errors.New("fund_quote_not_found")
}

func setupInvestmentToolTest(t *testing.T) (context.Context, *investment.Repository, *investment.Service, int64) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "test.db")
	cfg := testutil.LoadConfig(t)
	dbCfg := cfg.PG
	dbCfg.DSN = "file:" + path
	conn, err := db.Open(db.Config{
		DSN:             dbCfg.DSN,
		MaxOpenConns:    dbCfg.MaxOpenConns,
		MaxIdleConns:    dbCfg.MaxIdleConns,
		ConnMaxLifetime: dbCfg.ConnMaxLifetime,
	})
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("wd: %v", err)
	}
	if err := os.Chdir(filepath.Join(wd, "..", "..")); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(wd)
	})
	if err := db.ApplyMigrations(context.Background(), conn); err != nil {
		t.Fatalf("migrations: %v", err)
	}
	ctx := context.Background()
	repo := investment.NewRepository(conn)
	svc := investment.NewService(repo)
	user, err := repo.CreateUser(ctx, "u")
	if err != nil {
		t.Fatalf("create user: %v", err)
	}
	account, err := repo.CreateAccount(ctx, user.ID, "a", "CNY")
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	return ctx, repo, svc, account.ID
}

func upsertPositionForTest(t *testing.T, ctx context.Context, repo *investment.Repository, accountID int64, symbol string, name string, assetType string, quantity float64, avgCost float64, marketPrice *float64) {
	t.Helper()
	instrument, err := repo.UpsertInstrument(ctx, symbol, name, assetType)
	if err != nil {
		t.Fatalf("upsert instrument %s: %v", symbol, err)
	}
	if _, err := repo.SetPosition(ctx, accountID, instrument.ID, quantity, avgCost, marketPrice); err != nil {
		t.Fatalf("set position %s: %v", symbol, err)
	}
}

func assertAlmostEqual(t *testing.T, actual float64, expected float64) {
	t.Helper()
	const epsilon = 1e-8
	if math.Abs(actual-expected) > epsilon {
		t.Fatalf("expected %.8f, got %.8f", expected, actual)
	}
}

func snapshotPositionBySymbol(snapshot PortfolioSnapshot, symbol string) (PortfolioSnapshotPosition, bool) {
	for _, p := range snapshot.Positions {
		if p.Instrument.Symbol == symbol {
			return p, true
		}
	}
	return PortfolioSnapshotPosition{}, false
}

func hasString(items []string, target string) bool {
	for _, item := range items {
		if item == target {
			return true
		}
	}
	return false
}

func TestInvestmentToolListFundHoldings(t *testing.T) {
	ctx, repo, svc, accountID := setupInvestmentToolTest(t)
	fundInstrument, err := repo.UpsertInstrument(ctx, "FUND1", "fund", "fund")
	if err != nil {
		t.Fatalf("fund instrument: %v", err)
	}
	stockInstrument, err := repo.UpsertInstrument(ctx, "STK1", "stock", "stock")
	if err != nil {
		t.Fatalf("stock instrument: %v", err)
	}
	if _, err := repo.SetPosition(ctx, accountID, fundInstrument.ID, 1, 1, nil); err != nil {
		t.Fatalf("set fund position: %v", err)
	}
	if _, err := repo.SetPosition(ctx, accountID, stockInstrument.ID, 1, 1, nil); err != nil {
		t.Fatalf("set stock position: %v", err)
	}
	tool := NewInvestmentTool(svc)
	result, err := tool.Execute(ctx, map[string]interface{}{
		"action":     "list_fund_holdings",
		"account_id": accountID,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	positions, ok := result.Output.([]investment.Position)
	if !ok {
		t.Fatalf("unexpected output type")
	}
	if len(positions) != 1 {
		t.Fatalf("unexpected positions length: %d", len(positions))
	}
	if positions[0].Instrument.AssetType != "fund" {
		t.Fatalf("unexpected asset type")
	}
}

func TestInvestmentToolPortfolioSnapshotRealtimeSuccess(t *testing.T) {
	ctx, repo, svc, accountID := setupInvestmentToolTest(t)
	upsertPositionForTest(t, ctx, repo, accountID, "600000", "浦发银行", "stock", 10, 10, nil)
	upsertPositionForTest(t, ctx, repo, accountID, "000001", "华夏成长", "fund", 100, 1, nil)

	tool := NewInvestmentToolWithResolver(svc, NewEastMoneyTool(), NewMarketDataTool(svc), stubPortfolioPriceResolver{
		stock: map[string]testQuoteResult{
			"600000": {price: 12},
		},
		fund: map[string]testQuoteResult{
			"000001": {price: 1.2},
		},
	})

	result, err := tool.Execute(ctx, map[string]interface{}{
		"action":     "get_portfolio_snapshot",
		"account_id": accountID,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	snapshot, ok := result.Output.(PortfolioSnapshot)
	if !ok {
		t.Fatalf("unexpected output type: %T", result.Output)
	}
	if len(snapshot.Positions) != 2 {
		t.Fatalf("unexpected positions len: %d", len(snapshot.Positions))
	}
	if snapshot.HasStalePrices {
		t.Fatalf("expected no stale prices")
	}
	if len(snapshot.PriceFailures) != 0 {
		t.Fatalf("expected no price failures")
	}

	stock, ok := snapshotPositionBySymbol(snapshot, "600000")
	if !ok {
		t.Fatalf("missing stock position")
	}
	assertAlmostEqual(t, stock.CurrentPrice, 12)
	if stock.PriceSource != portfolioPriceSourceRealtime {
		t.Fatalf("unexpected stock source: %s", stock.PriceSource)
	}
	assertAlmostEqual(t, stock.Cost, 100)
	assertAlmostEqual(t, stock.MarketValue, 120)
	assertAlmostEqual(t, stock.PnL, 20)
	assertAlmostEqual(t, stock.PnLPct, 0.2)

	fund, ok := snapshotPositionBySymbol(snapshot, "000001")
	if !ok {
		t.Fatalf("missing fund position")
	}
	assertAlmostEqual(t, fund.CurrentPrice, 1.2)
	if fund.PriceSource != portfolioPriceSourceRealtime {
		t.Fatalf("unexpected fund source: %s", fund.PriceSource)
	}
	assertAlmostEqual(t, fund.Cost, 100)
	assertAlmostEqual(t, fund.MarketValue, 120)
	assertAlmostEqual(t, fund.PnL, 20)
	assertAlmostEqual(t, fund.PnLPct, 0.2)

	assertAlmostEqual(t, snapshot.Summary.TotalCost, 200)
	assertAlmostEqual(t, snapshot.Summary.TotalValue, 240)
	assertAlmostEqual(t, snapshot.Summary.TotalPnL, 40)
	assertAlmostEqual(t, snapshot.Summary.PnLPct, 0.2)
}

func TestInvestmentToolPortfolioSnapshotFallbackStoredPrice(t *testing.T) {
	ctx, repo, svc, accountID := setupInvestmentToolTest(t)
	stored := 9.0
	upsertPositionForTest(t, ctx, repo, accountID, "600519", "贵州茅台", "stock", 10, 10, &stored)

	tool := NewInvestmentToolWithResolver(svc, NewEastMoneyTool(), NewMarketDataTool(svc), stubPortfolioPriceResolver{
		stock: map[string]testQuoteResult{
			"600519": {err: errors.New("network_timeout")},
		},
	})

	result, err := tool.Execute(ctx, map[string]interface{}{
		"action":     "get_portfolio_snapshot",
		"account_id": accountID,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	snapshot, ok := result.Output.(PortfolioSnapshot)
	if !ok {
		t.Fatalf("unexpected output type: %T", result.Output)
	}
	if len(snapshot.Positions) != 1 {
		t.Fatalf("unexpected positions len: %d", len(snapshot.Positions))
	}
	position := snapshot.Positions[0]
	if position.PriceSource != portfolioPriceSourceStored {
		t.Fatalf("expected stored price source, got: %s", position.PriceSource)
	}
	assertAlmostEqual(t, position.CurrentPrice, 9)
	assertAlmostEqual(t, position.PnL, -10)
	if !snapshot.HasStalePrices {
		t.Fatalf("expected stale prices")
	}
	if len(snapshot.PriceFailures) != 1 {
		t.Fatalf("expected one failure, got: %d", len(snapshot.PriceFailures))
	}
	if !strings.Contains(snapshot.PriceFailures[0].Reason, "stock_quote_failed") {
		t.Fatalf("unexpected failure reason: %s", snapshot.PriceFailures[0].Reason)
	}
}

func TestInvestmentToolPortfolioSnapshotFallbackAvgCost(t *testing.T) {
	ctx, repo, svc, accountID := setupInvestmentToolTest(t)
	upsertPositionForTest(t, ctx, repo, accountID, "600036", "招商银行", "stock", 15, 8, nil)

	tool := NewInvestmentToolWithResolver(svc, NewEastMoneyTool(), NewMarketDataTool(svc), stubPortfolioPriceResolver{
		stock: map[string]testQuoteResult{
			"600036": {err: errors.New("quote_unavailable")},
		},
	})

	result, err := tool.Execute(ctx, map[string]interface{}{
		"action":     "get_portfolio_snapshot",
		"account_id": accountID,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	snapshot, ok := result.Output.(PortfolioSnapshot)
	if !ok {
		t.Fatalf("unexpected output type: %T", result.Output)
	}
	position := snapshot.Positions[0]
	if position.PriceSource != portfolioPriceSourceAvgCost {
		t.Fatalf("expected avg_cost source, got: %s", position.PriceSource)
	}
	assertAlmostEqual(t, position.CurrentPrice, 8)
	assertAlmostEqual(t, position.PnL, 0)
	if !snapshot.HasStalePrices {
		t.Fatalf("expected stale prices")
	}
	if len(snapshot.PriceFailures) != 1 {
		t.Fatalf("expected one failure, got: %d", len(snapshot.PriceFailures))
	}
}

func TestInvestmentToolPortfolioSnapshotEmptyPositions(t *testing.T) {
	ctx, _, svc, accountID := setupInvestmentToolTest(t)

	tool := NewInvestmentToolWithResolver(svc, NewEastMoneyTool(), NewMarketDataTool(svc), stubPortfolioPriceResolver{})
	result, err := tool.Execute(ctx, map[string]interface{}{
		"action":     "get_portfolio_snapshot",
		"account_id": accountID,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	snapshot, ok := result.Output.(PortfolioSnapshot)
	if !ok {
		t.Fatalf("unexpected output type: %T", result.Output)
	}
	if len(snapshot.Positions) != 0 {
		t.Fatalf("expected empty positions")
	}
	assertAlmostEqual(t, snapshot.Summary.TotalCost, 0)
	assertAlmostEqual(t, snapshot.Summary.TotalValue, 0)
	assertAlmostEqual(t, snapshot.Summary.TotalPnL, 0)
	assertAlmostEqual(t, snapshot.Summary.PnLPct, 0)
	if snapshot.HasStalePrices {
		t.Fatalf("expected no stale prices")
	}
	if len(snapshot.PriceFailures) != 0 {
		t.Fatalf("expected no failures")
	}
}

func TestInvestmentToolPortfolioSnapshotDefaultAccount(t *testing.T) {
	ctx, repo, svc, _ := setupInvestmentToolTest(t)
	defaultAccountID, err := svc.DefaultAccountID(ctx)
	if err != nil {
		t.Fatalf("default account id: %v", err)
	}
	upsertPositionForTest(t, ctx, repo, defaultAccountID, "000858", "五粮液", "stock", 20, 5, nil)

	tool := NewInvestmentToolWithResolver(svc, NewEastMoneyTool(), NewMarketDataTool(svc), stubPortfolioPriceResolver{
		stock: map[string]testQuoteResult{
			"000858": {price: 6},
		},
	})

	result, err := tool.Execute(ctx, map[string]interface{}{
		"action": "get_portfolio_snapshot",
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	snapshot, ok := result.Output.(PortfolioSnapshot)
	if !ok {
		t.Fatalf("unexpected output type: %T", result.Output)
	}
	if snapshot.Summary.AccountID != defaultAccountID {
		t.Fatalf("unexpected account id: got=%d want=%d", snapshot.Summary.AccountID, defaultAccountID)
	}
	if len(snapshot.Positions) != 1 {
		t.Fatalf("unexpected positions len: %d", len(snapshot.Positions))
	}
	if snapshot.Positions[0].PriceSource != portfolioPriceSourceRealtime {
		t.Fatalf("unexpected price source: %s", snapshot.Positions[0].PriceSource)
	}
}

func TestDefaultPortfolioPriceResolverUsesMarketDataIntraday(t *testing.T) {
	executor := &stubMarketDataExecutor{
		results: map[string]ToolResult{
			"get_stock_intraday:600519": {
				Name:   "marketdata",
				Output: []IntradayPoint{{Time: "09:31", Price: 1480}, {Time: "09:32", Price: 1492.5}},
			},
			"get_fund_intraday:014597": {
				Name:   "marketdata",
				Output: []IntradayPoint{{Time: "14:59", Price: 0}, {Time: "15:00", Price: 1.023}},
			},
		},
	}
	resolver := newDefaultPortfolioPriceResolver(executor)
	ctx := context.Background()

	stockPrice, err := resolver.ResolveStockPrice(ctx, "600519")
	if err != nil {
		t.Fatalf("resolve stock price: %v", err)
	}
	fundPrice, err := resolver.ResolveFundPrice(ctx, "014597")
	if err != nil {
		t.Fatalf("resolve fund price: %v", err)
	}

	assertAlmostEqual(t, stockPrice, 1492.5)
	assertAlmostEqual(t, fundPrice, 1.023)

	calls := executor.CalledKeys()
	expected := []string{
		"get_stock_intraday:600519",
		"get_fund_intraday:014597",
	}
	for _, key := range expected {
		if !hasString(calls, key) {
			t.Fatalf("missing marketdata call: %s, calls=%v", key, calls)
		}
	}
	for _, key := range calls {
		if strings.HasPrefix(key, "get_stock_quote:") {
			t.Fatalf("stock quote should not be called, calls=%v", calls)
		}
	}
}

func TestInvestmentToolPortfolioSnapshotQuoteCandidateFallback(t *testing.T) {
	ctx, repo, svc, accountID := setupInvestmentToolTest(t)
	upsertPositionForTest(t, ctx, repo, accountID, "华泰", "014597", "fund", 1000, 1, nil)

	tool := NewInvestmentToolWithResolver(svc, NewEastMoneyTool(), NewMarketDataTool(svc), stubPortfolioPriceResolver{
		fund: map[string]testQuoteResult{
			"华泰":     {err: errors.New("fund_request_failed")},
			"014597": {price: 1.01},
		},
	})

	result, err := tool.Execute(ctx, map[string]interface{}{
		"action":     "get_portfolio_snapshot",
		"account_id": accountID,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	snapshot, ok := result.Output.(PortfolioSnapshot)
	if !ok {
		t.Fatalf("unexpected output type: %T", result.Output)
	}
	if len(snapshot.Positions) != 1 {
		t.Fatalf("unexpected positions len: %d", len(snapshot.Positions))
	}
	if snapshot.Positions[0].PriceSource != portfolioPriceSourceRealtime {
		t.Fatalf("expected realtime price source, got: %s", snapshot.Positions[0].PriceSource)
	}
	assertAlmostEqual(t, snapshot.Positions[0].CurrentPrice, 1.01)
	if snapshot.HasStalePrices {
		t.Fatalf("expected no stale prices")
	}
	if len(snapshot.PriceFailures) != 0 {
		t.Fatalf("expected no failures, got: %+v", snapshot.PriceFailures)
	}
}

func TestInvestmentToolAddPositionByQuantity(t *testing.T) {
	ctx, _, svc, accountID := setupInvestmentToolTest(t)
	tool := NewInvestmentTool(svc)

	result, err := tool.Execute(ctx, map[string]interface{}{
		"action":       "add_position_by_quantity",
		"account_id":   accountID,
		"symbol":       "600000",
		"name":         "浦发银行",
		"asset_type":   "stock",
		"quantity":     12.5,
		"avg_cost":     10.2,
		"market_price": 10.8,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	position, ok := result.Output.(investment.Position)
	if !ok {
		t.Fatalf("unexpected output type: %T", result.Output)
	}
	assertAlmostEqual(t, position.Quantity, 12.5)
	assertAlmostEqual(t, position.AvgCost, 10.2)
	if position.MarketPrice == nil {
		t.Fatalf("expected market price")
	}
	assertAlmostEqual(t, *position.MarketPrice, 10.8)
	if position.Instrument.Symbol != "600000" {
		t.Fatalf("unexpected symbol: %s", position.Instrument.Symbol)
	}
}

func TestInvestmentToolAddPositionByQuantityNormalizeIdentity(t *testing.T) {
	ctx, _, svc, accountID := setupInvestmentToolTest(t)
	tool := NewInvestmentToolWithResolver(svc, NewEastMoneyTool(), NewMarketDataTool(svc), stubPortfolioPriceResolver{
		fund: map[string]testQuoteResult{
			"014597": {price: 1.02},
		},
	})

	addResult, err := tool.Execute(ctx, map[string]interface{}{
		"action":     "add_position_by_quantity",
		"account_id": accountID,
		"symbol":     "华泰",
		"name":       "014597",
		"asset_type": "fund",
		"quantity":   1000,
		"avg_cost":   1,
	})
	if err != nil {
		t.Fatalf("add execute: %v", err)
	}
	position, ok := addResult.Output.(investment.Position)
	if !ok {
		t.Fatalf("unexpected add output type: %T", addResult.Output)
	}
	if position.Instrument.Symbol != "014597" {
		t.Fatalf("symbol should be normalized to code, got: %s", position.Instrument.Symbol)
	}
	if position.Instrument.Name != "华泰" {
		t.Fatalf("name should be normalized to display name, got: %s", position.Instrument.Name)
	}

	snapshotResult, err := tool.Execute(ctx, map[string]interface{}{
		"action":     "get_portfolio_snapshot",
		"account_id": accountID,
	})
	if err != nil {
		t.Fatalf("snapshot execute: %v", err)
	}
	snapshot, ok := snapshotResult.Output.(PortfolioSnapshot)
	if !ok {
		t.Fatalf("unexpected snapshot output type: %T", snapshotResult.Output)
	}
	if len(snapshot.Positions) != 1 {
		t.Fatalf("unexpected positions len: %d", len(snapshot.Positions))
	}
	if snapshot.Positions[0].PriceSource != portfolioPriceSourceRealtime {
		t.Fatalf("expected realtime source, got: %s", snapshot.Positions[0].PriceSource)
	}
	assertAlmostEqual(t, snapshot.Positions[0].Cost, 1000)
	assertAlmostEqual(t, snapshot.Positions[0].MarketValue, 1020)
	assertAlmostEqual(t, snapshot.Positions[0].PnL, 20)
}

func TestInvestmentToolSearchInstrumentsIncludesStockAndFund(t *testing.T) {
	ctx, _, svc, _ := setupInvestmentToolTest(t)
	tool := NewInvestmentToolWithResolver(
		svc,
		stubInstrumentSearchProvider{
			searchResults: []SearchItem{
				{Code: "007345", Name: "富国科技创新灵活配置混合", Type: "fund", Price: 1.234},
			},
			stockResult: &SearchItem{
				Code:  "600519",
				Name:  "贵州茅台",
				Type:  "stock",
				Price: 1888.8,
			},
		},
		NewMarketDataTool(svc),
		stubPortfolioPriceResolver{},
	)

	result, err := tool.Execute(ctx, map[string]interface{}{
		"action": "search_instruments",
		"query":  "600519",
		"limit":  10,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	records, ok := result.Output.([]map[string]interface{})
	if !ok {
		t.Fatalf("unexpected output type: %T", result.Output)
	}
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got=%d", len(records))
	}

	first := records[0]
	if first["symbol"] != "600519" {
		t.Fatalf("unexpected stock symbol: %+v", first["symbol"])
	}
	if first["asset_type"] != "stock" {
		t.Fatalf("expected stock asset_type, got=%v", first["asset_type"])
	}
	if first["type"] != "stock" {
		t.Fatalf("expected stock type, got=%v", first["type"])
	}
	if first["name"] != "贵州茅台" {
		t.Fatalf("unexpected stock name: %v", first["name"])
	}

	second := records[1]
	if second["asset_type"] != "fund" {
		t.Fatalf("expected fund asset_type, got=%v", second["asset_type"])
	}
	if second["type"] != "fund" {
		t.Fatalf("expected fund type, got=%v", second["type"])
	}
}

func TestInvestmentToolSearchInstrumentsStockFallbackByCode(t *testing.T) {
	ctx, _, svc, _ := setupInvestmentToolTest(t)
	tool := NewInvestmentToolWithResolver(
		svc,
		stubInstrumentSearchProvider{
			searchResults: nil,
			stockErr:      errors.New("quote_timeout"),
		},
		NewMarketDataTool(svc),
		stubPortfolioPriceResolver{},
	)

	result, err := tool.Execute(ctx, map[string]interface{}{
		"action": "search_instruments",
		"query":  "600519",
		"limit":  10,
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	records, ok := result.Output.([]map[string]interface{})
	if !ok {
		t.Fatalf("unexpected output type: %T", result.Output)
	}
	if len(records) != 1 {
		t.Fatalf("expected 1 fallback stock record, got=%d", len(records))
	}
	if records[0]["symbol"] != "600519" {
		t.Fatalf("unexpected fallback symbol: %v", records[0]["symbol"])
	}
	if records[0]["asset_type"] != "stock" {
		t.Fatalf("unexpected fallback asset_type: %v", records[0]["asset_type"])
	}
	if records[0]["type"] != "stock" {
		t.Fatalf("unexpected fallback type: %v", records[0]["type"])
	}
}
