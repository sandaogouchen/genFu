# internal/api/

> **分析时间**: 2026-03-31T20:00:00+08:00
> **源分支**: `main` | **分析提交**: `04bd5e16fa7d`
> **直接源文件数**: 4+
> **直接子目录**: 无

## 目录职责概述

API 处理器集合，集中存放非模块化的HTTP处理器。

## 文件分析

### §1 `investment_handler.go`
- **类型**: 投资API
- **职责**: 投资组合相关HTTP端点

### §2 `marketdata_handler.go`
- **类型**: 行情API
- **职责**: 行情数据HTTP端点

### §3 `ocr_holdings_handler.go`
- **类型**: OCR持仓API (~21KB)
- **职责**: 从截图/PDF中识别持仓信息

## 本目录内部依赖关系

各文件通过包内引用协作。

## 对外暴露接口

被其他 internal 包或 main.go 导入使用。
