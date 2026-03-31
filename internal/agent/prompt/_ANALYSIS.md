# internal/agent/prompt/

> **分析时间**: 2026-03-31T20:00:00+08:00
> **源分支**: `main` | **分析提交**: `04bd5e16fa7d`
> **直接源文件数**: 10
> **直接子目录**: 无

## 目录职责概述

集中存放部分 Agent 的 Prompt 模板文件。每个 `.md` 文件对应一个 Agent 的系统提示。

## 文件分析

### §1 `bear.md` — 看空 Agent Prompt
### §2 `bull.md` — 看多 Agent Prompt
### §3 `debate.md` — 辩论 Agent Prompt
### §4 `decision.md` ⭐ (~3.6KB) — 决策 Agent Prompt
- §4.1 定义了 Decision Agent 可用的工具：marketdata、eastmoney、investment
- §4.2 要求输出严格 JSON 格式：`decisions[].action/quantity/price/confidence/reason`
- §4.3 **注意**: 虽然 trade_guide 数据会被注入上下文，但 prompt 中对如何使用 guide 信息的指导非常有限
### §5 `execution_planner.md` — 执行计划 Agent Prompt
### §6 `fund_manager.md` — 基金经理 Agent Prompt
### §7 `kline.md` — K线分析 Agent Prompt
### §8 `ocr_holdings.md` — OCR 持仓识别 Agent Prompt
### §9 `post_trade_review.md` — 交易复盘 Agent Prompt
### §10 `summary.md` — 摘要 Agent Prompt

## 本目录内部依赖关系

无（纯模板文件，被各 Agent 的 agent.go 引用）

## 对外暴露接口

每个 `.md` 文件被对应 Agent 的 `NewLLMAgentFromFile()` 加载

## 补充观察

`decision.md` 是交易指南消费链路的关键环节。当前 prompt 对 trade guide 的使用指导不足，建议增加明确的推理步骤要求（如 "先检查 trade_guide 中的买卖规则，再做出决策"）。
