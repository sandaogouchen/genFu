# internal/agent/stockscreener/

> **分析时间**: 2026-03-31T20:00:00+08:00
> **源分支**: `main` | **分析提交**: `04bd5e16fa7d`
> **直接源文件数**: 2 (agent.go, prompt.md)
> **直接子目录**: 无

## 目录职责概述

股票筛选 Agent，根据策略条件筛选候选股票。

## 文件分析

### §1 `agent.go`
- **类型**: Agent 工厂
- **职责**: 创建 股票筛选 Agent 实例
- §1.1 使用本目录的 `prompt.md` 作为系统提示

### §2 `prompt.md`
- **类型**: LLM Prompt
- **职责**: 定义 股票筛选 Agent 的角色、输入要求和输出格式

## 本目录内部依赖关系

- `agent.go` → `prompt.md`

## 对外暴露接口

- `New(model, registry)` — 返回 `agent.Agent`
