package tool

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"genFu/internal/dashboard"
)

// DashboardTool implements the Tool interface for dashboard generation.
type DashboardTool struct {
	dataSvc   *dashboard.DataService
	generator *dashboard.HTMLGenerator
	outputDir string
}

// NewDashboardTool creates a new DashboardTool.
func NewDashboardTool(dataSvc *dashboard.DataService, generator *dashboard.HTMLGenerator, outputDir string) *DashboardTool {
	return &DashboardTool{
		dataSvc:   dataSvc,
		generator: generator,
		outputDir: outputDir,
	}
}

// Spec returns the tool specification for LLM agent integration.
func (t *DashboardTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "dashboard",
		Description: "生成持仓仪表盘，包括盈亏热力图、资产配置图、KPI 面板和估值趋势图。支持 HTML 仪表盘生成、结构化数据获取和盈亏汇总。",
		Params: map[string]string{
			"action":       "操作类型：generate_dashboard | get_dashboard_data | get_pnl_summary | get_treemap_data（必需）",
			"account_id":   "账户 ID（可选，默认使用系统默认账户）",
			"color_scheme": "配色方案：cn（红涨绿跌）| us（绿涨红跌）（可选，默认 cn）",
			"days":         "估值历史天数（可选，默认 30）",
			"group_by":     "分组方式：industry | asset_type（可选，默认 industry）",
			"include_cash": "是否包含现金：true | false（可选，默认 true）",
		},
		Required: []string{"action"},
	}
}

// Execute runs the dashboard tool action.
func (t *DashboardTool) Execute(ctx context.Context, args map[string]string) (ToolResult, error) {
	action := args["action"]
	switch action {
	case "generate_dashboard":
		return t.generateDashboard(ctx, args)
	case "get_dashboard_data":
		return t.getDashboardData(ctx, args)
	case "get_pnl_summary":
		return t.getPnLSummary(ctx, args)
	case "get_treemap_data":
		return t.getTreeMapData(ctx, args)
	default:
		return ToolResult{
			Success: false,
			Error:   fmt.Sprintf("未知操作: %s, 支持的操作: generate_dashboard, get_dashboard_data, get_pnl_summary, get_treemap_data", action),
		}, nil
	}
}

// buildOptions constructs DashboardOptions from args map.
func (t *DashboardTool) buildOptions(args map[string]string) dashboard.DashboardOptions {
	opts := dashboard.DashboardOptions{
		ValuationDays: 30,
		ColorScheme:   "cn",
		IncludeCash:   true,
		GroupBy:       "industry",
	}
	if days, ok := args["days"]; ok {
		if d, err := strconv.Atoi(days); err == nil && d > 0 {
			opts.ValuationDays = d
		}
	}
	if scheme, ok := args["color_scheme"]; ok && (scheme == "cn" || scheme == "us") {
		opts.ColorScheme = scheme
	}
	if cash, ok := args["include_cash"]; ok && cash == "false" {
		opts.IncludeCash = false
	}
	if groupBy, ok := args["group_by"]; ok {
		opts.GroupBy = groupBy
	}
	return opts
}

// parseAccountID extracts account_id from args, returns 0 if not set.
func (t *DashboardTool) parseAccountID(args map[string]string) int64 {
	if s, ok := args["account_id"]; ok {
		if id, err := strconv.ParseInt(s, 10, 64); err == nil {
			return id
		}
	}
	return 0
}

