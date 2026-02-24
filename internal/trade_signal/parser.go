package trade_signal

import (
	"encoding/json"
	"errors"
	"strings"
)

func ParseDecisionOutput(raw string, defaultAccountID int64) (DecisionOutput, []TradeSignal, error) {
	text := strings.TrimSpace(raw)
	if text == "" {
		return DecisionOutput{}, nil, errors.New("empty_decision_output")
	}
	if !strings.HasPrefix(text, "{") {
		return DecisionOutput{}, nil, errors.New("invalid_decision_json")
	}
	var output DecisionOutput
	if err := json.Unmarshal([]byte(text), &output); err != nil {
		return DecisionOutput{}, nil, err
	}
	if len(output.Decisions) == 0 {
		return DecisionOutput{}, nil, errors.New("empty_decisions")
	}
	signals := make([]TradeSignal, 0, len(output.Decisions))
	for _, d := range output.Decisions {
		action := strings.ToLower(strings.TrimSpace(d.Action))
		if action == "" {
			return DecisionOutput{}, nil, errors.New("missing_action")
		}
		accountID := d.AccountID
		if accountID == 0 {
			accountID = defaultAccountID
		}
		if accountID == 0 {
			return DecisionOutput{}, nil, errors.New("missing_account_id")
		}
		if strings.TrimSpace(d.Symbol) == "" {
			return DecisionOutput{}, nil, errors.New("missing_symbol")
		}
		if action != "buy" && action != "sell" && action != "hold" {
			return DecisionOutput{}, nil, errors.New("invalid_action")
		}
		if action != "hold" {
			if d.Quantity <= 0 {
				return DecisionOutput{}, nil, errors.New("invalid_quantity")
			}
			if d.Price <= 0 {
				return DecisionOutput{}, nil, errors.New("invalid_price")
			}
		}
		signals = append(signals, TradeSignal{
			AccountID:  accountID,
			Symbol:     d.Symbol,
			Name:       d.Name,
			AssetType:  d.AssetType,
			Action:     action,
			Quantity:   d.Quantity,
			Price:      d.Price,
			Confidence: d.Confidence,
			ValidUntil: d.ValidUntil,
			Reason:     d.Reason,
			DecisionID: output.DecisionID,
		})
	}
	return output, signals, nil
}
