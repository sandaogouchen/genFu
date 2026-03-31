# internal/agent/

> **分析时间**: 2026-03-31T20:00:00+08:00
> **源分支**: `main` | **分析提交**: `04bd5e16fa7d`
> **直接源文件数**: 6 (agent.go, basic.go, func_agent.go, llm_agent.go, llm_agent_test.go, message.go)
> **直接子目录**: bear, bull, debate, decision, execution_planner, fund_manager, kline, pdfsummary, portfoliofit, post_trade_review, prompt, regime, stockpicker, stockscreener, summary, tradeguidecompiler

## 目录职责概述

Agent 包是系统的 **LLM Agent 框架层**。定义了 Agent 接口和基础实现，所有具体 Agent（选股、决策、多空辩论等）都通过此框架创建。

## 文件分析

### §1 `agent.go`
- **类型**: 接口定义
- **职责**: 定义核心 `Agent` 接口
- §1.1 `Agent` 接口：`Run(ctx, input) (output, error)`

### §2 `basic.go`
- **类型**: 基础实现
- **职责**: Agent 基础结构和公共方法

### §3 `llm_agent.go` (~12KB)
- **类型**: LLM Agent 实现
- **职责**: 基于 LLM 的 Agent 通用实现
- §3.1 `NewLLMAgentFromFile(name, skills, promptPath, model, registry)` — 从文件加载 Prompt 创建 Agent
- §3.2 使用 Eino 框架的 ChatModel 接口
- §3.3 自动注入 Tool 调用能力
- §3.4 管理对话上下文和 token 限制

### §4 `func_agent.go`
- **类型**: 函数式 Agent
- **职责**: 基于纯函数的 Agent 实现（无 LLM）

### §5 `message.go`
- **类型**: 消息模型
- **职责**: 定义 Agent 间消息传递的数据结构

### §6 `llm_agent_test.go`
- **类型**: 测试
- **职责**: LLM Agent 的单元测试

## 本目录内部依赖关系

- 所有子目录中的具体 Agent 都依赖 `llm_agent.go` 的 `NewLLMAgentFromFile()`
- `llm_agent.go` → `agent.go`（接口）→ `message.go`（消息）

## 对外暴露接口

- `Agent` 接口
- `NewLLMAgentFromFile()` 工厂函数
- 被 `main.go` 和各 service 包调用
