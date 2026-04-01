# internal/stockpicker — 选股与交易指南生成模块分析

## 模块概述

本模块是 Checklist（交易指南/OperationGuide）生成的**核心模块**，承担从市场数据→策略路由→候选筛选→深度分析→组合约束→交易指南编译的完整 6 步 Pipeline。

### 关键文件

| 文件 | 职责 | 大小 |
|------|------|------|
| `service.go` | 选股主服务，6 步 Pipeline 入口 | 43KB |
| `models.go` | 数据结构定义，含 `OperationGuide`/`StockPick` | 8.5KB |
| `strategy_tools.go` | 10 种市场策略路由 + 量化筛选工具 | 16KB |

---

## Checklist（交易指南）生成流程深度分析

### Pipeline 概览

```
PickStocks()
  ├── Step 1: 数据准备（市场/新闻/持仓/股票列表）
  ├── Step 2: RegimeAgent 市场状态识别
  ├── Step 3: ScreenerAgent 策略路由 + 候选池
  ├── Step 4: AnalyzerAgent 深度分析
  ├── Step 5: PortfolioFitAgent 组合约束重排
  └── Step 6: 交易指南编译（v1确定性 → v2 LLM编译）
```

### Step 6 交易指南生成的具体实现

#### 6a: `attachTradeGuides()` — 确定性 v1 规则生成

**调用链**：`attachTradeGuides()` → `buildTradeGuideForStock()` → 各 resolve 辅助函数

**核心逻辑**：
1. `resolveStrategyMeta()` — 从筛选输出获取策略类型和名称
2. `resolvePriceReference()` — 获取参考价格（优先用 StockPick.CurrentPrice）
3. `resolveSupportResistance()` — 从技术分析 KeyLevels 和 OperationGuide 中提取支撑/压力位
4. `resolveStopLossTakeProfit()` — 从 OperationGuide 提取止损止盈（默认 0.95x/1.10x）
5. 基于筛选条件（MA5>MA20, MA20Rising, MACDGolden, RSIOversold 等）动态组装买卖规则

#### 6b: `runTradeGuideCompilerAgent()` — LLM 编译 v2

将 stocks（含 v1 规则）发送给 TradeGuideCompilerAgent，由 LLM 生成增强版 v2 规则。

#### 6c: `applyCompiledTradeGuides()` — 合并编译结果

逐标的匹配 symbol，用编译结果覆盖 v2/v1/text 字段。若编译失败则 fallback。

#### 6d: `buildPersistableGuide()` — 持久化

构建可存储的 OperationGuide，若 BuyConditions/SellConditions 为空则从 trade_guide_json 逆推。

---

## 🔴 关键问题诊断

### 问题 1: 确定性规则过于简化，缺乏市场语境

**位置**: `buildTradeGuideForStock()` 函数

**问题描述**:
- 买入规则基本只有"价格突破压力位"一条核心规则
- 其他规则（MA交叉、MACD金叉、RSI回升等）仅在筛选条件匹配时才添加
- 大量标的的 v1 规则可能只有 1-2 条，无法构成有效的交易指南
- 止损止盈默认值（-5%/+10%）是硬编码的，与标的波动率无关

**影响**: 生成的交易指南可操作性低，缺少进场时机精细度、仓位管理和动态止损逻辑。

### 问题 2: 支撑/压力位解析依赖正则从文本提取，可靠性差

**位置**: `resolveSupportResistance()` 和 `parseFirstNumber()`

**问题描述**:
- 使用正则 `[-+]?\d*\.?\d+` 从中文文本中提取第一个数字作为价格
- 关键字匹配依赖 `strings.Contains(lower, "支撑")` / `"压力"` 等中文关键字
- 当 Agent 输出格式略有变化（如用"阻力"替代"压力"）就会导致解析失败

### 问题 3: v1→v2 转换是纯机械映射，无增值

**位置**: `convertLegacyGuideToV2()` 和 `projectGuideV2ToV1()`

**问题描述**:
- v2 只是把 `buy_rules` 重命名为 `entries`，`sell_rules` 重命名为 `exits`
- 没有利用 v2 schema 引入新的结构化信息

### 问题 4: TradeGuideCompilerAgent 输入信噪比低

**位置**: `runTradeGuideCompilerAgent()` 序列化的 payload

**问题描述**:
- 整个 stocks 数组被全部序列化发送，单次输入可能超过 20KB
- LLM 在大 JSON 中容易丢失关键规则信息或产生幻觉

### 问题 5: 降级策略无差异化

**位置**: `fillTradeGuideV2Fallback()`

**问题描述**:
- 编译失败时 v2 直接由 v1 机械转换填充
- 没有根据失败原因进行差异化降级

---

## 🟡 改进建议

### 建议 1: 引入波动率自适应的规则参数化

使用 ATR（平均真实波幅）替代硬编码的 5%/10%，使止损止盈适应标的实际波动率。

### 建议 2: 结构化 v2 Schema 设计

v2 应包含：
- `entry_conditions[]` 带 `required_count`（至少 N 条同时触发）
- `exit_conditions[]` 带 `urgency`（normal/urgent/immediate）
- `position_sizing` 带 `initial_ratio`、`add_on_rules`、`scale_out_rules`
- `invalidation_conditions[]`（指南失效条件）
- `confidence_decay`（随时间衰减函数）

### 建议 3: 价格位提取结构化

Condition 增加结构化字段 `price_level`、`indicator`、`operator`，避免正则提取。

### 建议 4: 编译器输入精简

只传递编译所需的最小字段集。

### 建议 5: 规则质量评分

在持久化之前对规则进行质量打分（规则数量、止损合理性、价格距离合理性等）。

---

## 模块依赖关系

```
stockpicker.Service
├── agent.Agent (regime, screener, analyzer, portfolioFit, tradeGuideCompiler)
├── tool.Registry (stock_screener 工具)
├── DataProvider (市场数据/新闻/持仓/股票列表)
├── AllocationService (仓位分配)
├── GuideRepository (指南持久化)
└── RunRepository (运行快照)
```
