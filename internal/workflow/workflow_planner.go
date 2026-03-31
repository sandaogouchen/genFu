package workflow

import "strings"

const (
	workflowNodeHoldings     = "holdings"
	workflowNodeHoldingsMkt  = "holdings_market"
	workflowNodeTargetMarket = "target_market"
	workflowNodeNewsFetch    = "news_fetch"
	workflowNodeNewsSummary  = "news_summary"
	workflowNodeBull         = "bull"
	workflowNodeBear         = "bear"
	workflowNodeDebate       = "debate"
	workflowNodeSummary      = "summary"
)

var workflowNodeOrder = []string{
	workflowNodeHoldings,
	workflowNodeHoldingsMkt,
	workflowNodeTargetMarket,
	workflowNodeNewsFetch,
	workflowNodeNewsSummary,
	workflowNodeBull,
	workflowNodeBear,
	workflowNodeDebate,
	workflowNodeSummary,
}

type WorkflowSkippedNode struct {
	Node   string `json:"node"`
	Reason string `json:"reason"`
}

type WorkflowPlan struct {
	Order   []string              `json:"order"`
	Enabled []string              `json:"enabled"`
	Skipped []WorkflowSkippedNode `json:"skipped,omitempty"`
}

func (p WorkflowPlan) ShouldRun(node string) bool {
	for _, enabled := range p.Enabled {
		if enabled == node {
			return true
		}
	}
	return false
}

type WorkflowPlannerAgent struct {
	defaultNewsRoutes []string
	hasInvestmentRepo bool
}

func NewWorkflowPlannerAgent(defaultNewsRoutes []string, hasInvestmentRepo bool) *WorkflowPlannerAgent {
	return &WorkflowPlannerAgent{
		defaultNewsRoutes: normalizeRoutes(defaultNewsRoutes),
		hasInvestmentRepo: hasInvestmentRepo,
	}
}

func (a *WorkflowPlannerAgent) Plan(input StockWorkflowInput) WorkflowPlan {
	if a == nil {
		return WorkflowPlan{Order: append([]string{}, workflowNodeOrder...), Enabled: append([]string{}, workflowNodeOrder...)}
	}
	prompt := strings.ToLower(strings.TrimSpace(input.Prompt))
	enabled := map[string]bool{
		workflowNodeHoldings:     true,
		workflowNodeHoldingsMkt:  true,
		workflowNodeTargetMarket: true,
		workflowNodeNewsFetch:    true,
		workflowNodeNewsSummary:  true,
		workflowNodeBull:         true,
		workflowNodeBear:         true,
		workflowNodeDebate:       true,
		workflowNodeSummary:      true,
	}
	skipped := make([]WorkflowSkippedNode, 0, 4)

	disable := func(node string, reason string) {
		if !enabled[node] {
			return
		}
		enabled[node] = false
		skipped = append(skipped, WorkflowSkippedNode{Node: node, Reason: reason})
	}

	if !a.hasInvestmentRepo || containsAny(prompt, "忽略持仓", "不看持仓", "skip holdings", "without holdings") {
		disable(workflowNodeHoldings, "holdings_unavailable")
		disable(workflowNodeHoldingsMkt, "holdings_unavailable")
	}

	routes := normalizeRoutes(append(append([]string{}, input.StockNewsRoutes...), input.IndustryNewsRoutes...))
	if len(routes) == 0 {
		routes = append(routes, a.defaultNewsRoutes...)
	}
	if len(routes) == 0 || containsAny(prompt, "忽略新闻", "不看新闻", "skip news", "without news") {
		disable(workflowNodeNewsFetch, "news_unavailable")
		disable(workflowNodeNewsSummary, "news_unavailable")
	}

	bullOnly := containsAny(prompt, "只看多头", "只要多头", "看多", "bull only")
	bearOnly := containsAny(prompt, "只看空头", "只要空头", "看空", "bear only")
	if bullOnly && !bearOnly {
		disable(workflowNodeBear, "prompt_bull_only")
		disable(workflowNodeDebate, "single_side")
	}
	if bearOnly && !bullOnly {
		disable(workflowNodeBull, "prompt_bear_only")
		disable(workflowNodeDebate, "single_side")
	}
	if !enabled[workflowNodeBull] || !enabled[workflowNodeBear] {
		disable(workflowNodeDebate, "single_side")
	}

	enabledNodes := make([]string, 0, len(workflowNodeOrder))
	for _, node := range workflowNodeOrder {
		if enabled[node] {
			enabledNodes = append(enabledNodes, node)
		}
	}
	return WorkflowPlan{
		Order:   append([]string{}, workflowNodeOrder...),
		Enabled: enabledNodes,
		Skipped: skipped,
	}
}

func containsAny(value string, patterns ...string) bool {
	if value == "" || len(patterns) == 0 {
		return false
	}
	for _, pattern := range patterns {
		p := strings.ToLower(strings.TrimSpace(pattern))
		if p == "" {
			continue
		}
		if strings.Contains(value, p) {
			return true
		}
	}
	return false
}
