package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"genFu/internal/db"
	"genFu/internal/investment"
	"genFu/internal/testutil"
)

// TestAllToolsWithStock 测试所有工具使用股票代码的完整功能
func TestAllToolsWithStock(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 使用实际股票代码：浦发��行 600000
	stockCode := "600000"
	stockName := "浦发银行"

	t.Run("EastMoney_GetStockQuote", func(t *testing.T) {
		tool := NewEastMoneyTool()
		result, err := tool.Execute(ctx, map[string]interface{}{
			"action": "get_stock_quote",
			"code":   stockCode,
		})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if result.Error != "" {
			t.Fatalf("Tool error: %s", result.Error)
		}

		quote, ok := result.Output.(StockQuote)
		if !ok {
			t.Fatalf("Unexpected output type: %T", result.Output)
		}

		// 验证输出完整性
		if quote.Code == "" {
			t.Error("Missing code")
		}
		if quote.Name == "" {
			t.Error("Missing name")
		}
		if quote.Price <= 0 {
			t.Errorf("Invalid price: %f", quote.Price)
		}

		t.Logf("Stock Quote - Code: %s, Name: %s, Price: %.2f, Change: %.2f, ChangeRate: %.2f%%, Amount: %.0f",
			quote.Code, quote.Name, quote.Price, quote.Change, quote.ChangeRate, quote.Amount)

		// 输出完整JSON
		data, _ := json.MarshalIndent(quote, "", "  ")
		t.Logf("Full Output:\n%s", string(data))
	})

	t.Run("MarketData_GetStockKline", func(t *testing.T) {
		tool := NewMarketDataTool(nil)
		end := time.Now().Format("20060102")
		start := time.Now().AddDate(0, 0, -7).Format("20060102")

		result, err := tool.Execute(ctx, map[string]interface{}{
			"action": "get_stock_kline",
			"code":   stockCode,
			"start":  start,
			"end":    end,
		})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if result.Error != "" {
			t.Fatalf("Tool error: %s", result.Error)
		}

		points, ok := result.Output.([]KlinePoint)
		if !ok {
			t.Fatalf("Unexpected output type: %T", result.Output)
		}

		if len(points) == 0 {
			t.Fatal("No kline data returned")
		}

		// 验证数据完整性
		for i, p := range points {
			if p.Time == "" {
				t.Errorf("Point %d missing time", i)
			}
			if p.Open <= 0 || p.Close <= 0 || p.High <= 0 || p.Low <= 0 {
				t.Errorf("Point %d has invalid OHLC values", i)
			}
			if p.High < p.Low {
				t.Errorf("Point %d has High < Low", i)
			}
		}

		t.Logf("Got %d kline points for stock %s", len(points), stockCode)
		t.Logf("First point: %+v", points[0])
		t.Logf("Last point: %+v", points[len(points)-1])

		// 输出完整数据摘要
		data, _ := json.MarshalIndent(points[:min(5, len(points))], "", "  ")
		t.Logf("Sample Output (first 5 points):\n%s", string(data))
	})

	t.Run("MarketData_GetStockIntraday", func(t *testing.T) {
		tool := NewMarketDataTool(nil)
		result, err := tool.Execute(ctx, map[string]interface{}{
			"action": "get_stock_intraday",
			"code":   stockCode,
			"days":   1,
		})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}
		if result.Error != "" {
			t.Fatalf("Tool error: %s", result.Error)
		}

		points, ok := result.Output.([]IntradayPoint)
		if !ok {
			t.Fatalf("Unexpected output type: %T", result.Output)
		}

		if len(points) == 0 {
			t.Fatal("No intraday data returned")
		}

		// 验证数据完整性
		for i, p := range points {
			if p.Time == "" {
				t.Errorf("Point %d missing time", i)
			}
			if p.Price <= 0 {
				t.Errorf("Point %d has invalid price", i)
			}
		}

		t.Logf("Got %d intraday points for stock %s", len(points), stockCode)
		t.Logf("First point: %+v", points[0])
		t.Logf("Last point: %+v", points[len(points)-1])

		// 输出完整数据摘要
		data, _ := json.MarshalIndent(points[:min(10, len(points))], "", "  ")
		t.Logf("Sample Output (first 10 points):\n%s", string(data))
	})

	// 测试投资工具与股票集成
	t.Run("Investment_WithStock", func(t *testing.T) {
		// 设置数据库
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
		defer func() {
			_ = os.Chdir(wd)
		}()

		if err := db.ApplyMigrations(context.Background(), conn); err != nil {
			t.Fatalf("migrations: %v", err)
		}

		repo := investment.NewRepository(conn)
		svc := investment.NewService(repo)
		tool := NewInvestmentTool(svc)

		// 创建用户和账户
		user, err := svc.CreateUser(ctx, "test_user")
		if err != nil {
			t.Fatalf("CreateUser failed: %v", err)
		}
		t.Logf("Created user: %+v", user)

		account, err := svc.CreateAccount(ctx, user.ID, "test_account", "CNY")
		if err != nil {
			t.Fatalf("CreateAccount failed: %v", err)
		}
		t.Logf("Created account: %+v", account)

		// 创建股票工具
		instrument, err := svc.UpsertInstrument(ctx, stockCode, stockName, "stock")
		if err != nil {
			t.Fatalf("UpsertInstrument failed: %v", err)
		}
		t.Logf("Created instrument: %+v", instrument)

		// 设置持仓
		position, err := svc.SetPosition(ctx, account.ID, instrument.ID, 1000, 10.5, nil)
		if err != nil {
			t.Fatalf("SetPosition failed: %v", err)
		}
		t.Logf("Set position: %+v", position)

		// 查询持仓
		result, err := tool.Execute(ctx, map[string]interface{}{
			"action":     "list_positions",
			"account_id": account.ID,
		})
		if err != nil {
			t.Fatalf("Execute list_positions failed: %v", err)
		}
		if result.Error != "" {
			t.Fatalf("Tool error: %s", result.Error)
		}

		positions, ok := result.Output.([]investment.Position)
		if !ok {
			t.Fatalf("Unexpected output type: %T", result.Output)
		}

		if len(positions) == 0 {
			t.Fatal("No positions returned")
		}

		t.Logf("Got %d positions", len(positions))
		data, _ := json.MarshalIndent(positions, "", "  ")
		t.Logf("Full Output:\n%s", string(data))

		// 获取投资组合摘要
		summary, err := tool.Execute(ctx, map[string]interface{}{
			"action":     "get_portfolio_summary",
			"account_id": account.ID,
		})
		if err != nil {
			t.Fatalf("Execute get_portfolio_summary failed: %v", err)
		}
		if summary.Error != "" {
			t.Fatalf("Tool error: %s", summary.Error)
		}

		data, _ = json.MarshalIndent(summary.Output, "", "  ")
		t.Logf("Portfolio Summary:\n%s", string(data))
	})
}

