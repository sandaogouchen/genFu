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

// TestAllToolsWithStock_Complete 测试所有工具使用股票代码的完整功能
func TestAllToolsWithStock_Complete(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 使用实际股票代码：浦发银行 600000
	stockCode := "600000"
	stockName := "浦发银行"

	// 测试结果收集
	var results []ToolTestResult

	t.Log("\n" + strings.Repeat("=", 80))
	t.Log("股票工具集成测���开始")
	t.Logf("测试目标: 股票代码 %s (%s)", stockCode, stockName)
	t.Log(strings.Repeat("=", 80))

	// 1. 测试 EastMoney 工具 - 获取股票行情
	t.Run("EastMoney_GetStockQuote", func(t *testing.T) {
		result := ToolTestResult{
			ToolName:    "eastmoney",
			Function:    "get_stock_quote",
			Description: "获取股票实时行情，包括价格、涨跌额、涨跌幅、成交额等",
		}

		defer func() {
			results = append(results, result)
			if result.Success {
				t.Logf("✓ [%s] %s - 成功获取 %d 条数据", result.ToolName, result.Description, result.DataPoints)
			} else {
				t.Logf("✗ [%s] %s - 失败: %s", result.ToolName, result.Description, result.Error)
			}
		}()

		tool := NewEastMoneyTool()
		toolResult, err := tool.Execute(ctx, map[string]interface{}{
			"action": "get_stock_quote",
			"code":   stockCode,
		})
		if err != nil {
			result.Error = err.Error()
			return
		}
		if toolResult.Error != "" {
			result.Error = toolResult.Error
			return
		}

		quote, ok := toolResult.Output.(StockQuote)
		if !ok {
			result.Error = fmt.Sprintf("输出类型错误: %T", toolResult.Output)
			return
		}

		// 验证输出完整性
		if quote.Code == "" {
			result.Error = "缺少股票代码"
			return
		}
		if quote.Name == "" {
			result.Error = "缺少股票名称"
			return
		}
		if quote.Price <= 0 {
			result.Error = fmt.Sprintf("价格无效: %f", quote.Price)
			return
		}

		result.Success = true
		result.DataPoints = 1

		// 输出详细数据
		t.Logf("\n功能说明: %s", result.Description)
		t.Logf("股票代码: %s, 名称: %s", quote.Code, quote.Name)
		t.Logf("当前价: %.2f, 涨跌: %.2f, 涨跌幅: %.2f%%", quote.Price, quote.Change, quote.ChangeRate)
		t.Logf("成交额: %.0f", quote.Amount)

		data, _ := json.MarshalIndent(quote, "", "  ")
		t.Logf("完整数据:\n%s", string(data))
	})

	// 2. 测试 MarketData 工具 - 获取股票K线
	t.Run("MarketData_GetStockKline", func(t *testing.T) {
		result := ToolTestResult{
			ToolName:    "marketdata",
			Function:    "get_stock_kline",
			Description: "获取股票历史K线数据，包括开盘价、收盘价、最高价、最低价、成交量等",
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
		end := time.Now().Format("20060102")
		start := time.Now().AddDate(0, 0, -7).Format("20060102")

		toolResult, err := tool.Execute(ctx, map[string]interface{}{
			"action": "get_stock_kline",
			"code":   stockCode,
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
			if p.Open <= 0 || p.Close <= 0 || p.High <= 0 || p.Low <= 0 {
				result.Error = fmt.Sprintf("第 %d 条数据OHLC值无效", i)
				return
			}
			if p.High < p.Low {
				result.Error = fmt.Sprintf("第 %d 条数据最高价小于最低价", i)
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

	// 3. 测试 MarketData 工具 - 获取股票分时数据
	t.Run("MarketData_GetStockIntraday", func(t *testing.T) {
		result := ToolTestResult{
			ToolName:    "marketdata",
			Function:    "get_stock_intraday",
			Description: "获取股票分时行情数据，包括每分钟价格、成交量、均价等",
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
			"action": "get_stock_intraday",
			"code":   stockCode,
			"days":   1,
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
			if p.Time == "" {
				result.Error = fmt.Sprintf("第 %d 条数据缺少时间字段", i)
				return
			}
			if p.Price <= 0 {
				result.Error = fmt.Sprintf("第 %d 条数据价格无效: %f", i, p.Price)
				return
			}
		}

		result.Success = true
		result.DataPoints = len(points)

		// 输出详细数据
		t.Logf("\n功能说明: %s", result.Description)
		t.Logf("获取数据点数: %d", len(points))

		data, _ := json.MarshalIndent(points[:min(10, len(points))], "", "  ")
		t.Logf("示例数据 (前10条):\n%s", string(data))
	})

	// 4. 测试 Investment 工具 - 股票持仓管理
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

		// 4.1 创建用户
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

		// 4.2 创建账户
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

		// 4.3 创建股票工具
		upsertInstrumentResult := ToolTestResult{
			ToolName:    "investment",
			Function:    "upsert_instrument",
			Description: "创建或更新投资工具（股票）",
		}
		instrument, err := svc.UpsertInstrument(ctx, stockCode, stockName, "stock")
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

		// 4.4 设置持仓
		setPositionResult := ToolTestResult{
			ToolName:    "investment",
			Function:    "set_position",
			Description: "设置股票持仓数量和成本",
		}
		position, err := svc.SetPosition(ctx, account.ID, instrument.ID, 1000, 10.5, nil)
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

		// 4.5 查询持仓
		listPositionsResult := ToolTestResult{
			ToolName:    "investment",
			Function:    "list_positions",
			Description: "查询账户中的所有持仓",
		}
		positionsResult, err := tool.Execute(ctx, map[string]interface{}{
			"action":     "list_positions",
			"account_id": account.ID,
		})
		if err != nil {
			listPositionsResult.Error = err.Error()
			results = append(results, listPositionsResult)
			t.Logf("✗ [investment] %s - 失败: %s", listPositionsResult.Description, listPositionsResult.Error)
		} else if positionsResult.Error != "" {
			listPositionsResult.Error = positionsResult.Error
			results = append(results, listPositionsResult)
			t.Logf("✗ [investment] %s - 失败: %s", listPositionsResult.Description, listPositionsResult.Error)
		} else {
			positions, ok := positionsResult.Output.([]investment.Position)
			if !ok {
				listPositionsResult.Error = fmt.Sprintf("输出类型错误: %T", positionsResult.Output)
				results = append(results, listPositionsResult)
				t.Logf("✗ [investment] %s - 失败: %s", listPositionsResult.Description, listPositionsResult.Error)
			} else {
				listPositionsResult.Success = true
				listPositionsResult.DataPoints = len(positions)
				results = append(results, listPositionsResult)
				t.Logf("✓ [investment] %s - 成功 (持仓数: %d)", listPositionsResult.Description, len(positions))

				data, _ := json.MarshalIndent(positions, "", "  ")
				t.Logf("持仓详情:\n%s", string(data))
			}
		}

		// 4.6 获取投资组合摘要
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
