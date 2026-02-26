package analyze

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"

	"genFu/internal/agent"
	"genFu/internal/generate"
	"genFu/internal/message"
	"genFu/internal/tool"
)

type Analyzer struct {
	kline          agent.Agent
	manager        agent.Agent
	bull           agent.Agent
	bear           agent.Agent
	debate         agent.Agent
	summary        agent.Agent
	registry       *tool.Registry
	repo           *Repository
	titleGenerator *TitleGenerator
}

type FundHolding struct {
	Symbol string  `json:"symbol"`
	Name   string  `json:"name"`
	Weight float64 `json:"weight"`
}

type StepError struct {
	Step string
	Err  error
}

func (e StepError) Error() string {
	if e.Err == nil {
		return "step_error"
	}
	return e.Step + ": " + e.Err.Error()
}

func NewAnalyzer(kline agent.Agent, manager agent.Agent, bull agent.Agent, bear agent.Agent, debate agent.Agent, summary agent.Agent, registry *tool.Registry, repo *Repository) *Analyzer {
	a := &Analyzer{
		kline:    kline,
		manager:  manager,
		bull:     bull,
		bear:     bear,
		debate:   debate,
		summary:  summary,
		registry: registry,
		repo:     repo,
	}
	// Initialize title generator if we have a summary agent and repo
	if summary != nil && repo != nil {
		a.titleGenerator = NewTitleGenerator(summary, repo)
	}
	return a
}

// GetRepo returns the repository for external use
func (a *Analyzer) GetRepo() *Repository {
	if a == nil {
		return nil
	}
	return a.repo
}

func (a *Analyzer) Analyze(ctx context.Context, req AnalyzeRequest) (AnalyzeResponse, error) {
	steps, summary, err := a.AnalyzeSteps(ctx, req, nil)
	if err != nil {
		return AnalyzeResponse{}, err
	}
	return AnalyzeResponse{
		Type:    req.Type,
		Symbol:  req.Symbol,
		Name:    req.Name,
		Steps:   steps,
		Summary: summary,
	}, nil
}

func (a *Analyzer) AnalyzeSteps(ctx context.Context, req AnalyzeRequest, onStep func(AnalyzeStep) error) ([]AnalyzeStep, string, error) {
	if a == nil {
		return nil, "", errors.New("analyzer_not_initialized")
	}
	if strings.TrimSpace(req.Symbol) == "" {
		return nil, "", errors.New("missing_symbol")
	}
	if strings.TrimSpace(req.Type) == "" {
		return nil, "", errors.New("missing_type")
	}
	steps := []AnalyzeStep{}

	req = a.enrichMarketData(ctx, req)
	if strings.ToLower(strings.TrimSpace(req.Type)) == "stock" && strings.TrimSpace(req.Kline) == "" {
		if req.Meta == nil || strings.TrimSpace(req.Meta["kline_error"]) == "" {
			return nil, "", StepError{Step: "kline", Err: errors.New("missing_kline")}
		}
		return nil, "", StepError{Step: "kline", Err: errors.New(req.Meta["kline_error"])}
	}
	if strings.ToLower(strings.TrimSpace(req.Type)) == "fund" && strings.TrimSpace(req.Kline) == "" {
		if req.Meta == nil || strings.TrimSpace(req.Meta["kline_error"]) == "" {
			return nil, "", StepError{Step: "kline", Err: errors.New("missing_kline")}
		}
		return nil, "", StepError{Step: "kline", Err: errors.New(req.Meta["kline_error"])}
	}
	klineInput := buildKlineInput(req)
	klineStep, err := a.runStep(ctx, "kline", a.kline, klineInput)
	if err != nil {
		return nil, "", StepError{Step: "kline", Err: err}
	}
	if err := appendStep(&steps, klineStep, onStep); err != nil {
		return nil, "", err
	}

	var managerStep *AnalyzeStep
	if strings.ToLower(req.Type) == "fund" {
		managerInput := buildManagerInput(req, klineStep.Output)
		step, err := a.runStep(ctx, "manager", a.manager, managerInput)
		if err != nil {
			return nil, "", StepError{Step: "manager", Err: err}
		}
		managerStep = &step
		if err := appendStep(&steps, step, onStep); err != nil {
			return nil, "", err
		}
	}

	bullInput := buildBullInput(req, klineStep.Output, managerStep)
	bullStep, err := a.runStep(ctx, "bull", a.bull, bullInput)
	if err != nil {
		return nil, "", StepError{Step: "bull", Err: err}
	}
	if err := appendStep(&steps, bullStep, onStep); err != nil {
		return nil, "", err
	}

	bearInput := buildBearInput(req, klineStep.Output, managerStep)
	bearStep, err := a.runStep(ctx, "bear", a.bear, bearInput)
	if err != nil {
		return nil, "", StepError{Step: "bear", Err: err}
	}
	if err := appendStep(&steps, bearStep, onStep); err != nil {
		return nil, "", err
	}

	debateInput := buildDebateInput(req, bullStep.Output, bearStep.Output)
	debateStep, err := a.runStep(ctx, "debate", a.debate, debateInput)
	if err != nil {
		return nil, "", StepError{Step: "debate", Err: err}
	}
	if err := appendStep(&steps, debateStep, onStep); err != nil {
		return nil, "", err
	}

	summaryInput := buildSummaryInput(req, steps)
	summaryStep, err := a.runStep(ctx, "summary", a.summary, summaryInput)
	if err != nil {
		return nil, "", StepError{Step: "summary", Err: err}
	}
	if err := appendStep(&steps, summaryStep, onStep); err != nil {
		return nil, "", err
	}

	return steps, summaryStep.Output, nil
}

