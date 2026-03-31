# internal/agent/fund_manager/

> **分析时间**: 2026-03-31T20:00:00+08:00
> **源分支**: `main` | **分析提交**: `04bd5e16fa7d`
> **直接源文件数**: 1
> **直接子目录**: 无

## 目录职责概述

基金经理 Agent，模拟基金经理视角进行投资分析。通过加载 `prompt/fund_manager.md` 创建 LLM Agent 实例。

## 文件分析

### §1 `agent.go`
- **类型**: Agent 工厂
- **职责**: 调用 `agent.NewLLMAgentFromFile()` 创建 基金经理 Agent 实例
- §1.1 Prompt 来源: `internal/agent/prompt/fund_manager.md`

## 本目录内部依赖关系

- `agent.go` → `internal/agent/llm_agent.go`（框架）

## 对外暴露接口

- `New(model, registry)` — 返回 `agent.Agent`
