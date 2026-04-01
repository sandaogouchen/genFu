# internal/agent/tradeguidecompiler — 交易指南编译 Agent 分析

## 模块概述

TradeGuideCompilerAgent 负责将选股阶段产出的 v1 确定性交易规则，通过 LLM 编译为更丰富的 v2 版本。它是 Checklist Pipeline 中唯一的 LLM 增强环节。

### 文件结构

| 文件 | 职责 | 大小 |
|------|------|------|
| `agent.go` | Agent 工厂函数，注册工具和 Prompt | 491B |
| `prompt.md` | LLM System Prompt | 963B |

---

## 🔴 关键问题

### 问题 1: Prompt 过于简陋（963 字节 vs stockpicker 的 7678 字节）

缺少：如何增强规则的具体指导、市场状态感知、质量标准、规则组合逻辑。

### 问题 2: v2 Schema 未得到充分定义

Prompt 只说 v2 "至少包含 schema_version, entries, exits, risk_controls"，但没有定义增强维度。

### 问题 3: 未利用注册工具

Agent 注册了 `trade_rule_compilation`, `rule_normalization`, `json_schema_compilation` 三个工具，但 prompt 从未提及，导致编译器退化为纯 JSON 格式转换器。

### 问题 4: 输入数据膨胀

整个 stocks 数组（含所有嵌套字段）被全部序列化发送，信噪比低。

---

## 🟡 改进方案

### 方案 1: 重写 Prompt

扩充至 3000-4000 字节，添加：
- 基于 market_regime 的参数调整策略
- v2 增强维度（required_signals_count, urgency, invalidation, trailing_stop, scale_in_rules 等）
- 工具使用说明
- 质量标准定义

### 方案 2: 精简输入 Payload

只传递编译所需的最小字段集。

### 方案 3: 增加编译质量验证

编译后对每只标的的 v2 JSON 进行结构验证。

---

## 模块在 Checklist Pipeline 中的定位

```
StockPicker.attachTradeGuides() → v1 确定性规则
         │
         ▼
TradeGuideCompilerAgent.Handle() → 尝试编译 v2
         │
    ┌────┴────┐
    ▼         ▼
成功: v2 覆盖   失败: v1 机械转 v2
         │
         ▼
GuideRepository.SaveGuide() → 持久化
         │
         ▼
DecisionService.resolveGuideSelections() → 消费
```
