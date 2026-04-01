# internal/agent/stockpicker — 选股 Agent 分析

## 模块概述

StockPicker Agent 是 Checklist（交易指南）的**初始生产者**。通过 LLM 生成包含 `operation_guide`、`trade_guide_text`、`trade_guide_json` 的选股结果。

### 文件结构

| 文件 | 职责 | 大小 |
|------|------|------|
| `agent.go` | Agent 工厂，注册工具和 Prompt | 434B |
| `prompt.md` | LLM System Prompt（系统中最大的 Prompt） | 7.6KB |

---

## 🔴 关键问题

### 问题 1: operation_guide 与 trade_guide_json 的重复与冲突

Prompt 同时要求输出两套描述同一组买卖逻辑的数据：
- `operation_guide`：自然语言条件
- `trade_guide_json`：量化规则

**风险**: 数据冗余，LLM 可能在两处给出不同价格。下游困惑。

### 问题 2: trade_guide_json 的 LLM 生成质量不稳定

- LLM 生成的 JSON 字符串经常无法被正确解析
- service.go 中的 `json.Valid()` 校验和降级路径证明此问题已被意识到

### 问题 3: Prompt 输出示例只有 1 个标的

对于需要同时输出 3-5 只的场景，LLM 可能在后续标的上省略字段或降低质量。

---

## 🟡 改进建议

1. **统一 operation_guide 和 trade_guide 为单一结构**：消除冗余
2. **增加输出 Schema 验证**：编译后验证
3. **多标的示例**：至少展示 2 只不同风格的标的

---

## 在 Checklist Pipeline 中的定位

```
StockPicker Agent (本模块) → 产出 StockPick[] 含 guide
    ↓
StockPicker Service.attachTradeGuides() → 确定性规则覆盖/补充
    ↓
TradeGuideCompilerAgent → LLM 编译为 v2
    ↓
GuideRepository → 持久化
    ↓
DecisionService → 消费
```

**注意**: StockPicker Agent 生成的 operation_guide 可能被 `attachTradeGuides()` 的确定性规则覆盖。
