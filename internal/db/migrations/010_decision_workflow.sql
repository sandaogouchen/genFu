CREATE TABLE IF NOT EXISTS decision_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    decision_id TEXT NOT NULL,
    account_id INTEGER NOT NULL,
    request_json TEXT NOT NULL,
    decision_json TEXT NOT NULL,
    risk_budget_json TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'running',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_decision_runs_decision_id ON decision_runs (decision_id);
CREATE INDEX IF NOT EXISTS idx_decision_runs_account_id_created_at ON decision_runs (account_id, created_at DESC);

CREATE TABLE IF NOT EXISTS execution_orders (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id INTEGER NOT NULL REFERENCES decision_runs(id) ON DELETE CASCADE,
    order_id TEXT NOT NULL,
    symbol TEXT NOT NULL,
    name TEXT NOT NULL DEFAULT '',
    asset_type TEXT NOT NULL DEFAULT '',
    action TEXT NOT NULL,
    quantity NUMERIC(20,8) NOT NULL,
    price NUMERIC(20,8) NOT NULL,
    notional NUMERIC(20,8) NOT NULL,
    confidence REAL NOT NULL DEFAULT 0,
    planner_reason TEXT NOT NULL DEFAULT '',
    guard_status TEXT NOT NULL DEFAULT 'pending',
    guard_reason TEXT NOT NULL DEFAULT '',
    execution_status TEXT NOT NULL DEFAULT 'pending',
    trade_id INTEGER,
    execution_error TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_execution_orders_run_id ON execution_orders (run_id);
CREATE INDEX IF NOT EXISTS idx_execution_orders_order_id ON execution_orders (order_id);
CREATE INDEX IF NOT EXISTS idx_execution_orders_symbol ON execution_orders (symbol);

CREATE TABLE IF NOT EXISTS post_trade_reviews (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    run_id INTEGER NOT NULL REFERENCES decision_runs(id) ON DELETE CASCADE,
    summary TEXT NOT NULL DEFAULT '',
    review_json TEXT NOT NULL,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_post_trade_reviews_run_id ON post_trade_reviews (run_id);
