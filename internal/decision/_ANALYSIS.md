# internal/decision/

> **分析时间**: 2026-03-31T20:00:00+08:00
> **源分支**: `main` | **分析提交**: `04bd5e16fa7d`
> **直接源文件数**: 8+ (含测试文件)
> **直接子目录**: 无

## 目录职责概述

Decision 模块是系统的**交易决策引擎**，负责将投资分析转化为可执行的交易指令。它是交易指南（trade guide/checklist）的**核心消费者**：加载操作指南、结合实时行情和新闻，通过 LLM Agent 生成买卖决策，再经过 PolicyGuard 风控检查后输出最终交易信号。

## 文件分析

### §1 `service.go` ⭐ 关键文件（~24KB）
- **类型**: 核心业务逻辑
- **职责**: 10 步决策管线实现
- §1.1 `Decide(ctx, request)` 主方法 — 10 步管线：
  - Step 1: 加载研报和分析数据
  - Step 2: 加载当前持仓
  - **Step 3: `resolveGuideSelections()` — 解析和加载操作指南**
    - 从请求中的 `GuideSelections` 获取 symbol→guideID 映射
    - 调用 `guide_repository.GetGuideByID()` 加载指南详情
    - 验证指南 symbol 与请求 symbol 匹配
    - 转换为 `decisionGuideInput` 结构
  - Step 4: 获取实时行情数据
  - Step 5: 获取最新新闻和事件
  - Step 6: `buildDecisionInput()` — **组装 LLM 决策输入**
    - 将 `selected_trade_guides` 序列化为 JSON 纳入 prompt
    - 指南数据以纯文本形式嵌入，无结构化约束
  - Step 7: 调用 Decision Agent（LLM 决策）
  - Step 8: 解析 LLM 输出为 PlannedOrder 列表
  - Step 9: PolicyGuard 风控检查
  - Step 10: 输出 GuardedOrder 列表 + PostTradeReview
- §1.2 `resolveGuideSelections(req)` — 指南解析
  - 遍历 `req.GuideSelections`，逐个加载指南
  - 如果 guideID 无效或 symbol 不匹配，跳过该指南（静默忽略）
  - 将 OperationGuide 转换为 `decisionGuideInput`（包含 trade_guide_text, trade_guide_json, conditions）
- §1.3 `buildDecisionInput(...)` — 组装决策上下文
  - 创建 JSON 结构包含：市场数据、新闻摘要、持仓信息、**selected_trade_guides**
  - trade_guides 以数组形式嵌入，每个元素包含 symbol、guide_text、guide_json、conditions
- §1.4 `persistGuideSelections(...)` — 持久化指南选择
  - 将最终使用的指南 ID 写回 position 记录的 `operation_guide_id` 字段

### §2 `types.go`
- **类型**: 数据模型
- **职责**: 决策模块数据结构定义
- §2.1 `DecisionRequest` — 包含 `GuideSelections []GuideSelection`
- §2.2 `GuideSelection` — `{Symbol string, GuideID int64}`
- §2.3 `RiskBudget` — 风控预算参数
- §2.4 `PlannedOrder` — LLM 输出的计划订单
- §2.5 `GuardedOrder` — 经风控检查后的订单
- §2.6 `PostTradeReview` — 交易后复盘

### §3 `policy_guard.go` (~5KB)
- **类型**: 风控逻辑
- **职责**: PolicyGuard 风控代理实现
- §3.1 `Guard(planned, budget)` — 风控检查主方法
  - 检查单笔订单比例（max_single_order_ratio）
  - 检查单股暴露度（max_symbol_exposure）
  - 检查日交易比例（max_daily_trade_ratio）
  - 检查最低置信度（min_confidence）
  - 检查可用现金/可卖数量
- §3.2 不合规订单被降级或拒绝，附带原因说明

### §4 `handler.go`
- **类型**: HTTP 处理器
- **职责**: 暴露 `/api/decision` 端点（POST，支持 SSE）
- §4.1 接收 `DecisionRequest` JSON
- §4.2 调用 `service.Decide()` 并以 SSE 流式返回结果

### §5 `provider.go`
- **类型**: 数据提供层
- **职责**: 为决策流程提供持仓、行情等数据
- §5.1 调用 investment 模块获取当前持仓和现金余额

### §6 `provider_news.go`
- **类型**: 新闻数据提供
- **职责**: 从 news 模块获取与决策相关的新闻摘要
- §6.1 筛选与持仓/候选股票相关的新闻事件

### §7 `repository.go`
- **类型**: 数据访问层
- **职责**: 决策记录的存储和查询

### §8 `DECISION_SERVICE_FIX.md`
- **类型**: 文档
- **职责**: 记录 Decision 服务的已知问题和修复方案
- §8.1 文档化了决策管线中发现的具体 bug 和改进计划

## 本目录内部依赖关系

- `service.go` → `types.go`（数据结构）
- `service.go` → `policy_guard.go`（风控）
- `service.go` → `provider.go` + `provider_news.go`（数据获取）
- `service.go` → `repository.go`（持久化）
- `handler.go` → `service.go`（HTTP 入口调用）

## 对外暴露接口

- `NewService(...)` — 构造函数
- `Decide(ctx, DecisionRequest)` — 主决策方法
- `handler.go` 暴露 POST `/api/decision` 端点
- 消费 `internal/stockpicker` 的 OperationGuide 和 guide_repository

## 补充观察

### 🔍 交易指南消费端问题分析

**问题 1: 指南数据以非结构化文本传入 LLM**
`buildDecisionInput()` 将指南序列化为 JSON 文本嵌入 prompt，但 Decision Agent 的 prompt（`decision.md`）未提供明确的推理框架来解释如何基于指南做出决策。指南信息可能被 LLM 当作参考背景而非强约束。

**问题 2: 指南选择失败静默忽略**
`resolveGuideSelections()` 在指南 ID 无效或 symbol 不匹配时静默跳过，不报告错误。这意味着用户可能以为某个指南在指导决策，实际上该指南根本没有被加载。

**问题 3: 无指南执行度量**
系统不追踪决策结果与指南建议的一致性。无法回答"决策在多大程度上遵循了指南"这个关键问题。

**问题 4: 指南与风控断裂**
PolicyGuard 检查基于量化预算（比例、暴露度），但不参考操作指南中的止损/止盈价位。指南设定的止损可能被 PolicyGuard 的暴露度限制覆盖，导致指南形同虚设。