func (a *Analyzer) runStep(ctx context.Context, name string, ag agent.Agent, input string) (AnalyzeStep, error) {
	if ag == nil {
		return AnalyzeStep{}, errors.New("agent_not_available")
	}
	resp, err := ag.Handle(ctx, generate.GenerateRequest{
		Messages: []message.Message{
			{Role: message.RoleUser, Content: input},
		},
	})
	if err != nil {
		return AnalyzeStep{}, err
	}
	results := parseToolResultsMeta(resp.Meta)
	return AnalyzeStep{
		Name:        name,
		Input:       input,
		Output:      resp.Message.Content,
		ToolResults: results,
	}, nil
}

func parseToolResultsMeta(meta map[string]string) []tool.ToolResult {
	if meta == nil {
		return nil
	}
	raw := strings.TrimSpace(meta["tool_results"])
	if raw == "" {
		return nil
	}
	results := []tool.ToolResult{}
	if err := json.Unmarshal([]byte(raw), &results); err != nil {
		return nil
	}
	return results
}

func appendStep(steps *[]AnalyzeStep, step AnalyzeStep, onStep func(AnalyzeStep) error) error {
	*steps = append(*steps, step)
	if onStep != nil {
		return onStep(step)
	}
	return nil
}

func (a *Analyzer) enrichMarketData(ctx context.Context, req AnalyzeRequest) AnalyzeRequest {
	if a == nil || a.registry == nil {
		return req
	}
	if req.Meta == nil {
		req.Meta = map[string]string{}
	}
	dataType := strings.ToLower(strings.TrimSpace(req.Type))
	klineAction := "get_stock_kline"
	intradayAction := "get_stock_intraday"
	if dataType == "fund" {
		klineAction = "get_fund_kline"
		intradayAction = "get_fund_intraday"
	}
	if strings.TrimSpace(req.Kline) == "" {
		args := map[string]interface{}{
			"action": klineAction,
			"code":   req.Symbol,
		}
		if v := strings.TrimSpace(req.Meta["kline_period"]); v != "" {
			args["period"] = v
		}
		if v := strings.TrimSpace(req.Meta["kline_klt"]); v != "" {
			args["klt"] = v
		}
		if v := strings.TrimSpace(req.Meta["kline_adjust"]); v != "" {
			args["adjust"] = v
		}
		if v := strings.TrimSpace(req.Meta["kline_start"]); v != "" {
			args["start"] = v
		}
		if v := strings.TrimSpace(req.Meta["kline_end"]); v != "" {
			args["end"] = v
		}
		result, err := a.registry.Execute(ctx, tool.ToolCall{Name: "marketdata", Arguments: args})
		if err == nil && strings.TrimSpace(result.Error) == "" {
			raw, _ := json.Marshal(result.Output)
			req.Kline = string(raw)
			if strings.ToLower(strings.TrimSpace(req.Type)) == "fund" {
				req.Meta["kline_source"] = "fund_nav"
			} else {
				req.Meta["kline_source"] = "direct"
			}
		} else {
			req.Meta["kline_error"] = pickErrorString(err, result.Error)
		}
	}
	if strings.ToLower(strings.TrimSpace(req.Type)) == "fund" && strings.TrimSpace(req.Kline) == "" {
		estimated, err := a.estimateFundKline(ctx, req)
		if err == nil && strings.TrimSpace(estimated) != "" {
			req.Kline = estimated
			req.Meta["kline_source"] = "estimated_holdings"
			req.Meta["kline_error"] = ""
		} else if err != nil {
			req.Meta["kline_error"] = err.Error()
		}
	}
	if strings.TrimSpace(req.Meta["intraday"]) == "" {
		args := map[string]interface{}{
			"action": intradayAction,
			"code":   req.Symbol,
		}
		if v := strings.TrimSpace(req.Meta["intraday_days"]); v != "" {
			args["days"] = v
		}
		result, err := a.registry.Execute(ctx, tool.ToolCall{Name: "marketdata", Arguments: args})
		if err == nil && strings.TrimSpace(result.Error) == "" {
			raw, _ := json.Marshal(result.Output)
			req.Meta["intraday"] = string(raw)
			if strings.ToLower(strings.TrimSpace(req.Type)) == "fund" {
				req.Meta["intraday_source"] = "fund_realtime"
			}
		} else {
			req.Meta["intraday_error"] = pickErrorString(err, result.Error)
		}
	}
	return req
}

