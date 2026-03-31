package ruleengine

import (
	"fmt"
	"strings"
)

// EvaluateConditionGroup evaluates a ConditionGroup against an environment map.
// If group is nil the evaluation vacuously succeeds (no conditions = always pass).
func EvaluateConditionGroup(group *ConditionGroup, env map[string]interface{}) (bool, error) {
	if group == nil {
		return true, nil
	}

	op := strings.ToLower(group.Operator)
	if op == "" {
		op = "and"
	}

	switch op {
	case "and":
		return evaluateAnd(group, env)
	case "or":
		return evaluateOr(group, env)
	default:
		return false, fmt.Errorf("unsupported condition group operator: %q", group.Operator)
	}
}

// evaluateAnd returns true only when every condition and every sub-group is true.
func evaluateAnd(group *ConditionGroup, env map[string]interface{}) (bool, error) {
	for i := range group.Conditions {
		ok, err := evaluateCondition(&group.Conditions[i], env)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	for _, sub := range group.Groups {
		ok, err := EvaluateConditionGroup(sub, env)
		if err != nil {
			return false, err
		}
		if !ok {
			return false, nil
		}
	}
	return true, nil
}

// evaluateOr returns true when at least one condition or sub-group is true.
func evaluateOr(group *ConditionGroup, env map[string]interface{}) (bool, error) {
	for i := range group.Conditions {
		ok, err := evaluateCondition(&group.Conditions[i], env)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	for _, sub := range group.Groups {
		ok, err := EvaluateConditionGroup(sub, env)
		if err != nil {
			return false, err
		}
		if ok {
			return true, nil
		}
	}
	return false, nil
}

// evaluateCondition evaluates a single Condition against the environment.
func evaluateCondition(cond *Condition, env map[string]interface{}) (bool, error) {
	raw, ok := env[cond.Field]
	if !ok {
		return false, fmt.Errorf("field %q not found in environment", cond.Field)
	}

	op := strings.ToLower(cond.Operator)

	// cross_above and cross_below are placeholders that require historical data.
	// For now they always evaluate to true.
	if op == "cross_above" || op == "cross_below" {
		return true, nil
	}

	lhs, err := toFloat64(raw)
	if err != nil {
		return false, fmt.Errorf("field %q: %w", cond.Field, err)
	}

	rhs, err := toFloat64(cond.Value)
	if err != nil {
		return false, fmt.Errorf("condition value for field %q: %w", cond.Field, err)
	}

	switch op {
	case "gte":
		return lhs >= rhs, nil
	case "lte":
		return lhs <= rhs, nil
	case "gt":
		return lhs > rhs, nil
	case "lt":
		return lhs < rhs, nil
	case "eq":
		return lhs == rhs, nil
	default:
		return false, fmt.Errorf("unsupported condition operator: %q", cond.Operator)
	}
}

// toFloat64 attempts to convert an arbitrary value to float64.
func toFloat64(v interface{}) (float64, error) {
	switch n := v.(type) {
	case float64:
		return n, nil
	case float32:
		return float64(n), nil
	case int:
		return float64(n), nil
	case int8:
		return float64(n), nil
	case int16:
		return float64(n), nil
	case int32:
		return float64(n), nil
	case int64:
		return float64(n), nil
	case uint:
		return float64(n), nil
	case uint8:
		return float64(n), nil
	case uint16:
		return float64(n), nil
	case uint32:
		return float64(n), nil
	case uint64:
		return float64(n), nil
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}
