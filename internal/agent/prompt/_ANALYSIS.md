# internal/agent/prompt — Agent Prompt 模板分析

## 模块概述

本目录存放所有 LLM Agent 的 System Prompt 模板文件。对于 Checklist 整合问题，最关键的是 `decision.md`。

---

## 🔴 decision.md 深度审查

### 内容结构

1. 角色定义（1行）
2. 可用工具列表（marketdata/eastmoney/investment，详细参数说明）
3. 工作流程（4步）
4. 重要提示（工具使用强制）
5. 输出格式（DecisionOutput JSON schema）
6. 规则（5条约束）
7. 决策说明要求（hold/buy/sell 的 reason 示例）

### 缺失内容清单

| 应包含内容 | 当前状态 | 影响级别 |
|-----------|---------|----------|
| `selected_trade_guides` 字段说明 | ❌ 完全缺失 | 🔴 致命 |
| 如何评估 trade guide 的 buy/sell conditions | ❌ 完全缺失 | 🔴 致命 |
| trade guide 对 confidence 评分的影响 | ❌ 完全缺失 | 🔴 严重 |
| trade guide 的 stop_loss/take_profit 使用 | ❌ 完全缺失 | 🟡 中等 |
| trade guide 的 risk_monitors 使用 | ❌ 完全缺失 | 🟡 中等 |
| reason 字段应引用 guide 规则匹配情况 | ❌ 完全缺失 | 🟡 中等 |

### Prompt 质量对比

| 维度 | stockpicker/prompt.md | decision.md | 差距 |
|------|----------------------|-------------|------|
| 大小 | 7678B | 3608B | 决策端仅一半 |
| Trade Guide | 详细 guide 输出要求 | ❌ 完全无 | 🔴 致命差距 |

---

## 改进方案: decision.md 重写建议

在 Prompt 中添加交易指南评估章节：
- 定义评估步骤（获取实时行情 → 逐条评估条件 → 计算匹配率）
- 定义基于匹配率的决策逻辑（≥80% 强执行 → 40-59% 参考 → <20% 可能失效）
- 要求 reason 字段包含匹配率和触发规则列表
- 要求使用 guide 的 stop_loss/take_profit 作为风控参考
