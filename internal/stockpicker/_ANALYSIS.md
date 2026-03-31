# internal/stockpicker/

> **分析时间**: 2026-03-31T20:00:00+08:00
> **源分支**: `main` | **分析提交**: `04bd5e16fa7d`
> **直接源文件数**: 12+ (含测试文件)
> **直接子目录**: 无

## 目录职责概述

StockPicker 是系统的核心选股与交易指南生成模块。它实现了从技术筛选到操作指南编译的完整管线（pipeline），是"整合 checklist"（操作指南/trade guide）系统的**核心实现位置**。该模块负责：
1. 接收用户投资偏好并筛选候选股票
2. 对候选股票进行技术面/基本面评分
3. **为每只推荐股票生成结构化交易指南（v1 确定性规则 + v2 LLM 编译规则）**
4. 持久化操作指南到数据库
5. 与下游 Decision 服务集成

## 文件分析

### §1 `service.go` ⭐ 关键文件（~43KB）
- **类型**: 核心业务逻辑
- **职责**: StockPicker 服务的完整选股管线实现
- §1.1 `PickStocks()` 主方法 — 6 步管线：
  - Step 1: 加载用户投资画像和持仓
  - Step 2: 运行 StockScreener Agent（LLM 筛选）
  - Step 3: 解析筛选结果，匹配真实行情数据
  - Step 4: 运行 StockPicker Agent（LLM 精选）
  - Step 5: 资金分配计算（allocation）
  - **Step 6: 交易指南编译管线（Trade Guide Pipeline）**
- §1.2 `attachTradeGuides(output, screeningOutput, screeningResult)` — **确定性 v1 指南生成**
  - 遍历每只 StockPick，调用 `buildTradeGuideForStock()` 生成 v1 JSON
  - v1 schema: `{"buy_rules":[], "sell_rules":[], "risk_controls":[]}`
  - 每条规则为量化规则：基于 MA5/MA20 交叉、MACD、RSI、成交量等技术指标
  - 自动计算买入/卖出/止损/止盈价位：买入=现价×1.02、卖出=现价×0.97、止损=现价×0.95、止盈=现价×1.10
  - 从 `stock.TechnicalReasons.KeyLevels` 和 `stock.OperationGuide` 提取支撑/阻力位
- §1.3 `buildTradeGuideForStock(stock, screeningConds)` — 单只股票指南构建
  - 输入：StockPick 数据 + 筛选条件
  - 从筛选条件映射到具体操作规则（如 "MA5上穿MA20" → 买入规则）
  - 计算支撑位和阻力位并纳入规则
  - 输出：填充 stock.TradeGuideJSON（v1 格式）和 stock.TradeGuideText
- §1.4 `runTradeGuideCompilerAgent(ctx, ...)` — **LLM v2 指南编译**
  - 将 v1 确定性规则 + 策略上下文 + 行情数据组装为 Prompt
  - 调用 TradeGuideCompilerAgent（LLM）生成 v2 格式指南
  - v2 schema: `{"entries":[], "exits":[], "risk_controls":[], "schema_version":"v2"}`
  - **失败率较高**：LLM 必须同时输出 v1 和 v2 两种格式的 JSON
- §1.5 `applyCompiledTradeGuides(output, compilerOutput)` — 合并 LLM 编译结果
  - 将 LLM 生成的 v2 指南覆盖写入 StockPick 对应字段
  - 同时保留 v1 格式用于向下兼容
- §1.6 `fillTradeGuideV2Fallback(output)` — v1→v2 回退转换
  - 当 LLM 编译失败时触发
  - 调用 `convertLegacyGuideToV2()` 将 v1 格式机械转换为 v2
  - **信息丢失**：v1 的 buy_rules/sell_rules 语义映射到 entries/exits 时可能不精确
- §1.7 `convertLegacyGuideToV2(v1JSON)` — v1→v2 格式转换
  - buy_rules → entries
  - sell_rules → exits
  - risk_controls 保持
  - 添加 schema_version: "v2"
- §1.8 `projectGuideV2ToV1(v2JSON)` — v2→v1 反向投影
  - entries → buy_rules
  - exits → sell_rules
  - 用于向下兼容需要 v1 格式的消费者
- §1.9 `buildPersistableGuide(stock)` — 构建可持久化的操作指南
  - 将 StockPick 的字段转换为 OperationGuide 结构
  - 填充 BuyConditions、SellConditions、StopLoss、TakeProfit、RiskMonitors
  - 关联 trade_guide_text、trade_guide_json、trade_guide_version
- §1.10 `deriveConditionsFromTradeGuide(guideJSON)` — 从 JSON 反解析条件
  - 解析 v1/v2 格式的 JSON 字符串
  - 转换为 Condition 结构体数组
  - 支持 type: price/technical/quant/text

### §2 `models.go`
- **类型**: 数据模型定义
- **职责**: 定义选股模块所有数据结构
- §2.1 `StockPick` 结构体 — 核心输出模型
  - 包含：Symbol, Name, Score, Reason, TradeGuideText, TradeGuideJSON, TradeGuideJSONV2, TradeGuideVersion, OperationGuide
  - `TradeGuideVersion` 标识当前使用的 schema 版本（"v1" 或 "v2"）
- §2.2 `OperationGuide` 结构体 — 操作指南
  - BuyConditions, SellConditions: `[]Condition`
  - StopLoss, TakeProfit: string（价位）
  - RiskMonitors: `[]string`
  - 关联字段: TradeGuideText, TradeGuideJSON, TradeGuideVersion
- §2.3 `Condition` 结构体 — 条件原子
  - Type: "price" | "news" | "technical" | "fundamental" | "text" | "quant"
  - Description: 人类可读描述
  - Value: 量化值（可选）
