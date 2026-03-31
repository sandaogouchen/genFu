package eval

import (
	"errors"
	"fmt"
	"slices"
	"sort"
	"strings"
)

type TaskType string

const (
	TaskTypeRebalance TaskType = "rebalance"
	TaskTypeDiagnose  TaskType = "diagnose"
	TaskTypeDiscover  TaskType = "discover"
)

type Scenario struct {
	ID           string       `json:"id"`
	TaskType     TaskType     `json:"task_type"`
	Prompt       string       `json:"prompt,omitempty"`
	Constraints  Constraints  `json:"constraints"`
	Expectations Expectations `json:"expectations"`
}

type Constraints struct {
	AllowedActions   []string `json:"allowed_actions,omitempty"`
	ForbiddenActions []string `json:"forbidden_actions,omitempty"`
	MaxPositionRatio float64  `json:"max_position_ratio,omitempty"`
	RequireReasons   bool     `json:"require_reasons,omitempty"`
}

type Expectations struct {
	RequiredSymbols   []string `json:"required_symbols,omitempty"`
	RequiredRiskFlags []string `json:"required_risk_flags,omitempty"`
	RequireActions    bool     `json:"require_actions,omitempty"`
}

type Prediction struct {
	System     string   `json:"system"`
	ScenarioID string   `json:"scenario_id"`
	TaskType   TaskType `json:"task_type"`
	Summary    string   `json:"summary,omitempty"`
	RiskFlags  []string `json:"risk_flags,omitempty"`
	Actions    []Action `json:"actions,omitempty"`
}

type Action struct {
	Symbol      string   `json:"symbol"`
	Action      string   `json:"action"`
	TargetRatio float64  `json:"target_ratio,omitempty"`
	Reasons     []string `json:"reasons,omitempty"`
}

type ScoreBreakdown struct {
	TaskMatch            float64 `json:"task_match"`
	ConstraintCompliance float64 `json:"constraint_compliance"`
	Coverage             float64 `json:"coverage"`
	Actionability        float64 `json:"actionability"`
	Total                float64 `json:"total"`
}

type ScenarioResult struct {
	System     string         `json:"system"`
	ScenarioID string         `json:"scenario_id"`
	Score      ScoreBreakdown `json:"score"`
}

type SystemSummary struct {
	Name          string  `json:"name"`
	ScenarioCount int     `json:"scenario_count"`
	AverageScore  float64 `json:"average_score"`
	ScenarioWins  int     `json:"scenario_wins"`
}

type Report struct {
	Results []ScenarioResult `json:"results"`
	Systems []SystemSummary  `json:"systems"`
}

func EvaluateScenario(s Scenario, p Prediction) ScenarioResult {
	score := ScoreBreakdown{
		TaskMatch:            scoreTaskMatch(s, p),
		ConstraintCompliance: scoreConstraintCompliance(s, p),
		Coverage:             scoreCoverage(s, p),
		Actionability:        scoreActionability(s, p),
	}
	score.Total = (score.TaskMatch + score.ConstraintCompliance + score.Coverage + score.Actionability) / 4

	return ScenarioResult{
		System:     p.System,
		ScenarioID: p.ScenarioID,
		Score:      score,
	}
}

