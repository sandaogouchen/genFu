# internal/db/migrations/

> **分析时间**: 2026-03-31T20:00:00+08:00
> **源分支**: `main` | **分析提交**: `04bd5e16fa7d`
> **直接源文件数**: 20 (10 对 up/down SQL)
> **直接子目录**: 无

## 目录职责概述

数据库迁移 SQL 文件集合，定义了系统的完整数据库 schema 演进。

## 文件分析

### §1-2 `001_init` — 初始表结构
### §3-4 `002_add_conversations` — 对话表
### §5-6 `003_add_analysis` — 分析表
### §7-8 `004_add_news_events` — 新闻事件表
### §9-10 `005_add_investment` — 投资组合/持仓/交易表
### §11-12 `006_add_stockpicker` — 选股结果表
### §13-14 `007_add_stockpicker_fields` — 选股结果扩展字段
### §15-16 `008_add_news_pipeline` — 新闻管线表
### §17-18 `009_add_trade_guide` ⭐ — **操作指南表**
- §17.1 创建 `operation_guides` 表：symbol, pick_id, buy_conditions, sell_conditions, stop_loss, take_profit, risk_monitors, valid_until
- §17.2 为 `positions` 表添加 `operation_guide_id` 外键
- §17.3 添加 trade_guide_text, trade_guide_json, trade_guide_version 列
- §17.4 **缺失**: v2 JSON 字段 `trade_guide_json_v2` 未在迁移中体现
### §19-20 `010_add_decision_tables` — 决策记录表

## 补充观察

迁移 009 添加了操作指南的核心表结构，但 schema 设计中缺少 `trade_guide_json_v2` 列。这意味着 v2 格式可能仅存在于内存而未被持久化，或通过其他方式存储。这是一个潜在的数据一致性风险。