- §2.4 `TradeGuideCompilerOutput` — LLM 编译器输出
- §2.5 `TradeGuideCompilerRecord` — 单只股票的编译结果

### §3 `guide_repository.go`
- **类型**: 数据访问层
- **职责**: 操作指南的 CRUD 操作（SQLite `operation_guides` 表）
- §3.1 `SaveGuide(guide)` — 保存操作指南，返回 ID
- §3.2 `GetLatestGuide(symbol)` — 获取某股票最新指南
- §3.3 `GetGuideByID(id)` — 按 ID 获取指南
- §3.4 `ListGuidesBySymbol(symbol)` — 按股票代码列出所有指南（时间倒序）
- §3.5 `ListGuidesByPickID(pickID)` — 按选股批次列出指南

### §4 `handler.go`
- **类型**: HTTP 处理器
- **职责**: 暴露 `/api/stockpicker` 和 `/api/operation-guides` 端点
- §4.1 POST `/api/stockpicker` — 触发选股流程（支持 SSE 流式输出）
- §4.2 GET `/api/operation-guides` — 查询操作指南（按 symbol 或 pick_id）

### §5 `provider.go` (~21KB)
- **类型**: 数据提供层
- **职责**: 为选股 Agent 提供实时行情数据和历史数据
- §5.1 调用 marketdata 和 eastmoney 工具获取 K 线、指标数据
- §5.2 构建传入 LLM Agent 的上下文数据

### §6 `screener.go`
- **类型**: 筛选器
- **职责**: 实现股票筛选逻辑
- §6.1 定义筛选条件和过滤规则
- §6.2 与 StockScreener Agent 协作

### §7 `screener_models.go`
- **类型**: 筛选器数据模型
- **职责**: 筛选器专用数据结构定义

### §8 `indicators.go`
- **类型**: 技术指标计算
- **职责**: 实现 MA、MACD、RSI、成交量等技术指标的本地计算

### §9 `allocation.go`
- **类型**: 资金分配
- **职责**: 实现推荐股票的资金分配算法（等权或按评分加权）

### §10 `strategy_tools.go` (~16KB)
- **类型**: 策略工具集
- **职责**: 为 Agent 提供的策略分析工具函数
- §10.1 技术分析工具
- §10.2 基本面分析工具

### §11 `run_repository.go`
- **类型**: 运行记录存储
- **职责**: 保存选股运行的历史记录

### §12 `service_trade_guide_test.go`
- **类型**: 测试文件
- **职责**: 交易指南生成相关的单元测试
- §12.1 测试 `attachTradeGuides` — 验证 v1 JSON 有效性
- §12.2 测试 `buildTradeGuideForStock` — 验证回退阈值（买入=1.02x, 卖出=0.97x, 止损=0.95x, 止盈=1.10x）
- §12.3 测试 `buildPersistableGuide` — 验证持久化字段完整性

### §13 `guide_repository_test.go`
- **类型**: 测试文件
- **职责**: 操作指南 CRUD 测试
- §13.1 测试 SaveGuide/GetLatestGuide/ListGuides
- §13.2 验证排序和多股票场景

## 本目录内部依赖关系

- `service.go` → `models.go`（数据结构）
- `service.go` → `guide_repository.go`（持久化）
- `service.go` → `provider.go`（行情数据）
- `service.go` → `screener.go` → `screener_models.go`（筛选）
- `service.go` → `allocation.go`（资金分配）
- `service.go` → `indicators.go`（指标计算）

## 对外暴露接口

- `NewService(...)` — 构造函数
- `PickStocks(ctx, request)` — 主选股方法
- `handler.go` 暴露的 HTTP 端点
- `OperationGuide` 和 `StockPick` 结构体被 decision 和 workflow 包消费

## 补充观察

### 🔍 交易指南整合（Checklist）问题深度分析

**问题 1: 双 Schema 复杂度导致质量不稳定**
当前系统维护 v1（buy_rules/sell_rules/risk_controls）和 v2（entries/exits/risk_controls）两套格式。LLM 编译器被要求同时输出两种格式的 JSON，这显著增加了输出错误率。当 LLM 失败时回退到机械转换，导致最终指南质量参差不齐。

**问题 2: 确定性规则过于粗糙**
`buildTradeGuideForStock()` 使用固定百分比阈值（买入=现价×1.02、卖出=现价×0.97、止损=现价×0.95、止盈=现价×1.10），不区分个股特征（波动率、行业、市值）。对于高波动股票，0.95 止损可能过紧；对于低波动蓝筹，0.97 卖出可能过早。

**问题 3: LLM 编译器 Prompt 过载**
`tradeguidecompiler/prompt.md` 要求 LLM 同时：(a) 理解策略上下文，(b) 整合多个数据源，(c) 输出严格双格式 JSON。这三重要求使得 Prompt 任务复杂度过高，容易出现格式错误。

**问题 4: 指南与决策的弱耦合**
生成的指南通过 `buildDecisionInput()` 序列化为 JSON 文本传入 Decision Agent，但 Decision Agent 的 Prompt（`decision.md`）并未对如何使用 trade guide 给出明确的推理框架。指南信息可能被 LLM 忽略或仅表面引用。

**问题 5: 无反馈闭环**
指南在选股阶段一次性生成，不会根据实际交易结果（盈亏、触发条件命中率）进行更新。即使某只股票的止损被反复触发，下次生成指南时仍使用相同的固定阈值。

**问题 6: v1↔v2 转换信息丢失**
`convertLegacyGuideToV2` 做的是字段名映射（buy_rules→entries），但 v2 的 entries 概念更丰富（可包含条件组合、优先级），机械转换无法填充这些语义。反向的 `projectGuideV2ToV1` 同样会丢失 v2 独有的结构信息。