func (a *Analyzer) estimateFundKline(ctx context.Context, req AnalyzeRequest) (string, error) {
	holdings, source, err := a.loadFundHoldings(ctx, req)
	if err != nil {
		return "", err
	}
	if len(holdings) == 0 {
		return "", errors.New("missing_fund_holdings")
	}
	if req.Meta != nil && source != "" {
		req.Meta["fund_holdings_source"] = source
		req.Meta["fund_holdings_count"] = strconv.Itoa(len(holdings))
	}
	weights := normalizeHoldingWeights(holdings)
	type series struct {
		points []tool.KlinePoint
		weight float64
	}
	seriesList := make([]series, 0, len(holdings))
	for i, h := range holdings {
		if strings.TrimSpace(h.Symbol) == "" {
			continue
		}
		args := map[string]interface{}{
			"action": "get_stock_kline",
			"code":   h.Symbol,
		}
		result, err := a.registry.Execute(ctx, tool.ToolCall{Name: "marketdata", Arguments: args})
		if err != nil || strings.TrimSpace(result.Error) != "" {
			continue
		}
		raw, _ := json.Marshal(result.Output)
		var points []tool.KlinePoint
		if err := json.Unmarshal(raw, &points); err != nil {
			continue
		}
		if len(points) == 0 {
			continue
		}
		w := 1.0 / float64(len(holdings))
		if i < len(weights) {
			w = weights[i]
		}
		seriesList = append(seriesList, series{points: points, weight: w})
	}
	if len(seriesList) == 0 {
		return "", errors.New("empty_kline")
	}
	timeIndex := map[string]int{}
	count := map[string]int{}
	sumOpen := map[string]float64{}
	sumClose := map[string]float64{}
	sumHigh := map[string]float64{}
	sumLow := map[string]float64{}
	sumVolume := map[string]float64{}
	sumAmount := map[string]float64{}
	sumAmp := map[string]float64{}
	weightTotal := 0.0
	for _, s := range seriesList {
		weightTotal += s.weight
	}
	if weightTotal == 0 {
		weightTotal = float64(len(seriesList))
		for i := range seriesList {
			seriesList[i].weight = 1.0
		}
	}
	for _, s := range seriesList {
		w := s.weight / weightTotal
		for _, p := range s.points {
			key := p.Time
			if _, ok := timeIndex[key]; !ok {
				timeIndex[key] = len(timeIndex)
			}
			count[key]++
			sumOpen[key] += p.Open * w
			sumClose[key] += p.Close * w
			sumHigh[key] += p.High * w
			sumLow[key] += p.Low * w
			sumVolume[key] += p.Volume * w
			sumAmount[key] += p.Amount * w
			sumAmp[key] += p.Amplitude * w
		}
	}
	totalSeries := len(seriesList)
	result := make([]tool.KlinePoint, 0, len(timeIndex))
	keys := make([]string, 0, len(timeIndex))
	for k := range timeIndex {
		if count[k] == totalSeries {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	for _, k := range keys {
		result = append(result, tool.KlinePoint{
			Time:      k,
			Open:      sumOpen[k],
			Close:     sumClose[k],
			High:      sumHigh[k],
			Low:       sumLow[k],
			Volume:    sumVolume[k],
			Amount:    sumAmount[k],
			Amplitude: sumAmp[k],
		})
	}
	if len(result) == 0 {
		return "", errors.New("empty_kline")
	}
	raw, _ := json.Marshal(result)
	return string(raw), nil
}

func (a *Analyzer) loadFundHoldings(ctx context.Context, req AnalyzeRequest) ([]FundHolding, string, error) {
	if req.Meta != nil {
		if raw := strings.TrimSpace(req.Meta["fund_holdings"]); raw != "" {
			var holdings []FundHolding
			if err := json.Unmarshal([]byte(raw), &holdings); err == nil {
				return holdings, "meta", nil
			}
		}
	}
	result, err := a.registry.Execute(ctx, tool.ToolCall{Name: "investment", Arguments: map[string]interface{}{"action": "list_fund_holdings"}})
	if err != nil || strings.TrimSpace(result.Error) != "" {
		if err != nil {
			return nil, "investment", err
		}
		return nil, "investment", errors.New(result.Error)
	}
	raw, _ := json.Marshal(result.Output)
	var positions []struct {
		Instrument struct {
			Symbol    string `json:"symbol"`
			Name      string `json:"name"`
			AssetType string `json:"asset_type"`
		} `json:"instrument"`
		Quantity    float64  `json:"quantity"`
		AvgCost     float64  `json:"avg_cost"`
		MarketPrice *float64 `json:"market_price"`
	}
	if err := json.Unmarshal(raw, &positions); err != nil {
		return nil, "investment", err
	}
	holdings := make([]FundHolding, 0, len(positions))
	var total float64
	for _, p := range positions {
		price := p.AvgCost
		if p.MarketPrice != nil && *p.MarketPrice > 0 {
			price = *p.MarketPrice
		}
		value := price * p.Quantity
		total += value
		holdings = append(holdings, FundHolding{
			Symbol: p.Instrument.Symbol,
			Name:   p.Instrument.Name,
			Weight: value,
		})
	}
	if total > 0 {
		for i := range holdings {
			holdings[i].Weight = holdings[i].Weight / total
		}
	}
	return holdings, "investment", nil
}

func normalizeHoldingWeights(holdings []FundHolding) []float64 {
	if len(holdings) == 0 {
		return nil
	}
	total := 0.0
	for _, h := range holdings {
		if h.Weight > 0 {
			total += h.Weight
		}
	}
	weights := make([]float64, len(holdings))
	if total == 0 {
		each := 1.0 / float64(len(holdings))
		for i := range weights {
			weights[i] = each
		}
		return weights
	}
	for i, h := range holdings {
		weights[i] = h.Weight / total
	}
	return weights
}

func pickErrorString(err error, toolErr string) string {
	if err != nil {
		return err.Error()
	}
	return strings.TrimSpace(toolErr)
}

func buildKlineInput(req AnalyzeRequest) string {
	intraday := ""
	if req.Meta != nil {
		intraday = strings.TrimSpace(req.Meta["intraday"])
	}
	payload, _ := json.Marshal(map[string]interface{}{
		"type":     req.Type,
		"symbol":   req.Symbol,
		"name":     req.Name,
		"kline":    req.Kline,
		"intraday": intraday,
		"meta":     req.Meta,
	})
	return "K线分析任务，请基于以下数据进行分析：\n" + string(payload)
}

func buildManagerInput(req AnalyzeRequest, klineOutput string) string {
	payload, _ := json.Marshal(map[string]interface{}{
		"type":           req.Type,
		"symbol":         req.Symbol,
		"name":           req.Name,
		"manager":        req.Manager,
		"kline_analysis": klineOutput,
	})
	return "基金经理分析任务，请结合K线分析：\n" + string(payload)
}

func buildBullInput(req AnalyzeRequest, klineOutput string, managerStep *AnalyzeStep) string {
	payload := map[string]interface{}{
		"type":           req.Type,
		"symbol":         req.Symbol,
		"name":           req.Name,
		"kline_analysis": klineOutput,
	}
	if managerStep != nil {
		payload["manager_analysis"] = managerStep.Output
	}
	data, _ := json.Marshal(payload)
	return "多头分析任务：\n" + string(data)
}

func buildBearInput(req AnalyzeRequest, klineOutput string, managerStep *AnalyzeStep) string {
	payload := map[string]interface{}{
		"type":           req.Type,
		"symbol":         req.Symbol,
		"name":           req.Name,
		"kline_analysis": klineOutput,
	}
	if managerStep != nil {
		payload["manager_analysis"] = managerStep.Output
	}
	data, _ := json.Marshal(payload)
	return "空头分析任务：\n" + string(data)
}

func buildDebateInput(req AnalyzeRequest, bullOutput string, bearOutput string) string {
	payload, _ := json.Marshal(map[string]interface{}{
		"type":   req.Type,
		"symbol": req.Symbol,
		"name":   req.Name,
		"bull":   bullOutput,
		"bear":   bearOutput,
	})
	return "多空辩论任务：\n" + string(payload)
}

func buildSummaryInput(req AnalyzeRequest, steps []AnalyzeStep) string {
	payload, _ := json.Marshal(map[string]interface{}{
		"type":   req.Type,
		"symbol": req.Symbol,
		"name":   req.Name,
		"steps":  steps,
	})
	return "请汇总以下步骤输出为最终文档：\n" + string(payload)
}
