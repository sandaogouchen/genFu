-- 操作指南表
CREATE TABLE IF NOT EXISTS operation_guides (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol TEXT NOT NULL,                    -- 股票代码
    pick_id TEXT,                            -- 关联的选股ID
    buy_conditions TEXT NOT NULL,            -- 买入条件(JSON数组)
    sell_conditions TEXT NOT NULL,           -- 卖出条件(JSON数组)
    stop_loss TEXT,                          -- 止损建议
    take_profit TEXT,                        -- 止盈建议
    risk_monitors TEXT,                      -- 风险监控点(JSON数组)
    valid_until TEXT,                        -- 有效期至
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_operation_guides_symbol ON operation_guides(symbol);
CREATE INDEX IF NOT EXISTS idx_operation_guides_pick_id ON operation_guides(pick_id);

-- 为positions表添加操作指南关联字段
ALTER TABLE positions ADD COLUMN operation_guide_id INTEGER REFERENCES operation_guides(id);
