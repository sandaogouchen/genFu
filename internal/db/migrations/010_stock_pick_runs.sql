CREATE TABLE IF NOT EXISTS stock_pick_runs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    pick_id TEXT NOT NULL UNIQUE,
    request_json TEXT NOT NULL,
    market_data_json TEXT NOT NULL,
    regime_json TEXT NOT NULL,
    routing_json TEXT NOT NULL,
    candidate_pool_json TEXT NOT NULL,
    analysis_json TEXT NOT NULL,
    portfolio_fit_json TEXT NOT NULL,
    trade_guides_json TEXT NOT NULL,
    warnings_json TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'completed',
    error_message TEXT,
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_stock_pick_runs_pick_id ON stock_pick_runs(pick_id);
CREATE INDEX IF NOT EXISTS idx_stock_pick_runs_created_at ON stock_pick_runs(created_at);

ALTER TABLE operation_guides ADD COLUMN trade_guide_json_v2 TEXT;
