# internal/db/

> **分析时间**: 2026-03-31T20:00:00+08:00
> **源分支**: `main` | **分析提交**: `04bd5e16fa7d`
> **直接源文件数**: 7 (config.go, db.go, db_test.go, migrate.go, seed.sql, time.go, time_test.go)
> **直接子目录**: migrations

## 目录职责概述

数据库基础设施包，负责 SQLite 连接管理、迁移执行和通用数据库工具函数。

## 文件分析

### §1 `db.go`
- **类型**: 数据库核心
- **职责**: SQLite 连接创建和管理
### §2 `config.go`
- **类型**: 数据库配置
### §3 `migrate.go`
- **类型**: 迁移执行器
- **职责**: 读取 migrations/ 目录下的 SQL 文件并按序执行
### §4 `seed.sql`
- **类型**: 种子数据
### §5 `time.go` / §6 `time_test.go`
- **类型**: 时间工具
- **职责**: 数据库时间格式处理

## 对外暴露接口

- `NewDB(config)` — 创建数据库连接
- `RunMigrations(db)` — 执行迁移
