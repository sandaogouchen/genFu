# internal/decision — 交易决策服务分析

## 模块概述

本模块是 Checklist（交易指南）的**消费端**，将选股模块产出的 OperationGuide/TradeGuide 作为决策依据，驱动最终的买卖指令生成和执行。

### 关键文件

| 文件 | 职责 | 大小 |
|------|------|------|
| `service.go` | 决策主服务，10 步决策流程 | 25KB |
| `types.go` | 数据结构定义，含 `DecisionRequest`/`GuideSelection` | 4KB |
| `policy_guard.go` | 风控门禁，订单级风险预检 | 5.2KB |
| `DECISION_SERVICE_FIX.md` | 已知问题文档 | 10KB |

---

## Checklist 消费流程深度分析

### 决策 Pipeline

```
Decide()
  ├── Step 1: 加载分析报告
  ├── Step 2: 确定账户ID
  ├── Step 3: 加载持仓和指南 ← 🔴 Checklist 消费入口
  ├── Step 4: 加载市场和新闻数据
  ├── Step 5: 调用决策Agent ← 🔴 Checklist 传入但未被利用
  ├── Step 6: 解析决策输出
  ├── Step 7: 生成执行订单（ExecutionPlanner）
  ├── Step 8: 风控门禁（PolicyGuard）
  ├── Step 9: 执行订单
  └── Step 10: 执行后复盘（PostTradeReview）
```

### 🔴🔴🔴 核心问题: 决策 Agent Prompt 完全缺失 Checklist 使用指导

**数据层**: `selected_trade_guides` 包含完整的买卖条件、止损止盈、量化规则

**Prompt层**: `decision.md` 完全不知道 trade guide 的存在

**输出层**: `DecisionOutput` 决策可能完全忽略 trade guide

**这是 Checklist 整合效果差的最根本原因。**

Decision Agent 的 Prompt 审查结果：
- ❌ 无 `selected_trade_guides` 字段说明
- ❌ 无如何评估 buy/sell conditions 的指导
- ❌ 无 trade guide 对 confidence 评分的影响
- ❌ 无 stop_loss/take_profit 使用指导
- ❌ 无 risk_monitors 使用指导
- ❌ 无 reason 字段引用 guide 规则要求

### 次要问题

1. **Execution Planner 未参考 Trade Guide**: 无法执行分批建仓逻辑
2. **Post-Trade Review 未评估 Guide 命中率**: 缺失反馈循环
3. **PolicyGuard 与 Trade Guide 脱节**: 不参考 guide 的止损止盈和仓位限制

---

## 🟡 改进方案

### 方案 1: 重写 Decision Prompt（高优先级，低成本）

添加交易指南评估章节，定义条件匹配率 → 决策逻辑 → reason 格式要求。

### 方案 2: 增加 Guide 评估中间层

在 Decide() Step 5 之前，自动评估每条 guide 条件的满足状态。

### 方案 3: 将 Trade Guide 传递给 ExecutionPlanner

让执行计划器参考 guide 的仓位管理规则。

### 方案 4: Post-Trade Review 增加 Guide 命中率归因

让复盘 Agent 评估哪些规则被触发、预测是否准确。

---

## 模块依赖关系

```
decision.Service
├── agent.Agent (decision, planner, review)
├── PolicyGuard (风控)
├── trade_signal.Engine (执行引擎)
├── analyze.Repository (分析报告)
├── investment.Repository (持仓)
├── stockpicker.GuideRepository (指南存储) ← Checklist 来源
└── MarketNewsProvider (市场/新闻)
```
