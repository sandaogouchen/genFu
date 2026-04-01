package tool

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/sandaogouchen/genFu/internal/ruleengine"
)

// ---------------------------------------------------------------------------
// Tool interface types
//
// These mirror genFu's existing Tool / ToolSpec / ToolResult contracts that
// live in the tool.Registry subsystem.  If the canonical definitions already
// exist elsewhere in the repository, replace these with the real imports.
// ---------------------------------------------------------------------------

// ToolSpec describes a tool's metadata and its accepted JSON-Schema parameters.
type ToolSpec struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Parameters  json.RawMessage `json:"parameters"` // JSON Schema
}

// ToolResult is the value returned to the caller after tool execution.
type ToolResult struct {
	Content string `json:"content"`
	IsError bool   `json:"is_error"`
}

// Tool is the interface every genFu tool must satisfy.
type Tool interface {
	Spec() ToolSpec
	Execute(ctx context.Context, args json.RawMessage) (ToolResult, error)
}

// ---------------------------------------------------------------------------
// RuleEngineTool
// ---------------------------------------------------------------------------

// RuleEngineTool exposes the rule-engine subsystem (stop-loss / take-profit
// rule management, position checking, trigger history) as a genFu Tool.
type RuleEngineTool struct {
	engine  *ruleengine.Engine
	store   ruleengine.RuleStore
	monitor *ruleengine.Monitor
}

// NewRuleEngineTool wires the tool to the given engine, persistent store
// and live monitor.
func NewRuleEngineTool(engine *ruleengine.Engine, store ruleengine.RuleStore, monitor *ruleengine.Monitor) *RuleEngineTool {
	return &RuleEngineTool{
		engine:  engine,
		store:   store,
		monitor: monitor,
	}
}

// Spec returns the tool metadata including a JSON Schema for the accepted
// arguments.
func (t *RuleEngineTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "rule_engine",
		Description: "Manage stop-loss and take-profit rules, check positions against active rules, and query trigger history.",
		Parameters: json.RawMessage(`{
  "type": "object",
  "required": ["action"],
  "properties": {
    "action": {
      "type": "string",
      "enum": ["create_rule","update_rule","delete_rule","list_rules","get_rule","check_position","check_all","list_triggers"],
      "description": "The operation to perform."
    },
    "rule": {
      "type": "object",
      "description": "Rule definition (for create_rule / update_rule)."
    },
    "rule_id": {
      "type": "string",
      "description": "Rule identifier (for get_rule / delete_rule)."
    },
    "account_id": {
      "type": "integer",
      "description": "Account identifier."
    },
    "symbol": {
      "type": "string",
      "description": "Ticker symbol (for check_position)."
    },
    "filter": {
      "type": "object",
      "description": "Filter criteria for list_rules."
    },
    "trigger_filter": {
      "type": "object",
      "description": "Filter criteria for list_triggers."
    }
  }
}`),
	}
}

// ruleEngineArgs is the deserialised form of the JSON arguments blob.
type ruleEngineArgs struct {
	Action        string                    `json:"action"`
	Rule          *ruleengine.Rule          `json:"rule,omitempty"`
	RuleID        string                    `json:"rule_id,omitempty"`
	AccountID     int64                     `json:"account_id,omitempty"`
	Symbol        string                    `json:"symbol,omitempty"`
	Filter        *ruleengine.RuleFilter    `json:"filter,omitempty"`
	TriggerFilter *ruleengine.TriggerFilter `json:"trigger_filter,omitempty"`
}

// Execute dispatches to the appropriate rule-engine operation based on the
// "action" field in the incoming JSON arguments.
func (t *RuleEngineTool) Execute(ctx context.Context, raw json.RawMessage) (ToolResult, error) {
	var args ruleEngineArgs
	if err := json.Unmarshal(raw, &args); err != nil {
		return errorResult(fmt.Sprintf("invalid arguments: %v", err)), nil
	}

	switch args.Action {
	case "create_rule":
		return t.createRule(ctx, args)
	case "update_rule":
		return t.updateRule(ctx, args)
	case "delete_rule":
		return t.deleteRule(ctx, args)
	case "list_rules":
		return t.listRules(ctx, args)
	case "get_rule":
		return t.getRule(ctx, args)
	case "check_position":
		return t.checkPosition(ctx, args)
	case "check_all":
		return t.checkAll(ctx, args)
	case "list_triggers":
		return t.listTriggers(ctx, args)
	default:
		return errorResult(fmt.Sprintf("unknown action: %q", args.Action)), nil
	}
}