// ToolTestResult 工具测试结果
type ToolTestResult struct {
	ToolName    string
	Function    string
	Description string
	Success     bool
	Error       string
	DataPoints  int
}

// TestAllToolsWithFund 测试所有工具使用基金代码的完整功能
func TestAllToolsWithFund(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 使用实际基金代码：华夏成长 000001
	fundCode := "000001"
	fundName := "华夏成长"

	// 测试结果收集
	var results []ToolTestResult

	t.Log("\n" + strings.Repeat("=", 80))
	t.Log("基金工具集成测试开始")
	t.Logf("测试目标: 基金代码 %s (%s)", fundCode, fundName)
	t.Log(strings.Repeat("=", 80))

	// 1. 测试 MarketData 工具 - 获取基金历史净值
	t.Run("MarketData_GetFundKline", func(t *testing.T) {
		result := ToolTestResult{
			ToolName:    "marketdata",
			Function:    "get_fund_kline",
			Description: "获取基金历史净值数据（K线），包括日期、单位净值、累计净值等",
		}

		defer func() {
			results = append(results, result)
			if result.Success {
				t.Logf("✓ [%s] %s - 成功获取 %d 条数据", result.ToolName, result.Description, result.DataPoints)
			} else {
				t.Logf("✗ [%s] %s - 失败: %s", result.ToolName, result.Description, result.Error)
			}
		}()

		tool := NewMarketDataTool(nil)
		end := time.Now().Format("2006-01-02")
		start := time.Now().AddDate(0, -1, 0).Format("2006-01-02")

		toolResult, err := tool.Execute(ctx, map[string]interface{}{
			"action": "get_fund_kline",
			"code":   fundCode,
			"start":  start,
			"end":    end,
		})
		if err != nil {
			result.Error = err.Error()
			return
		}
		if toolResult.Error != "" {
			result.Error = toolResult.Error
			return
		}

		points, ok := toolResult.Output.([]KlinePoint)
		if !ok {
			result.Error = fmt.Sprintf("输出类型错误: %T", toolResult.Output)
			return
		}

		if len(points) == 0 {
			result.Error = "未返回任何数据"
			return
		}

		// 验证数据完整性
		for i, p := range points {
			if p.Time == "" {
				result.Error = fmt.Sprintf("第 %d 条数据缺少时间字段", i)
				return
			}
			if p.Close <= 0 {
				result.Error = fmt.Sprintf("第 %d 条数据净值无效: %f", i, p.Close)
				return
			}
		}

		result.Success = true
		result.DataPoints = len(points)

		// 输出详细数据
		t.Logf("\n功能说明: %s", result.Description)
		t.Logf("数据时间范围: %s 至 %s", points[0].Time, points[len(points)-1].Time)
		t.Logf("获取数据点数: %d", len(points))

		data, _ := json.MarshalIndent(points[:min(5, len(points))], "", "  ")
		t.Logf("示例数据 (前5条):\n%s", string(data))
	})

	// 2. 测试 MarketData 工具 - 获取基金实时净值
	t.Run("MarketData_GetFundIntraday", func(t *testing.T) {
		result := ToolTestResult{
			ToolName:    "marketdata",
			Function:    "get_fund_intraday",
			Description: "获取基金实时估值数据，包括估算净值、实际净值、估值时间等",
		}

		defer func() {
			results = append(results, result)
			if result.Success {
				t.Logf("✓ [%s] %s - 成功获取 %d 条数据", result.ToolName, result.Description, result.DataPoints)
			} else {
				t.Logf("✗ [%s] %s - 失败: %s", result.ToolName, result.Description, result.Error)
			}
		}()

		tool := NewMarketDataTool(nil)
		toolResult, err := tool.Execute(ctx, map[string]interface{}{
			"action": "get_fund_intraday",
			"code":   fundCode,
		})
		if err != nil {
			result.Error = err.Error()
			return
		}
		if toolResult.Error != "" {
			result.Error = toolResult.Error
			return
		}

		points, ok := toolResult.Output.([]IntradayPoint)
		if !ok {
			result.Error = fmt.Sprintf("输出类型错误: %T", toolResult.Output)
			return
		}

		if len(points) == 0 {
			result.Error = "未返回任何数据"
			return
		}

		// 验证数据完整性
		for i, p := range points {
			if p.Price <= 0 {
				result.Error = fmt.Sprintf("第 %d 条数据价格无效: %f", i, p.Price)
				return
			}
		}

		result.Success = true
		result.DataPoints = len(points)

		// 输出详细数据
		t.Logf("\n功能说明: %s", result.Description)
		for i, p := range points {
			t.Logf("数据点 %d: 时间=%s, 估值=%.4f, 实际净值=%.4f", i, p.Time, p.Price, p.AvgPrice)
		}

		data, _ := json.MarshalIndent(points, "", "  ")
		t.Logf("完整数据:\n%s", string(data))
	})

	// 3. 测试 Investment 工具 - 基金持仓管理
	t.Run("Investment_WithFund", func(t *testing.T) {
		// 设置数据库
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
			t.Fatalf("数据库初始化失败: %v", err)
		}

		wd, err := os.Getwd()
		if err != nil {
			t.Fatalf("获取工作目录失败: %v", err)
		}
		if err := os.Chdir(filepath.Join(wd, "..", "..")); err != nil {
			t.Fatalf("切换目录失败: %v", err)
		}
		defer func() {
			_ = os.Chdir(wd)
		}()

		if err := db.ApplyMigrations(context.Background(), conn); err != nil {
			t.Fatalf("数据库迁移失败: %v", err)
		}

		repo := investment.NewRepository(conn)
		svc := investment.NewService(repo)
		tool := NewInvestmentTool(svc)

		// 3.1 创建用户
		createUserResult := ToolTestResult{
			ToolName:    "investment",
			Function:    "create_user",
			Description: "创建投资用户账户",
		}
		user, err := svc.CreateUser(ctx, "test_user")
		if err != nil {
			createUserResult.Error = err.Error()
			results = append(results, createUserResult)
			t.Logf("✗ [investment] %s - 失败: %s", createUserResult.Description, createUserResult.Error)
		} else {
			createUserResult.Success = true
			createUserResult.DataPoints = 1
			results = append(results, createUserResult)
			t.Logf("✓ [investment] %s - 成功 (用户ID: %d)", createUserResult.Description, user.ID)
		}

		// 3.2 创建账户
		createAccountResult := ToolTestResult{
			ToolName:    "investment",
			Function:    "create_account",
			Description: "创建投资账户，指定基础货币",
		}
		account, err := svc.CreateAccount(ctx, user.ID, "test_account", "CNY")
		if err != nil {
			createAccountResult.Error = err.Error()
			results = append(results, createAccountResult)
			t.Logf("✗ [investment] %s - 失败: %s", createAccountResult.Description, createAccountResult.Error)
		} else {
			createAccountResult.Success = true
			createAccountResult.DataPoints = 1
			results = append(results, createAccountResult)
			t.Logf("✓ [investment] %s - 成功 (账户ID: %d)", createAccountResult.Description, account.ID)
		}

		// 3.3 创建基金工具
		upsertInstrumentResult := ToolTestResult{
			ToolName:    "investment",
			Function:    "upsert_instrument",
			Description: "创建或更新投资工具（基金、股票等）",
		}
		instrument, err := svc.UpsertInstrument(ctx, fundCode, fundName, "fund")
		if err != nil {
			upsertInstrumentResult.Error = err.Error()
			results = append(results, upsertInstrumentResult)
			t.Logf("✗ [investment] %s - 失败: %s", upsertInstrumentResult.Description, upsertInstrumentResult.Error)
		} else {
			upsertInstrumentResult.Success = true
			upsertInstrumentResult.DataPoints = 1
			results = append(results, upsertInstrumentResult)
			t.Logf("✓ [investment] %s - 成功 (工具ID: %d, 代码: %s)", upsertInstrumentResult.Description, instrument.ID, instrument.Symbol)
		}

		// 3.4 设置持仓
		setPositionResult := ToolTestResult{
			ToolName:    "investment",
			Function:    "set_position",
			Description: "设置基金持仓数量和成本",
		}
		position, err := svc.SetPosition(ctx, account.ID, instrument.ID, 5000, 1.5, nil)
		if err != nil {
			setPositionResult.Error = err.Error()
			results = append(results, setPositionResult)
			t.Logf("✗ [investment] %s - 失败: %s", setPositionResult.Description, setPositionResult.Error)
		} else {
			setPositionResult.Success = true
			setPositionResult.DataPoints = 1
			results = append(results, setPositionResult)
			t.Logf("✓ [investment] %s - 成功 (数量: %.0f, 成本: %.2f)", setPositionResult.Description, position.Quantity, position.AvgCost)
		}

		// 3.5 查询基金持仓
		listFundHoldingsResult := ToolTestResult{
			ToolName:    "investment",
			Function:    "list_fund_holdings",
			Description: "查询账户中的所有基金持仓",
		}
		holdingsResult, err := tool.Execute(ctx, map[string]interface{}{
			"action":     "list_fund_holdings",
			"account_id": account.ID,
		})
		if err != nil {
			listFundHoldingsResult.Error = err.Error()
			results = append(results, listFundHoldingsResult)
			t.Logf("✗ [investment] %s - 失败: %s", listFundHoldingsResult.Description, listFundHoldingsResult.Error)
		} else if holdingsResult.Error != "" {
			listFundHoldingsResult.Error = holdingsResult.Error
			results = append(results, listFundHoldingsResult)
			t.Logf("✗ [investment] %s - 失败: %s", listFundHoldingsResult.Description, listFundHoldingsResult.Error)
		} else {
			positions, ok := holdingsResult.Output.([]investment.Position)
			if !ok {
				listFundHoldingsResult.Error = fmt.Sprintf("输出类型错误: %T", holdingsResult.Output)
				results = append(results, listFundHoldingsResult)
				t.Logf("✗ [investment] %s - 失败: %s", listFundHoldingsResult.Description, listFundHoldingsResult.Error)
			} else {
				listFundHoldingsResult.Success = true
				listFundHoldingsResult.DataPoints = len(positions)
				results = append(results, listFundHoldingsResult)
				t.Logf("✓ [investment] %s - 成功 (持仓数: %d)", listFundHoldingsResult.Description, len(positions))

				data, _ := json.MarshalIndent(positions, "", "  ")
				t.Logf("持仓详情:\n%s", string(data))
			}
		}

		// 3.6 记录交易
		recordTradeResult := ToolTestResult{
			ToolName:    "investment",
			Function:    "record_trade",
			Description: "记录基金买入/卖出交易",
		}
		tradeResult, err := tool.Execute(ctx, map[string]interface{}{
			"action":        "record_trade",
			"account_id":    account.ID,
			"instrument_id": instrument.ID,
			"side":          "buy",
			"quantity":      1000,
			"price":         1.6,
			"fee":           5.0,
		})
		if err != nil {
			recordTradeResult.Error = err.Error()
			results = append(results, recordTradeResult)
			t.Logf("✗ [investment] %s - 失败: %s", recordTradeResult.Description, recordTradeResult.Error)
		} else if tradeResult.Error != "" {
			recordTradeResult.Error = tradeResult.Error
			results = append(results, recordTradeResult)
			t.Logf("✗ [investment] %s - 失败: %s", recordTradeResult.Description, recordTradeResult.Error)
		} else {
			recordTradeResult.Success = true
			recordTradeResult.DataPoints = 1
			results = append(results, recordTradeResult)
			t.Logf("✓ [investment] %s - 成功 (买入 1000 份，价格 1.6)", recordTradeResult.Description)

			data, _ := json.MarshalIndent(tradeResult.Output, "", "  ")
			t.Logf("交易结果:\n%s", string(data))
		}

		// 3.7 查询交易记录
		listTradesResult := ToolTestResult{
			ToolName:    "investment",
			Function:    "list_trades",
			Description: "查询账户交易历史记录",
		}
		tradesResult, err := tool.Execute(ctx, map[string]interface{}{
			"action":     "list_trades",
			"account_id": account.ID,
			"limit":      10,
		})
		if err != nil {
			listTradesResult.Error = err.Error()
			results = append(results, listTradesResult)
			t.Logf("✗ [investment] %s - 失败: %s", listTradesResult.Description, listTradesResult.Error)
		} else if tradesResult.Error != "" {
			listTradesResult.Error = tradesResult.Error
			results = append(results, listTradesResult)
			t.Logf("✗ [investment] %s - 失败: %s", listTradesResult.Description, listTradesResult.Error)
		} else {
			listTradesResult.Success = true
			data, _ := json.MarshalIndent(tradesResult.Output, "", "  ")
			var trades []interface{}
			_ = json.Unmarshal(data, &trades)
			listTradesResult.DataPoints = len(trades)
			results = append(results, listTradesResult)
			t.Logf("✓ [investment] %s - 成功 (交易记录数: %d)", listTradesResult.Description, listTradesResult.DataPoints)

			t.Logf("交易记录:\n%s", string(data))
		}

		// 3.8 获取投资组合摘要
		getPortfolioSummaryResult := ToolTestResult{
			ToolName:    "investment",
			Function:    "get_portfolio_summary",
			Description: "获取投资组合摘要，包括总资产、总成本、收益率等",
		}
		summaryResult, err := tool.Execute(ctx, map[string]interface{}{
			"action":     "get_portfolio_summary",
			"account_id": account.ID,
		})
		if err != nil {
			getPortfolioSummaryResult.Error = err.Error()
			results = append(results, getPortfolioSummaryResult)
			t.Logf("✗ [investment] %s - 失败: %s", getPortfolioSummaryResult.Description, getPortfolioSummaryResult.Error)
		} else if summaryResult.Error != "" {
			getPortfolioSummaryResult.Error = summaryResult.Error
			results = append(results, getPortfolioSummaryResult)
			t.Logf("✗ [investment] %s - 失败: %s", getPortfolioSummaryResult.Description, getPortfolioSummaryResult.Error)
		} else {
			getPortfolioSummaryResult.Success = true
			getPortfolioSummaryResult.DataPoints = 1
			results = append(results, getPortfolioSummaryResult)
			t.Logf("✓ [investment] %s - 成功", getPortfolioSummaryResult.Description)

			data, _ := json.MarshalIndent(summaryResult.Output, "", "  ")
			t.Logf("投资组合摘要:\n%s", string(data))
		}
	})

	// 输出测试总结报告
	t.Log("\n" + strings.Repeat("=", 80))
	t.Log("测试总结报告")
	t.Log(strings.Repeat("=", 80))

	// 按工具分组统计
	toolStats := make(map[string]struct {
		success int
		total   int
		failed  []string
	})

	for _, r := range results {
		stats := toolStats[r.ToolName]
		stats.total++
		if r.Success {
			stats.success++
		} else {
			stats.failed = append(stats.failed, r.Function)
		}
		toolStats[r.ToolName] = stats
	}

	// 输出统计结果
	totalTests := 0
	totalSuccess := 0
	totalFailed := 0

	for toolName, stats := range toolStats {
		totalTests += stats.total
		totalSuccess += stats.success
		totalFailed += stats.total - stats.success

		t.Logf("\n[%s] %d/%d 功能可用", toolName, stats.success, stats.total)
		if len(stats.failed) > 0 {
			t.Logf("  不可用功能: %s", strings.Join(stats.failed, ", "))
		}

		// 输出每个功能的详细结果
		t.Logf("  详细结果:")
		for _, r := range results {
			if r.ToolName == toolName {
				if r.Success {
					t.Logf("    ✓ %s (%s) - 数据点: %d", r.Function, r.Description, r.DataPoints)
				} else {
					t.Logf("    ✗ %s (%s) - 错误: %s", r.Function, r.Description, r.Error)
				}
			}
		}
	}

	t.Log("\n" + strings.Repeat("-", 80))
	t.Logf("总计: %d/%d 功能可用", totalSuccess, totalTests)
	t.Logf("成功: %d, 失败: %d", totalSuccess, totalFailed)
	t.Log(strings.Repeat("=", 80))

	// 如果有失败的测试，标记为失败
	if totalFailed > 0 {
		t.Errorf("有 %d 个功能测试失败", totalFailed)
	}
}

