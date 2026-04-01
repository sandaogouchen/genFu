-- Migration 011: Rule Engine for Stop-Loss / Take-Profit
-- Tables: sl_tp_rules, sl_tp_trigger_events, sl_tp_strategy_templates

-- -------------------------------------------------------------------
-- 1. Rules table
-- -------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sl_tp_rules (
    id            TEXT    PRIMARY KEY,
    name          TEXT    NOT NULL,
    type          TEXT    NOT NULL CHECK (type IN ('stop_loss', 'take_profit')),
    strategy_id   TEXT    NOT NULL,
    params        TEXT    NOT NULL DEFAULT '{}',
    scope_level   TEXT    NOT NULL CHECK (scope_level IN ('account', 'symbol')),
    account_id    INTEGER NOT NULL,
    symbols       TEXT    NOT NULL DEFAULT '[]',
    priority      INTEGER NOT NULL DEFAULT 0,
    enabled       INTEGER NOT NULL DEFAULT 1,
    conditions    TEXT    NOT NULL DEFAULT 'null',
    action        TEXT    NOT NULL DEFAULT '{}',
    created_at    TEXT    NOT NULL,
    updated_at    TEXT    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sl_tp_rules_account
    ON sl_tp_rules (account_id);

CREATE INDEX IF NOT EXISTS idx_sl_tp_rules_type
    ON sl_tp_rules (type);

CREATE INDEX IF NOT EXISTS idx_sl_tp_rules_account_enabled
    ON sl_tp_rules (account_id, enabled);

CREATE INDEX IF NOT EXISTS idx_sl_tp_rules_strategy
    ON sl_tp_rules (strategy_id);

-- -------------------------------------------------------------------
-- 2. Trigger events table
-- -------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sl_tp_trigger_events (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    rule_id       TEXT    NOT NULL REFERENCES sl_tp_rules(id),
    account_id    INTEGER NOT NULL,
    symbol        TEXT    NOT NULL,
    trigger_price REAL    NOT NULL,
    market_price  REAL    NOT NULL,
    pnl_pct       REAL    NOT NULL,
    action        TEXT    NOT NULL DEFAULT '{}',
    status        TEXT    NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'executed', 'failed', 'cancelled')),
    trade_id      INTEGER,
    reason        TEXT    NOT NULL DEFAULT '',
    triggered_at  TEXT    NOT NULL,
    executed_at   TEXT
);

CREATE INDEX IF NOT EXISTS idx_sl_tp_events_account
    ON sl_tp_trigger_events (account_id);

CREATE INDEX IF NOT EXISTS idx_sl_tp_events_rule
    ON sl_tp_trigger_events (rule_id);

CREATE INDEX IF NOT EXISTS idx_sl_tp_events_symbol
    ON sl_tp_trigger_events (account_id, symbol);

CREATE INDEX IF NOT EXISTS idx_sl_tp_events_status
    ON sl_tp_trigger_events (status);

CREATE INDEX IF NOT EXISTS idx_sl_tp_events_triggered_at
    ON sl_tp_trigger_events (triggered_at);

-- -------------------------------------------------------------------
-- 3. Strategy templates table
-- -------------------------------------------------------------------
CREATE TABLE IF NOT EXISTS sl_tp_strategy_templates (
    id            TEXT    PRIMARY KEY,
    name          TEXT    NOT NULL,
    type          TEXT    NOT NULL CHECK (type IN ('stop_loss', 'take_profit')),
    description   TEXT    NOT NULL DEFAULT '',
    default_params TEXT   NOT NULL DEFAULT '{}',
    created_at    TEXT    NOT NULL,
    updated_at    TEXT    NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_sl_tp_templates_type
    ON sl_tp_strategy_templates (type);
