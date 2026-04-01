# genFu 项目分析索引 — Checklist 整合专题

## 项目概述

genFu 是一个基于 Go 的金融 AI Agent 系统，用于股票分析和交易决策。系统采用多 Agent 协作架构，核心流程为：**选股 → 交易指南生成 → 决策 → 执行 → 复盘**。

本次分析聚焦于 **Checklist（交易指南/OperationGuide/TradeGuide）整合部分**的实现问题与优化方案。

---

## 🔴 核心发现：Checklist 整合效果差的根因分析

### 问题全景图

```
选股端 (生产) ──────────────────────── 决策端 (消费)
     │                                       │
  StockPicker Agent                    Decision Agent
  生成 operation_guide                  接收 selected_trade_guides
  + trade_guide_json                    ↓
     │                               ❌ Prompt 未指导如何使用
     ▼                               ❌ 不知道数据存在
  确定性规则覆盖                      ❌ 决策与 guide 脱节
  (attachTradeGuides)                     │
     │                                    ▼
     ▼                               输出 DecisionOutput
  TradeGuideCompiler                  (未引用 trade guide)
  ❌ Prompt 过于简陋
  ❌ 注册的工具未被使用
  ❌ v2 仅是字段重命名
```

### 根因排序

| 排序 | 根因 | 影响范围 | 修复成本 |
|------|------|---------|----------|
| **#1** | Decision Agent Prompt 完全缺失 Trade Guide 使用指导 | 🔴 致命：指南生成再好也不会被消费 | 低（修改 prompt 文件） |
| **#2** | TradeGuideCompiler Prompt 过于简陋，无增强能力 | 🔴 严重：v2 等同于 v1 字段重命名 | 低（修改 prompt 文件） |
| **#3** | 确定性 v1 规则模板化严重，缺乏波动率适应性 | 🟡 中等：默认 5%/10% 止损止盈不适应所有标的 | 中（修改 service.go 逻辑） |
| **#4** | operation_guide 与 trade_guide_json 双轨冗余 | 🟡 中等：数据不一致风险 | 中（需重构数据模型） |
| **#5** | 价格位提取依赖中文正则，可靠性差 | 🟡 中等：支撑/压力位可能解析错误 | 中（增加结构化字段） |
| **#6** | Post-Trade Review 未评估 Guide 命中率 | 🟢 低：缺失反馈循环 | 低（修改 review 输入） |
| **#7** | ExecutionPlanner 未参考 Trade Guide 仓位规则 | 🟢 低：分批建仓逻辑缺失 | 低（传递 guide 数据） |

---

## 🟡 推荐改进路线图

### Phase 1: Prompt 修复（1-2 天，无代码改动）

1. **重写 `internal/agent/prompt/decision.md`**
   - 添加 `selected_trade_guides` 字段说明和使用流程
   - 定义基于条件匹配率的决策逻辑
   - 要求 reason 字段引用 guide 规则匹配情况
   - → 详见 [internal/agent/prompt/_ANALYSIS.md](internal/agent/prompt/_ANALYSIS.md)

2. **重写 `internal/agent/tradeguidecompiler/prompt.md`**
   - 扩充到 3000-4000 字节，添加增强规则指导
   - 添加 market_regime 感知的参数调整策略
   - 定义 v2 增强字段（required_signals_count, urgency, invalidation, trailing_stop 等）
   - 添加工具使用说明
   - → 详见 [internal/agent/tradeguidecompiler/_ANALYSIS.md](internal/agent/tradeguidecompiler/_ANALYSIS.md)

### Phase 2: 数据流增强（3-5 天）

3. **在决策服务中增加 Guide 条件预评估层**
   - 在 `Decide()` Step 5 之前，自动评估每条 guide 条件的满足状态
   - 将评估结果（而非原始规则）传给 Decision Agent

4. **将 Trade Guide 传递给 ExecutionPlanner 和 ReviewAgent**
   - 让执行计划器参考仓位管理规则
   - 让复盘 Agent 评估 guide 命中率

5. **精简 TradeGuideCompiler 的输入 payload**
   - 只传递编译所需的最小字段集

### Phase 3: 架构优化（1-2 周）

6. **统一 operation_guide 和 trade_guide 双轨为单一结构**
   - 定义新的 `TradeGuideV3` 结构，消除冗余
   - 包含结构化条件（类型化 entry/exit 而非纯文本 Condition）

7. **引入波动率自适应的规则参数化**
   - 用 ATR 替代硬编码止损止盈比例
   - 根据标的历史波动率动态调整阈值

8. **建立 Guide 质量评分和反馈循环**
   - 持久化 guide 的条件触发率、预测准确率
   - 用历史数据对 guide 进行回测评分

---

## 分析文件索引

| 分析文件 | 关注焦点 | 关键发现 |
|---------|---------|----------|
| [internal/stockpicker/_ANALYSIS.md](internal/stockpicker/_ANALYSIS.md) | Trade Guide 生成流程 | v1 规则模板化、v1→v2 无增值、输入膨胀 |
| [internal/decision/_ANALYSIS.md](internal/decision/_ANALYSIS.md) | Trade Guide 消费流程 | 🔴 Decision Prompt 完全不知道 trade guide 存在 |
| [internal/agent/tradeguidecompiler/_ANALYSIS.md](internal/agent/tradeguidecompiler/_ANALYSIS.md) | LLM 编译增强 | Prompt 过于简陋、工具未使用、v2 无实质增强 |
| [internal/agent/prompt/_ANALYSIS.md](internal/agent/prompt/_ANALYSIS.md) | Prompt 模板质量 | decision.md 缺失 trade guide 段落 |
| [internal/agent/stockpicker/_ANALYSIS.md](internal/agent/stockpicker/_ANALYSIS.md) | 初始 Guide 生成 | 双轨冗余、JSON 生成质量不稳定 |

---

## 技术架构速览

### Agent 架构

```
main.go
  ├── StockPicker Service (regime→screener→analyzer→portfolioFit→tradeGuideCompiler)
  ├── Analyze Service (bull→bear→debate→summary→technical→kline)
  ├── Decision Service (decision→execution_planner→policy_guard→post_trade_review)
  └── Workflow Orchestrator (stock_workflow.go)
```

### 核心数据类型

| 类型 | 包 | 用途 |
|------|---|------|
| `OperationGuide` | stockpicker | 自然语言交易指南 |
| `StockPick` | stockpicker | 选股结果（含 guide） |
| `TradeGuideCompilerOutput` | stockpicker | LLM 编译结果 |
| `DecisionRequest.GuideSelections` | decision | 决策请求中的 guide 引用 |
| `decisionGuideInput` | decision | 展开后的 guide 完整数据 |

### 关键数据流

```
DataProvider → RegimeAgent → ScreenerAgent → AnalyzerAgent
    → PortfolioFitAgent → attachTradeGuides(v1)
    → TradeGuideCompilerAgent(v2) → GuideRepository
    → DecisionService.resolveGuideSelections()
    → buildDecisionInput() → DecisionAgent
    → ExecutionPlanner → PolicyGuard → TradeEngine
    → PostTradeReview
```

---

## 仓库信息

- **仓库**: sandaogouchen/genFu
- **语言**: Go
- **源分支**: main
- **分析分支**: analysis
- **分析日期**: 2026-04-01
