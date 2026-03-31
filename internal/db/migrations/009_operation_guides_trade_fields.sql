-- 为 operation_guides 增加交易指南原文字段，兼容历史数据
ALTER TABLE operation_guides ADD COLUMN trade_guide_text TEXT;
ALTER TABLE operation_guides ADD COLUMN trade_guide_json TEXT;
ALTER TABLE operation_guides ADD COLUMN trade_guide_version TEXT;
