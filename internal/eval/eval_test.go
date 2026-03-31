package eval

import "testing"

func TestEvaluateScenarioScoresStructuredPrediction(t *testing.T) {
	scenario := Scenario{
		ID:       "rebalance-001",
		TaskType: TaskTypeRebalance,
		Constraints: Constraints{
			AllowedActions:   []string{"hold", "add", "reduce", "exit"},
			ForbiddenActions: []string{"open"},
			MaxPositionRatio: 0.20,
			RequireReasons:   true,
		},
		Expectations: Expectations{
			RequiredSymbols:   []string{"600519", "510300"},
			RequiredRiskFlags: []string{"concentration"},
			RequireActions:    true,
		},
	}

	prediction := Prediction{
		System:     "multi-agent",
		ScenarioID: "rebalance-001",
		TaskType:   TaskTypeRebalance,
		Summary:    "reduce concentration and rotate into broader exposure",
		RiskFlags:  []string{"concentration"},
		Actions: []Action{
			{
				Symbol:      "600519",
				Action:      "reduce",
				TargetRatio: 0.15,
				Reasons:     []string{"position too concentrated"},
			},
			{
				Symbol:      "510300",
				Action:      "add",
				TargetRatio: 0.10,
				Reasons:     []string{"better fit for current style"},
			},
		},
	}

	result := EvaluateScenario(scenario, prediction)
	if result.Score.TaskMatch != 1 {
		t.Fatalf("expected task match to be 1, got %v", result.Score.TaskMatch)
	}
	if result.Score.ConstraintCompliance != 1 {
		t.Fatalf("expected constraint compliance to be 1, got %v", result.Score.ConstraintCompliance)
	}
	if result.Score.Coverage != 1 {
		t.Fatalf("expected coverage to be 1, got %v", result.Score.Coverage)
	}
	if result.Score.Actionability != 1 {
		t.Fatalf("expected actionability to be 1, got %v", result.Score.Actionability)
	}
	if result.Score.Total != 1 {
		t.Fatalf("expected total score to be 1, got %v", result.Score.Total)
	}
}

func TestBuildReportAggregatesSystemScores(t *testing.T) {
	scenarios := []Scenario{
		{
			ID:       "rebalance-001",
			TaskType: TaskTypeRebalance,
			Constraints: Constraints{
				AllowedActions: []string{"hold", "add", "reduce", "exit"},
				RequireReasons: true,
			},
			Expectations: Expectations{
				RequiredSymbols: []string{"600519"},
				RequireActions:  true,
			},
		},
		{
			ID:       "diagnose-001",
			TaskType: TaskTypeDiagnose,
			Constraints: Constraints{
				AllowedActions:   []string{"hold", "reduce", "exit", "watch"},
				ForbiddenActions: []string{"open", "add"},
				RequireReasons:   true,
			},
			Expectations: Expectations{
				RequiredSymbols: []string{"159915"},
				RequireActions:  true,
			},
		},
	}

	predictions := []Prediction{
		{
			System:     "single-agent",
			ScenarioID: "rebalance-001",
			TaskType:   TaskTypeRebalance,
			Actions: []Action{
				{Symbol: "600519", Action: "reduce", TargetRatio: 0.18, Reasons: []string{"too concentrated"}},
			},
		},
		{
			System:     "single-agent",
			ScenarioID: "diagnose-001",
			TaskType:   TaskTypeDiagnose,
			Actions: []Action{
				{Symbol: "159915", Action: "open", TargetRatio: 0.10, Reasons: []string{"new chance"}},
			},
		},
		{
			System:     "multi-agent",
			ScenarioID: "rebalance-001",
			TaskType:   TaskTypeRebalance,
			Actions: []Action{
				{Symbol: "600519", Action: "reduce", TargetRatio: 0.18, Reasons: []string{"too concentrated"}},
			},
		},
		{
			System:     "multi-agent",
			ScenarioID: "diagnose-001",
			TaskType:   TaskTypeDiagnose,
			Actions: []Action{
				{Symbol: "159915", Action: "watch", TargetRatio: 0, Reasons: []string{"wait for confirmation"}},
			},
		},
	}

	report, err := BuildReport(scenarios, predictions)
	if err != nil {
		t.Fatalf("BuildReport returned error: %v", err)
	}
	if len(report.Systems) != 2 {
		t.Fatalf("expected 2 system summaries, got %d", len(report.Systems))
	}

	multi := report.System("multi-agent")
	single := report.System("single-agent")
	if multi == nil || single == nil {
		t.Fatalf("expected both system summaries to exist")
	}
	if multi.AverageScore <= single.AverageScore {
		t.Fatalf("expected multi-agent average to exceed single-agent average, got multi=%v single=%v", multi.AverageScore, single.AverageScore)
	}
	if multi.ScenarioCount != 2 {
		t.Fatalf("expected multi-agent scenario count 2, got %d", multi.ScenarioCount)
	}
}
