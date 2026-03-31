# internal/workflow/

> **分析时间**: 2026-03-31T20:00:00+08:00
> **源分支**: `main` | **分析提交**: `04bd5e16fa7d`
> **直接源文件数**: 7+ (含测试)
> **直接子目录**: 无

## 目录职责概述

Workflow 模块实现多 Agent 协作的工作流引擎，编排选股→分析→决策→执行的完整流程。它是系统的**顶层编排层**。

## 文件分析

### §1 `stock_workflow.go` (~27KB)
- **类型**: 核心工作流
- **职责**: 实现完整的股票投资工作流
- §1.1 编排 StockScreener → StockPicker → Bull/Bear/Debate → Decision → Execution 的多步 Agent 协作
- §1.2 在每个步骤间传递上下文和中间结果
- §1.3 支持 SSE 流式输出每个步骤的进度

### §2 `handler.go`
- **类型**: HTTP 处理器
- **职责**: 暴露 `/api/workflow` 端点

### §3 `sse_handler.go`
- **类型**: SSE 处理器
- **职责**: 实现 Server-Sent Events 流式响应

### §4 `node_streamer.go`
- **类型**: 流处理器
- **职责**: 工作流节点的流式输出处理

### §5 `workflow_planner.go`
- **类型**: 工作流规划器
- **职责**: 根据用户请求动态规划工作流步骤

### §6 `types.go`
- **类型**: 数据模型
- **职责**: 工作流相关数据结构

## 本目录内部依赖关系

- `stock_workflow.go` → `types.go`
- `handler.go` → `stock_workflow.go`
- `sse_handler.go` → `node_streamer.go`

## 对外暴露接口

- HTTP 端点 `/api/workflow`
- 消费 stockpicker、decision 等服务

## 补充观察

工作流层对交易指南的处理是透传式的——它调用 StockPicker 获取带指南的结果，再传给 Decision 服务。工作流层本身不对指南质量做任何验证或增强。
