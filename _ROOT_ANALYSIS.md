# / (项目根目录)

> **分析时间**: 2026-03-31T20:00:00+08:00
> **源分支**: `main` | **分析提交**: `04bd5e16fa7d`
> **根目录直接文件数**: 8 (排除 .DS_Store、go.sum、二进制文件 main、pr-replica-selector-skill.tgz)
> **直接子目录**: .vscode, cmd, docs, frontend-web, internal, scripts, testdata

## 目录职责概述

项目根目录是一个 Go 多智能体投资分析平台的顶层入口。包含主程序入口文件、Go 模块定义、全局配置和项目文档。项目采用 Go 后端 + React/TypeScript 前端的全栈架构，核心业务逻辑位于 `internal/` 下。

## 文件分析

### §1 `.gitignore`
- **类型**: 版本控制配置
- **职责**: 忽略编译产物、IDE 配置、环境变量文件和数据库文件
- §1.1 忽略规则包含 `*.db`、`.env`、`node_modules/` 等标准模式

### §2 `config.yaml`
- **类型**: 全局配置文件
- **职责**: 定义 LLM 模型配置、数据库路径、API 端口、新闻源、市场数据源等运行时参数
- §2.1 `llm` 段：配置 model provider（支持 OpenAI 兼容接口）、API key、base URL、embedding model
- §2.2 `database` 段：SQLite 数据库路径
- §2.3 `server` 段：HTTP 服务端口和 CORS 设置
- §2.4 `news` 段：RSSHub 地址、新闻收集间隔
- §2.5 `marketdata` 段：行情数据源配置

### §3 `go.mod`
- **类型**: Go 模块定义
- **职责**: 声明模块路径 `github.com/sandaogouchen/genFu` 和所有外部依赖
- §3.1 Go 版本: 1.23+
- §3.2 关键依赖: `github.com/cloudwego/eino`（LLM 框架）、`github.com/mattn/go-sqlite3`（SQLite）、`github.com/gin-gonic/gin`（HTTP 路由）

### §4 `main.go`
- **类型**: 主入口文件
- **职责**: 应用初始化和 HTTP 服务启动的核心入口
- §4.1 `main()` 函数：加载配置、初始化数据库、运行迁移、创建 LLM 模型实例
- §4.2 初始化所有 Agent（stockpicker、decision、kline、bear、bull、debate 等）
- §4.3 初始化所有 Tool（marketdata、eastmoney、investment、cninfo 等）并注册到 tool.Registry
- §4.4 初始化所有 Service（StockPicker、Decision、Investment、News、Chat、Analyze、Workflow 等）
- §4.5 设置路由并启动 Gin HTTP 服务器
- §4.6 组装依赖关系：Service → Agent → Tool → LLM Model 的依赖注入链

### §5 `main_adapter.go`
- **类型**: 辅助入口
- **职责**: 提供 `NewAdapter()` 工厂函数，用于在非标准入口场景下创建应用适配器
- §5.1 封装了与 `main.go` 类似的初始化逻辑，但以可嵌入的方式暴露

### §6 `main_pdfagent.go`
- **类型**: PDF 处理入口
- **职责**: 独立的 PDF 解析 Agent 入口，用于从研报 PDF 中提取结构化信息
- §6.1 初始化 PDF Summary Agent 并提供命令行接口

### §7 `README.md`
- **类型**: 项目文档
- **职责**: 项目介绍、功能说明、API 文档、部署指南
- §7.1 描述完整 API 接口：`/api/stockpicker`、`/api/decision`、`/api/operation-guides`、`/api/investment/*`
- §7.2 文档中明确提到 `guide_selections` 参数和操作指南（Operation Guide）系统

### §8 `test_financial.go`
- **类型**: 测试文件
- **职责**: 金融数据接口的集成测试
- §8.1 测试巨潮资讯（cninfo）API 调用
- §8.2 验证财务数据获取和解析

## 本目录内部依赖关系

- `main.go` 导入 `internal/` 下所有包进行组装
- `main_adapter.go` 复用 `main.go` 的初始化模式
- `config.yaml` 被 `internal/config/` 包加载

## 对外暴露接口

- `main.go`: HTTP 服务监听端口（默认 8080）
- `README.md`: 面向开发者的 API 文档