// generateDashboard builds and writes an HTML dashboard file.
func (t *DashboardTool) generateDashboard(ctx context.Context, args map[string]string) (ToolResult, error) {
	accountID := t.parseAccountID(args)
	opts := t.buildOptions(args)

	data, err := t.dataSvc.BuildDashboardData(ctx, accountID, opts)
	if err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("构建仪表盘数据失败: %v", err)}, nil
	}

	htmlContent, err := t.generator.GenerateHTMLString(data)
	if err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("渲染 HTML 失败: %v", err)}, nil
	}

	// Ensure output directory exists.
	outDir := t.outputDir
	if outDir == "" {
		outDir = "output"
	}
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("创建输出目录失败: %v", err)}, nil
	}

	filename := fmt.Sprintf("dashboard_%d_%s.html", data.AccountID, time.Now().Format("20060102_150405"))
	filePath := filepath.Join(outDir, filename)
	if err := os.WriteFile(filePath, []byte(htmlContent), 0644); err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("写入文件失败: %v", err)}, nil
	}

	absPath, _ := filepath.Abs(filePath)

	summary := fmt.Sprintf("仪表盘已生成\n文件路径: %s\n账户: %d\n持仓数: %d\n总资产: %.2f\n总盈亏: %.2f (%.2f%%)\n配色: %s",
		absPath, data.AccountID, data.Summary.PositionCount,
		data.Summary.TotalValue, data.Summary.TotalPnL, data.Summary.TotalPnLPct*100,
		opts.ColorScheme)

	return ToolResult{
		Success: true,
		Data: map[string]interface{}{
			"file_path":      absPath,
			"filename":       filename,
			"account_id":     data.AccountID,
			"position_count": data.Summary.PositionCount,
			"total_value":    data.Summary.TotalValue,
			"total_pnl":      data.Summary.TotalPnL,
			"total_pnl_pct":  data.Summary.TotalPnLPct,
		},
		Message: summary,
	}, nil
}

// getDashboardData returns the full structured data.
func (t *DashboardTool) getDashboardData(ctx context.Context, args map[string]string) (ToolResult, error) {
	accountID := t.parseAccountID(args)
	opts := t.buildOptions(args)

	data, err := t.dataSvc.BuildDashboardData(ctx, accountID, opts)
	if err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("构建数据失败: %v", err)}, nil
	}

	jsonBytes, _ := json.MarshalIndent(data, "", "  ")
	return ToolResult{
		Success: true,
		Data:    data,
		Message: string(jsonBytes),
	}, nil
}

// getPnLSummary returns only the KPI summary.
func (t *DashboardTool) getPnLSummary(ctx context.Context, args map[string]string) (ToolResult, error) {
	accountID := t.parseAccountID(args)
	opts := t.buildOptions(args)

	data, err := t.dataSvc.BuildDashboardData(ctx, accountID, opts)
	if err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("构建数据失败: %v", err)}, nil
	}

	summary := data.Summary
	msg := fmt.Sprintf("持仓概览\n总资产: %.2f\n总成本: %.2f\n总盈亏: %.2f (%.2f%%)\n今日盈亏: %.2f (%.2f%%)\n持仓数: %d\n现金余额: %.2f",
		summary.TotalValue, summary.TotalCost, summary.TotalPnL, summary.TotalPnLPct*100,
		summary.DailyPnL, summary.DailyPnLPct*100, summary.PositionCount, summary.CashBalance)

	return ToolResult{
		Success: true,
		Data:    summary,
		Message: msg,
	}, nil
}

// getTreeMapData returns treemap-ready hierarchical data.
func (t *DashboardTool) getTreeMapData(ctx context.Context, args map[string]string) (ToolResult, error) {
	accountID := t.parseAccountID(args)
	opts := t.buildOptions(args)

	data, err := t.dataSvc.BuildDashboardData(ctx, accountID, opts)
	if err != nil {
		return ToolResult{Success: false, Error: fmt.Sprintf("构建数据失败: %v", err)}, nil
	}

	treemapData := make([]map[string]interface{}, 0, len(data.IndustryBreak))
	for _, ig := range data.IndustryBreak {
		children := make([]map[string]interface{}, 0, len(ig.Positions))
		for _, p := range ig.Positions {
			children = append(children, map[string]interface{}{
				"name":       p.Name,
				"symbol":     p.Symbol,
				"value":      p.Value,
				"pnl":        p.PnL,
				"pnl_pct":    p.PnLPct,
				"colorValue": p.PnLPct,
			})
		}
		treemapData = append(treemapData, map[string]interface{}{
			"name":       ig.Industry,
			"total_value": ig.TotalValue,
			"total_pnl":  ig.TotalPnL,
			"weight":     ig.Weight,
			"children":   children,
		})
	}

	return ToolResult{
		Success: true,
		Data:    treemapData,
		Message: fmt.Sprintf("热力图数据已生成，包含 %d 个行业分组", len(treemapData)),
	}, nil
}