// TestRegistryAllTools 测试注册表中的所有工具
func TestRegistryAllTools(t *testing.T) {
	// 设置数据库
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
	defer func() {
		_ = os.Chdir(wd)
	}()

	if err := db.ApplyMigrations(context.Background(), conn); err != nil {
		t.Fatalf("migrations: %v", err)
	}

	repo := investment.NewRepository(conn)
	svc := investment.NewService(repo)

	// 创建注册表并注册所有工具
	registry := NewRegistry()
	registry.Register(NewEastMoneyTool())
	registry.Register(NewMarketDataTool(svc))
	registry.Register(NewInvestmentTool(svc))

	// 列出所有工具
	tools := registry.List()
	t.Logf("Registered %d tools:", len(tools))
	for _, spec := range tools {
		t.Logf("  - %s: %s", spec.Name, spec.Description)
		t.Logf("    Required params: %v", spec.Required)
		t.Logf("    Optional params: %v", spec.Params)
	}

	// 测试每个工具的基本功能
	ctx := context.Background()

	t.Run("EastMoney_Tool", func(t *testing.T) {
		tool, ok := registry.Get("eastmoney")
		if !ok {
			t.Fatal("Tool not found")
		}

		result, err := tool.Execute(ctx, map[string]interface{}{
			"action": "get_stock_quote",
			"code":   "600000",
		})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		resultData, _ := json.MarshalIndent(result, "", "  ")
		t.Logf("Result:\n%s", string(resultData))
	})

	t.Run("MarketData_Tool", func(t *testing.T) {
		tool, ok := registry.Get("marketdata")
		if !ok {
			t.Fatal("Tool not found")
		}

		result, err := tool.Execute(ctx, map[string]interface{}{
			"action": "get_stock_intraday",
			"code":   "600000",
			"days":   1,
		})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		t.Logf("Result (truncated):")
		if output, ok := result.Output.([]IntradayPoint); ok && len(output) > 0 {
			t.Logf("  Got %d data points", len(output))
			t.Logf("  First point: %+v", output[0])
		}
	})

	t.Run("Investment_Tool", func(t *testing.T) {
		tool, ok := registry.Get("investment")
		if !ok {
			t.Fatal("Tool not found")
		}

		result, err := tool.Execute(ctx, map[string]interface{}{
			"action": "help",
		})
		if err != nil {
			t.Fatalf("Execute failed: %v", err)
		}

		data, _ := json.MarshalIndent(result, "", "  ")
		t.Logf("Result:\n%s", string(data))
	})
}