func BuildReport(scenarios []Scenario, predictions []Prediction) (Report, error) {
	if len(scenarios) == 0 {
		return Report{}, errors.New("missing_scenarios")
	}
	scenarioByID := make(map[string]Scenario, len(scenarios))
	for _, s := range scenarios {
		if s.ID == "" {
			return Report{}, errors.New("scenario_missing_id")
		}
		scenarioByID[s.ID] = s
	}

	results := make([]ScenarioResult, 0, len(predictions))
	systemScores := map[string][]float64{}
	bestByScenario := map[string]float64{}
	for _, p := range predictions {
		scenario, ok := scenarioByID[p.ScenarioID]
		if !ok {
			return Report{}, fmt.Errorf("scenario_not_found: %s", p.ScenarioID)
		}
		result := EvaluateScenario(scenario, p)
		results = append(results, result)
		systemScores[p.System] = append(systemScores[p.System], result.Score.Total)
		if result.Score.Total > bestByScenario[p.ScenarioID] {
			bestByScenario[p.ScenarioID] = result.Score.Total
		}
	}

	systems := make([]SystemSummary, 0, len(systemScores))
	for name, scores := range systemScores {
		summary := SystemSummary{
			Name:          name,
			ScenarioCount: len(scores),
			AverageScore:  average(scores),
		}
		for _, result := range results {
			if result.System == name && result.Score.Total == bestByScenario[result.ScenarioID] {
				summary.ScenarioWins++
			}
		}
		systems = append(systems, summary)
	}
	sort.Slice(systems, func(i, j int) bool {
		if systems[i].AverageScore == systems[j].AverageScore {
			return systems[i].Name < systems[j].Name
		}
		return systems[i].AverageScore > systems[j].AverageScore
	})

	return Report{
		Results: results,
		Systems: systems,
	}, nil
}

func (r Report) System(name string) *SystemSummary {
	for i := range r.Systems {
		if r.Systems[i].Name == name {
			return &r.Systems[i]
		}
	}
	return nil
}

func scoreTaskMatch(s Scenario, p Prediction) float64 {
	if s.TaskType == "" || p.TaskType == "" {
		return 0
	}
	if s.TaskType == p.TaskType {
		return 1
	}
	return 0
}

func scoreConstraintCompliance(s Scenario, p Prediction) float64 {
	for _, action := range p.Actions {
		if len(s.Constraints.AllowedActions) > 0 && !slices.Contains(s.Constraints.AllowedActions, action.Action) {
			return 0
		}
		if slices.Contains(s.Constraints.ForbiddenActions, action.Action) {
			return 0
		}
		if s.Constraints.MaxPositionRatio > 0 && action.TargetRatio > s.Constraints.MaxPositionRatio {
			return 0
		}
	}
	return 1
}

func scoreCoverage(s Scenario, p Prediction) float64 {
	symbolScore := 1.0
	if len(s.Expectations.RequiredSymbols) > 0 {
		matched := 0
		for _, symbol := range s.Expectations.RequiredSymbols {
			for _, action := range p.Actions {
				if action.Symbol == symbol {
					matched++
					break
				}
			}
		}
		symbolScore = float64(matched) / float64(len(s.Expectations.RequiredSymbols))
	}

	riskScore := 1.0
	if len(s.Expectations.RequiredRiskFlags) > 0 {
		matched := 0
		for _, flag := range s.Expectations.RequiredRiskFlags {
			if slices.Contains(p.RiskFlags, flag) {
				matched++
			}
		}
		riskScore = float64(matched) / float64(len(s.Expectations.RequiredRiskFlags))
	}

	return (symbolScore + riskScore) / 2
}

func scoreActionability(s Scenario, p Prediction) float64 {
	if s.Expectations.RequireActions && len(p.Actions) == 0 {
		return 0
	}
	if s.Constraints.RequireReasons {
		for _, action := range p.Actions {
			if len(action.Reasons) == 0 {
				return 0
			}
		}
	}
	return 1
}

func average(values []float64) float64 {
	if len(values) == 0 {
		return 0
	}
	total := 0.0
	for _, value := range values {
		total += value
	}
	return total / float64(len(values))
}

func RenderMarkdownSummary(report Report) string {
	var b strings.Builder
	b.WriteString("| System | Scenarios | Avg Score | Wins |\n")
	b.WriteString("|---|---:|---:|---:|\n")
	for _, system := range report.Systems {
		b.WriteString(fmt.Sprintf("| %s | %d | %.3f | %d |\n",
			system.Name,
			system.ScenarioCount,
			system.AverageScore,
			system.ScenarioWins,
		))
	}
	return b.String()
}
