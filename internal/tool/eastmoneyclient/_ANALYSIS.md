# internal/tool/eastmoneyclient/

> **分析时间**: 2026-03-31T20:00:00+08:00
> **源分支**: `main` | **分析提交**: `04bd5e16fa7d`
> **直接源文件数**: 1
> **直接子目录**: 无

## 目录职责概述

东方财富 HTTP 客户端封装，提供底层 API 调用能力。

## 文件分析

### §1 `client.go` (~6KB)
- **类型**: HTTP客户端
- **职责**: 封装东方财富API的HTTP请求、响应解析和错误处理

## 对外暴露接口

- `NewClient()` — 创建东方财富API客户端
- 被 `internal/tool/eastmoney.go` 调用
