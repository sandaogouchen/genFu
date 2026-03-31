package news

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// CausalVerifierAgent verifies whether causal chains are evidence-backed.
type CausalVerifierAgent interface {
	VerifyBatch(ctx context.Context, batch []*NewsEvent, portfolio *PortfolioContext) ([]CausalVerification, error)
}

// LLMCausalVerifierAgent implements CausalVerifierAgent with LLM + fallback heuristics.
type LLMCausalVerifierAgent struct {
	llmSvc           LLMService
	weakThreshold    float64
	invalidThreshold float64
}

func NewLLMCausalVerifierAgent(llmSvc LLMService, weakThreshold, invalidThreshold float64) *LLMCausalVerifierAgent {
	if weakThreshold <= 0 || weakThreshold >= 1 {
		weakThreshold = 0.6
	}
	if invalidThreshold <= 0 || invalidThreshold >= weakThreshold {
		invalidThreshold = 0.4
	}
	return &LLMCausalVerifierAgent{
		llmSvc:           llmSvc,
		weakThreshold:    weakThreshold,
		invalidThreshold: invalidThreshold,
	}
}

type causalVerifierBatchResponse struct {
	Results []causalVerifierResult `json:"results"`
}

type causalVerifierResult struct {
	NewsIndex  int      `json:"news_index"`
	Score      float64  `json:"score"`
	Reason     string   `json:"reason"`
	Uncertains []string `json:"uncertains"`
}

func (a *LLMCausalVerifierAgent) VerifyBatch(ctx context.Context, batch []*NewsEvent, portfolio *PortfolioContext) ([]CausalVerification, error) {
	results := make([]CausalVerification, len(batch))
	for i, event := range batch {
		results[i] = a.fallbackVerification(event)
	}

	if len(batch) == 0 || a == nil || a.llmSvc == nil {
		return results, nil
	}

	systemPrompt := `你是因果链校验引擎，需要判断事件的因果推断是否成立。
输出 score(0-1)，并给出 reason 与 uncertains。
仅输出 JSON。`
	userPrompt := buildCausalVerifierUserPrompt(batch, portfolio)
	resp, err := a.llmSvc.ChatComplete(ctx, systemPrompt, userPrompt)
	if err != nil {
		return results, fmt.Errorf("causal verifier llm call failed: %w", err)
	}

	var parsed causalVerifierBatchResponse
	jsonStr := extractJSON(resp)
	if err := json.Unmarshal([]byte(jsonStr), &parsed); err != nil {
		return results, fmt.Errorf("causal verifier parse failed: %w", err)
	}

	for _, r := range parsed.Results {
		if r.NewsIndex < 0 || r.NewsIndex >= len(batch) {
			continue
		}
		score := clamp01(r.Score)
		results[r.NewsIndex] = CausalVerification{
			Verdict:    a.scoreToVerdict(score),
			Score:      score,
			Reason:     strings.TrimSpace(r.Reason),
			Uncertains: r.Uncertains,
		}
	}

	return results, nil
}

func buildCausalVerifierUserPrompt(batch []*NewsEvent, portfolio *PortfolioContext) string {
	var sb strings.Builder
	sb.WriteString("## 投资组合上下文\n")
	if portfolio != nil {
		sb.WriteString("持仓:\n")
		for _, h := range portfolio.Holdings {
			sb.WriteString(fmt.Sprintf("- %s(%s)\n", h.Name, h.Code))
		}
	}

	sb.WriteString("\n## 待校验事件\n")
	for i, event := range batch {
		priority := 0
		sketch := ""
		assetText := ""
		if event.FunnelResult != nil {
			priority = event.FunnelResult.L2Priority
			sketch = event.FunnelResult.L2CausalSketch
			if len(event.FunnelResult.L2AffectedAssets) > 0 {
				parts := make([]string, 0, len(event.FunnelResult.L2AffectedAssets))
				for _, asset := range event.FunnelResult.L2AffectedAssets {
					parts = append(parts, fmt.Sprintf("%s(%s)", asset.AssetName, asset.Direction))
				}
				assetText = strings.Join(parts, ",")
			}
		}
		summary := strings.TrimSpace(event.Summary)
		if len([]rune(summary)) > 180 {
			summary = string([]rune(summary)[:180])
		}
		sb.WriteString(fmt.Sprintf("[%d] 标题:%s\n摘要:%s\npriority:%d\ncausal_sketch:%s\naffected_assets:%s\n\n", i, event.Title, summary, priority, sketch, assetText))
	}

	sb.WriteString(`## 输出 JSON
{"results":[{"news_index":0,"score":0.0,"reason":"","uncertains":[""]}]}`)
	return sb.String()
}

func (a *LLMCausalVerifierAgent) fallbackVerification(event *NewsEvent) CausalVerification {
	score := 0.75
	reason := "因果链结构完整"
	uncertains := []string{}

	sketch := ""
	if event != nil && event.FunnelResult != nil {
		sketch = strings.TrimSpace(event.FunnelResult.L2CausalSketch)
	}
	if sketch == "" {
		score = 0.35
		reason = "缺少可校验的因果草图"
		uncertains = append(uncertains, "causal_sketch_missing")
	} else {
		for _, kw := range []string{"可能", "或许", "猜测", "传闻", "rumor", "might", "may"} {
			if strings.Contains(strings.ToLower(sketch), strings.ToLower(kw)) {
				score = 0.5
				reason = "因果链存在不确定表达"
				uncertains = append(uncertains, "high_uncertainty_language")
				break
			}
		}
	}

	return CausalVerification{
		Verdict:    a.scoreToVerdict(score),
		Score:      score,
		Reason:     reason,
		Uncertains: uncertains,
	}
}

func (a *LLMCausalVerifierAgent) scoreToVerdict(score float64) VerificationVerdict {
	if score < a.invalidThreshold {
		return VerificationVerdictInvalid
	}
	if score < a.weakThreshold {
		return VerificationVerdictWeak
	}
	return VerificationVerdictPassed
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
