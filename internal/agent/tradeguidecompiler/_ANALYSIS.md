# internal/agent/tradeguidecompiler/

> **分析时间**: 2026-03-31T20:00:00+08:00
> **源分支**: `main` | **分析提交**: `04bd5e16fa7d`
> **直接源文件数**: 2
> **直接子目录**: 无

## 目录职责概述

TradeGuideCompiler Agent 是交易指南整合系统的 **LLM 编译核心**。它接收确定性规则（v1）和策略上下文，通过大模型生成结构化的 v2 交易指南。是整合 checklist 效果好坏的关键环节。

## 文件分析

### §1 `agent.go`
- **类型**: Agent 工厂
- **职责**: 创建 TradeGuideCompiler Agent 实例
- §1.1 `New(model, registry)` — 调用 `agent.NewLLMAgentFromFile()` 创建
  - Agent 名称: `"trade_guide_compiler_agent"`
  - 技能标签: `["trade_rule_compilation", "rule_normalization", "json_schema_compilation"]`
  - Prompt 文件: `internal/agent/tradeguidecompiler/prompt.md`
- §1.2 使用 Eino 框架的 ToolCallingChatModel 接口

### §2 `prompt.md`
- **类型**: LLM Prompt 模板
- **职责**: 定义 TradeGuideCompiler Agent 的系统提示
- §2.1 角色定义：要求 LLM 作为"交易规则编译器"
- §2.2 输入要求：接收策略上下文、技术指标数据、已有 v1 规则
- §2.3 **输出要求**：必须输出包含以下字段的严格 JSON：
  - `stocks[].trade_guide_text` — 自然语言交易指南
  - `stocks[].trade_guide_json` — v1 格式 JSON
  - `stocks[].trade_guide_json_v2` — v2 格式 JSON
  - `stocks[].trade_guide_version` — 版本号
- §2.4 约束: "禁止输出非 JSON 内容"
- §2.5 **核心问题**: Prompt 要求同时输出 v1 和 v2 两种 JSON 格式，且嵌套在外层 JSON 中（JSON-in-JSON），极易导致转义错误

## 本目录内部依赖关系

- `agent.go` → `prompt.md`（加载 Prompt 模板）

## 对外暴露接口

- `New(model, registry)` — 返回 `agent.Agent` 接口
- 被 `internal/stockpicker/service.go` 的 `runTradeGuideCompilerAgent()` 调用

## 补充观察

### 🔍 Prompt 设计问题

**问题 1: JSON-in-JSON 嵌套**
Prompt 要求 LLM 输出的 JSON 中包含 `trade_guide_json` 和 `trade_guide_json_v2` 字段，这两个字段的值本身也是 JSON 字符串。这种嵌套导致 LLM 需要正确处理双重 JSON 转义，实际失败率很高。

**问题 2: 双格式冗余**
要求同时输出 v1 和 v2 格式，增加了 token 消耗和出错概率。实际上 v1 已经由确定性逻辑生成，LLM 只需要专注于 v2 即可。

**问题 3: 缺乏 Few-shot 示例**
Prompt 中未提供输出示例，仅描述了格式要求。对于复杂的嵌套 JSON 输出，few-shot 示例对提高格式正确率至关重要。
