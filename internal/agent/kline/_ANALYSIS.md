# internal/agent/kline/

> **分析时间**: 2026-03-31T20:00:00+08:00
> **源分支**: `main` | **分析提交**: `04bd5e16fa7d`
> **直接源文件数**: 1
> **直接子目录**: 无

## 目录职责概述

K线分析 Agent，分析 K 线图形态和技术信号。通过加载 `prompt/kline.md` 创建 LLM Agent 实例。

## 文件分析

### §1 `agent.go`
- **类型**: Agent 工厂
- **职责**: 调用 `agent.NewLLMAgentFromFile()` 创建 K线分析 Agent 实例
- §1.1 Prompt 来源: `internal/agent/prompt/kline.md`

## 本目录内部依赖关系

- `agent.go` → `internal/agent/llm_agent.go`（框架）

## 对外暴露接口

- `New(model, registry)` — 返回 `agent.Agent`
