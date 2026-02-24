-- 财报公告缓存表
CREATE TABLE IF NOT EXISTS financial_reports (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    symbol TEXT NOT NULL,                    -- 股票代码
    announcement_id TEXT NOT NULL UNIQUE,    -- 公告ID
    title TEXT NOT NULL,                     -- 公告标题
    report_type TEXT,                        -- 报告类型(年报/季报/快报等)
    announcement_date TEXT,                  -- 公告日期
    pdf_url TEXT,                            -- PDF链接
    summary TEXT,                            -- AI生成的摘要
    key_metrics TEXT,                        -- 关键指标(JSON)
    created_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TEXT NOT NULL DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_financial_reports_symbol ON financial_reports(symbol);
CREATE INDEX IF NOT EXISTS idx_financial_reports_date ON financial_reports(announcement_date);
