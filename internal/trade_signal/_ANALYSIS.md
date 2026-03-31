# internal/trade_signal/

> **分析时间**: 2026-03-31T20:00:00+08:00
> **源分支**: `main` | **分析提交**: `04bd5e16fa7d`
> **直接源文件数**: 4+ (含测试)
> **直接子目录**: 无

## 目录职责概述

TradeSignal 模块是交易信号执行引擎，负责将 Decision 服务输出的交易决策转换为实际的交易操作。它是交易指南系统的**末端执行层**。

## 文件分析

### §1 `engine.go`
- **类型**: 执行引擎
- **职责**: `InvestmentEngine.Execute()` 将交易信号转为实际交易
- §1.1 接收 `DecisionOutput` 中的 `TradeSignal` 列表
- §1.2 调用 `investment.Service.RecordTrade()` 记录交易
- §1.3 返回 `ExecutionResult` 包含成功/失败信息

### §2 `parser.go`
- **类型**: 解析器
- **职责**: 解析 Decision Agent 的 LLM 输出为结构化交易信号
- §2.1 从 JSON 文本中提取 action、quantity、price、confidence、reason 字段
- §2.2 处理 LLM 输出格式不一致的情况

### §3 `types.go`
- **类型**: 数据模型
- **职责**: 定义 DecisionOutput、DecisionItem、TradeSignal、ExecutionResult

## 本目录内部依赖关系

- `engine.go` → `types.go`
- `parser.go` → `types.go`

## 对外暴露接口

- `Execute(signals)` — 执行交易信号
- `Parse(llmOutput)` — 解析 LLM 决策输出
- 被 `internal/decision/service.go` 调用

## 补充观察

执行引擎本身不感知交易指南内容。指南的约束力完全依赖上游 Decision Agent 是否遵循了指南建议。这是整合 checklist 效果弱化的一个原因——指南没有在执行层进行二次校验。