// ---------------------------------------------------------------------------
// action handlers
// ---------------------------------------------------------------------------

func (t *RuleEngineTool) createRule(ctx context.Context, args ruleEngineArgs) (ToolResult, error) {
	if args.Rule == nil {
		return errorResult("rule is required for create_rule"), nil
	}
	if err := t.store.CreateRule(ctx, args.Rule); err != nil {
		return errorResult(fmt.Sprintf("create rule: %v", err)), nil
	}
	return jsonResult(args.Rule)
}

func (t *RuleEngineTool) updateRule(ctx context.Context, args ruleEngineArgs) (ToolResult, error) {
	if args.Rule == nil {
		return errorResult("rule is required for update_rule"), nil
	}
	if err := t.store.UpdateRule(ctx, args.Rule); err != nil {
		return errorResult(fmt.Sprintf("update rule: %v", err)), nil
	}
	return jsonResult(args.Rule)
}

func (t *RuleEngineTool) deleteRule(ctx context.Context, args ruleEngineArgs) (ToolResult, error) {
	if args.RuleID == "" {
		return errorResult("rule_id is required for delete_rule"), nil
	}
	if err := t.store.DeleteRule(ctx, args.RuleID); err != nil {
		return errorResult(fmt.Sprintf("delete rule: %v", err)), nil
	}
	return okResult(fmt.Sprintf("rule %s deleted", args.RuleID))
}

func (t *RuleEngineTool) listRules(ctx context.Context, args ruleEngineArgs) (ToolResult, error) {
	filter := ruleengine.RuleFilter{}
	if args.Filter != nil {
		filter = *args.Filter
	}
	rules, err := t.store.ListRules(ctx, filter)
	if err != nil {
		return errorResult(fmt.Sprintf("list rules: %v", err)), nil
	}
	return jsonResult(rules)
}

func (t *RuleEngineTool) getRule(ctx context.Context, args ruleEngineArgs) (ToolResult, error) {
	if args.RuleID == "" {
		return errorResult("rule_id is required for get_rule"), nil
	}
	rule, err := t.store.GetRule(ctx, args.RuleID)
	if err != nil {
		return errorResult(fmt.Sprintf("get rule: %v", err)), nil
	}
	return jsonResult(rule)
}

func (t *RuleEngineTool) checkPosition(ctx context.Context, args ruleEngineArgs) (ToolResult, error) {
	if args.Symbol == "" {
		return errorResult("symbol is required for check_position"), nil
	}
	tracker := t.engine.Tracker()
	if tracker == nil {
		return errorResult("no position tracker configured"), nil
	}
	snapshot, err := tracker.GetSnapshot(ctx, args.AccountID, args.Symbol)
	if err != nil {
		return errorResult(fmt.Sprintf("get snapshot: %v", err)), nil
	}
	results, err := t.engine.CheckPosition(ctx, snapshot)
	if err != nil {
		return errorResult(fmt.Sprintf("check position: %v", err)), nil
	}
	return jsonResult(results)
}

func (t *RuleEngineTool) checkAll(ctx context.Context, args ruleEngineArgs) (ToolResult, error) {
	results, err := t.engine.CheckAll(ctx, args.AccountID)
	if err != nil {
		return errorResult(fmt.Sprintf("check all: %v", err)), nil
	}
	return jsonResult(results)
}

func (t *RuleEngineTool) listTriggers(ctx context.Context, args ruleEngineArgs) (ToolResult, error) {
	filter := ruleengine.TriggerFilter{}
	if args.TriggerFilter != nil {
		filter = *args.TriggerFilter
	}
	triggers, err := t.store.ListTriggers(ctx, filter)
	if err != nil {
		return errorResult(fmt.Sprintf("list triggers: %v", err)), nil
	}
	return jsonResult(triggers)
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func jsonResult(v any) (ToolResult, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return errorResult(fmt.Sprintf("marshal result: %v", err)), nil
	}
	return ToolResult{Content: string(data), IsError: false}, nil
}

func okResult(msg string) (ToolResult, error) {
	return ToolResult{Content: msg, IsError: false}, nil
}

func errorResult(msg string) ToolResult {
	return ToolResult{Content: msg, IsError: true}
}