// TestToolErrorHandling 测试工具错误处理
func TestToolErrorHandling(t *testing.T) {
	ctx := context.Background()

	t.Run("EastMoney_InvalidCode", func(t *testing.T) {
		tool := NewEastMoneyTool()
		result, err := tool.Execute(ctx, map[string]interface{}{
			"action": "get_stock_quote",
			"code":   "INVALID123",
		})

		// 工具可能不会返回错误，但会在结果中包含错误信息
		if result.Error != "" {
			t.Logf("Expected error received: %s", result.Error)
		} else if err != nil {
			t.Logf("Expected error received: %v", err)
		} else {
			t.Log("No error for invalid code (may be expected)")
		}
	})

	t.Run("MarketData_InvalidAction", func(t *testing.T) {
		tool := NewMarketDataTool(nil)
		result, err := tool.Execute(ctx, map[string]interface{}{
			"action": "invalid_action",
		})

		if err == nil {
			t.Fatal("Expected error for invalid action")
		}
		t.Logf("Expected error: %v", err)

		if result.Error == "" {
			t.Fatal("Expected error in result")
		}
		t.Logf("Result error: %s", result.Error)
	})

	t.Run("Investment_MissingAction", func(t *testing.T) {
		tool := NewInvestmentTool(nil)
		result, err := tool.Execute(ctx, map[string]interface{}{})

		if err == nil {
			t.Fatal("Expected error for missing action")
		}
		t.Logf("Expected error: %v", err)

		if result.Error == "" {
			t.Fatal("Expected error in result")
		}
		t.Logf("Result error: %s", result.Error)
	})

	t.Run("Investment_UnsupportedAction", func(t *testing.T) {
		tool := NewInvestmentTool(nil)
		result, err := tool.Execute(ctx, map[string]interface{}{
			"action": "unsupported_action",
		})

		if err == nil {
			t.Fatal("Expected error for unsupported action")
		}
		t.Logf("Expected error: %v", err)

		if result.Error == "" {
			t.Fatal("Expected error in result")
		}
		t.Logf("Result error: %s", result.Error)
	})
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// TestMain 用于设置测试环境
func TestMain(m *testing.M) {
	// 可以在这里添加全局设置
	fmt.Println("Starting tool integration tests...")
	code := m.Run()
	fmt.Println("Tool integration tests completed")
	os.Exit(code)
}
