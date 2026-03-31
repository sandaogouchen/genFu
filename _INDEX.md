# genFu 项目分析索引

```yaml
last_updated: "2026-03-31T20:00:00+08:00"
analyzed_commit: "04bd5e16fa7d7052415b46d957b85dac5d212faf"
analyzed_branch: "main"
total_source_files_analyzed: 150
total_analysis_files_generated: 62
generation_mode: "full"
```

## 项目概述

genFu 是一个基于 Go 语言的**多智能体投资分析平台**，通过多个 LLM Agent 协作实现从新闻收集、股票筛选、多空辩论到交易决策和执行的完整投资工作流。项目采用 Go 后端（Gin + SQLite + Eino LLM 框架）+ React/TypeScript 前端的全栈架构，核心目标是为个人投资者提供 AI 辅助的系统化投资决策支持。平台的独特之处在于引入了"操作指南"（trade guide / operation guide）系统，试图将投资策略编译为结构化的可执行规则。

## 技术栈

| 类别 | 技术 | 版本 | 用途 |
|------|------|------|------|
| 后端语言 | Go | 1.23+ | 核心业务逻辑 |
| HTTP框架 | Gin | latest | API 路由和中间件 |
| 数据库 | SQLite | 3.x | 本地数据持久化（go-sqlite3） |
| LLM框架 | Eino (cloudwego) | latest | LLM 模型调用和 Agent 编排 |
| 前端框架 | React + TypeScript | 18.x | Web UI |
| 构建工具 | Vite | latest | 前端构建 |
| 样式 | Tailwind CSS | 3.x | 前端样式 |
| 状态管理 | Zustand | latest | 前端状态 |
| 通信协议 | SSE / WebSocket | - | 流式响应和实时通信 |

## 项目结构总览

```
genFu/
├── main.go / main_adapter.go / main_pdfagent.go   # 应用入口
├── config.yaml                                      # 全局配置
├── go.mod / go.sum                                  # Go 模块
├── cmd/eval-report/                                 # CLI 工具
├── docs/                                            # 设计文档
├── frontend-web/                                    # React 前端
│   └── src/
│       ├── components/                              # UI 组件
│       ├── pages/                                   # 页面组件
│       ├── stores/                                  # 状态管理
│       └── utils/                                   # API 封装
├── internal/                                        # 核心业务逻辑
│   ├── agent/                                       # LLM Agent 框架和所有 Agent 实现
│   │   ├── tradeguidecompiler/                      # ⭐ 交易指南编译 Agent
│   │   ├── stockpicker/ / stockscreener/            # 选股相关 Agent
│   │   ├── decision/ / execution_planner/           # 决策和执行 Agent
│   │   ├── bear/ / bull/ / debate/                  # 多空辩论 Agent
│   │   └── prompt/                                  # Prompt 模板集合
│   ├── stockpicker/                                 # ⭐ 选股与交易指南生成（核心模块）
│   ├── decision/                                    # ⭐ 交易决策引擎（指南消费者）
│   ├── trade_signal/                                # 交易信号执行
│   ├── workflow/                                    # 工作流编排
│   ├── news/                                        # 新闻收集与分析
│   ├── investment/                                  # 投资组合管理
│   ├── chat/                                        # 对话服务
│   ├── analyze/                                     # 复盘分析
│   ├── llm/                                         # LLM 集成层
│   ├── tool/                                        # Agent 工具函数集
│   ├── db/                                          # 数据库基础设施
│   ├── config/                                      # 配置管理
│   ├── server/                                      # HTTP 服务器
│   └── ...                                          # 其他辅助模块
├── scripts/                                         # 测试脚本
└── testdata/                                        # 测试数据
```

## 分析文件索引

| 分析文件 | 覆盖源文件 | 概要 | 文件数 |
|---------|-----------|------|--------|
| `_ROOT_ANALYSIS.md` | main.go, config.yaml, go.mod, README.md 等 | 项目入口、配置和模块文档 | 8 |
| `.vscode/_ANALYSIS.md` | launch.json | VS Code 调试配置 | 1 |
| `cmd/eval-report/_ANALYSIS.md` | main.go | 评估报告 CLI 入口 | 1 |
| `docs/_ANALYSIS.md` | 3个文档 | 项目设计和策略文档 | 3 |
| `docs/plans/_ANALYSIS.md` | 4个设计文档 | 接口设计、评估、副本选择 | 4 |
| `frontend-web/_ANALYSIS.md` | 配置文件 | 前端构建和依赖配置 | 9 |
| `frontend-web/src/_ANALYSIS.md` | App/main/css/types | 前端应用入口 | 4 |
| `frontend-web/src/components/_ANALYSIS.md` | 8个组件 | 通用UI组件（含TradeGuideCollapse） | 8 |
| `frontend-web/src/components/conversation/_ANALYSIS.md` | 10个组件 | 对话界面组件 | 10 |
| `frontend-web/src/components/ui/_ANALYSIS.md` | 8个组件 | 基础UI组件库 | 8 |
| `frontend-web/src/components/market/_ANALYSIS.md` | MarketChart | 行情图表组件 | 1 |
| `frontend-web/src/hooks/_ANALYSIS.md` | 2个hooks | 主题和提示hooks | 2 |
| `frontend-web/src/lib/_ANALYSIS.md` | utils.ts | 通用工具 | 1 |
| `frontend-web/src/pages/_ANALYSIS.md` | 10个页面 | 所有业务页面 | 10 |
| `frontend-web/src/stores/_ANALYSIS.md` | 2个store | Zustand状态管理 | 2 |
| `frontend-web/src/utils/_ANALYSIS.md` | 3个工具文件 | API封装、设置、SSE | 3 |
| `internal/access/_ANALYSIS.md` | 2个文件 | 访问控制和凭证管理 | 2 |
| **`internal/agent/_ANALYSIS.md`** | 6个框架文件 | **Agent接口和LLM Agent基础实现** | 6 |
| `internal/agent/bear/_ANALYSIS.md` | agent.go | 看空分析Agent | 1 |
| `internal/agent/bull/_ANALYSIS.md` | agent.go | 看多分析Agent | 1 |
| `internal/agent/debate/_ANALYSIS.md` | agent.go | 多空辩论Agent | 1 |
| `internal/agent/decision/_ANALYSIS.md` | agent.go | 交易决策Agent | 1 |
| `internal/agent/execution_planner/_ANALYSIS.md` | agent.go | 执行计划Agent | 1 |
| `internal/agent/fund_manager/_ANALYSIS.md` | agent.go | 基金经理Agent | 1 |
| `internal/agent/kline/_ANALYSIS.md` | agent.go | K线分析Agent | 1 |
| `internal/agent/pdfsummary/_ANALYSIS.md` | agent.go, prompt.md | PDF摘要Agent | 2 |
| `internal/agent/portfoliofit/_ANALYSIS.md` | agent.go, prompt.md | 组合适配Agent | 2 |
| `internal/agent/post_trade_review/_ANALYSIS.md` | agent.go | 交易复盘Agent | 1 |
| `internal/agent/prompt/_ANALYSIS.md` | 10个prompt文件 | 所有Agent的Prompt模板 | 10 |
| `internal/agent/regime/_ANALYSIS.md` | agent.go, prompt.md | 市场状态判断Agent | 2 |
| `internal/agent/stockpicker/_ANALYSIS.md` | agent.go, prompt.md | 选股Agent | 2 |
| `internal/agent/stockscreener/_ANALYSIS.md` | agent.go, prompt.md | 筛选Agent | 2 |
| `internal/agent/summary/_ANALYSIS.md` | agent.go | 摘要Agent | 1 |
| **`internal/agent/tradeguidecompiler/_ANALYSIS.md`** ⭐ | agent.go, prompt.md | **交易指南编译Agent（核心问题点）** | 2 |
| `internal/analyze/_ANALYSIS.md` | 11个文件 | 复盘分析服务 | 11 |
| `internal/api/_ANALYSIS.md` | 4个handler | API处理器集合 | 4 |
| `internal/chat/_ANALYSIS.md` | 9个文件 | 对话服务和意图路由 | 9 |
| `internal/config/_ANALYSIS.md` | 2个文件 | 配置管理 | 2 |
| `internal/conversationlog/_ANALYSIS.md` | 3个文件 | 对话日志 | 3 |
| `internal/db/_ANALYSIS.md` | 7个文件 | 数据库基础设施 | 7 |
| `internal/db/migrations/_ANALYSIS.md` | 20个SQL | 数据库迁移（含操作指南表） | 20 |
| **`internal/decision/_ANALYSIS.md`** ⭐ | 8个文件 | **交易决策引擎（指南消费核心）** | 8 |
| `internal/eval/_ANALYSIS.md` | 3个文件 | Agent评估框架 | 3 |
| `internal/financial/_ANALYSIS.md` | 5个文件 | 财务数据获取 | 5 |
| `internal/generate/_ANALYSIS.md` | 1个文件 | 通用类型定义 | 1 |
| `internal/investment/_ANALYSIS.md` | 4个文件 | 投资组合管理 | 4 |
| `internal/llm/_ANALYSIS.md` | 5个文件 | LLM集成（Eino适配） | 5 |
| `internal/message/_ANALYSIS.md` | 1个文件 | 通用消息结构 | 1 |
| `internal/news/_ANALYSIS.md` | 16个文件 | 新闻收集和分析管线 | 16 |
| `internal/newsgen/_ANALYSIS.md` | 1个文件 | LLM新闻生成 | 1 |
| `internal/router/_ANALYSIS.md` | 2个文件 | HTTP路由配置 | 2 |
| `internal/rsshub/_ANALYSIS.md` | 2个文件 | RSSHub客户端 | 2 |
| `internal/server/_ANALYSIS.md` | 3个文件 | HTTP服务器和OpenAPI | 3 |
| **`internal/stockpicker/_ANALYSIS.md`** ⭐ | 12个文件 | **选股与交易指南生成（核心模块）** | 12 |
| `internal/testutil/_ANALYSIS.md` | 2个文件 | 测试工具 | 2 |
| `internal/tool/_ANALYSIS.md` | 12个文件 | Agent工具函数集 | 12 |
| `internal/tool/eastmoneyclient/_ANALYSIS.md` | 1个文件 | 东方财富API客户端 | 1 |
| **`internal/trade_signal/_ANALYSIS.md`** | 4个文件 | **交易信号执行引擎** | 4 |
| **`internal/workflow/_ANALYSIS.md`** | 7个文件 | **工作流编排引擎** | 7 |
| `internal/ws/_ANALYSIS.md` | 2个文件 | WebSocket处理 | 2 |
| `scripts/_ANALYSIS.md` | 2个脚本 | 测试辅助脚本 | 2 |
| `testdata/eval/_ANALYSIS.md` | 2个JSON | 评估测试数据 | 2 |

## 模块依赖全景

```
┌─────────────┐
│  main.go    │  应用入口，组装所有依赖
└──────┬──────┘
       │
       ▼
┌──────────────────────────────────────────────────────────┐
│                    internal/workflow/                      │
│  stock_workflow.go — 编排完整投资工作流                      │
└──────┬───────────────────────┬────────────────────────────┘
       │                       │
       ▼                       ▼
┌──────────────┐      ┌───────────────┐
│ stockpicker/ │      │  decision/    │
│ 选股+指南生成  │─────→│ 决策+指南消费   │
│              │指南   │               │
└──────┬───────┘      └───────┬───────┘
       │                       │
       ▼                       ▼
┌──────────────┐      ┌───────────────┐
│ agent/trade  │      │ trade_signal/ │
│ guidecompiler│      │ 信号执行       │
│ LLM编译指南   │      └───────┬───────┘
└──────────────┘              │
                              ▼
                      ┌───────────────┐
                      │ investment/   │
                      │ 组合+持仓管理   │
                      └───────────────┘

横向支撑：
├── agent/          Agent 框架 + 所有 Agent 实现
├── tool/           Agent 工具集（marketdata, eastmoney, investment 等）
├── llm/            LLM 模型封装（Eino 框架）
├── news/           新闻收集与分析
├── db/             数据库基础设施
├── config/         配置管理
└── chat/           对话交互
```

**交易指南（Checklist）数据流**:
```
StockPicker.PickStocks()
  ├── attachTradeGuides()        → 生成 v1 确定性规则
  ├── runTradeGuideCompilerAgent() → LLM 编译 v2 (可能失败)
  │   ├── 成功 → applyCompiledTradeGuides()  → 使用 v2
  │   └── 失败 → fillTradeGuideV2Fallback()  → v1 机械转 v2
  └── buildPersistableGuide()    → 存入 SQLite

Decision.Decide()
  ├── resolveGuideSelections()   → 从 DB 加载指南
  ├── buildDecisionInput()       → 序列化为 JSON 传入 LLM
  ├── Decision Agent (LLM)       → 生成交易决策
  ├── PolicyGuard                → 风控检查（不参考指南）
  └── TradeSignal.Execute()      → 执行交易
```

## 外部依赖与集成

| 外部服务 | 用途 | 配置方式 |
|---------|------|---------|
| OpenAI 兼容 LLM API | 所有 Agent 的大模型调用 | config.yaml `llm` 段 |
| 东方财富 API | 行情数据、财经资讯 | 内置 HTTP 客户端 |
| 巨潮资讯 API | 上市公司财务数据 | 内置 HTTP 客户端 |
| RSSHub | 新闻 RSS 订阅 | config.yaml `news.rsshub_url` |
| SQLite | 本地数据持久化 | config.yaml `database.path` |

## 环境变量清单

| 变量名 | 用途 | 必需 | 使用位置 |
|--------|------|------|---------|
| (通过 config.yaml 配置) | 大部分配置通过 YAML 文件管理 | - | - |
| LLM API Key | LLM 服务认证 | 是 | config.yaml `llm.api_key` |
| RSSHub URL | 新闻源地址 | 否 | config.yaml `news.rsshub_url` |

## 改进建议

### ⭐ 重点：交易指南整合（Checklist）系统改进方案

基于对 `internal/stockpicker/service.go`、`internal/decision/service.go`、`internal/agent/tradeguidecompiler/` 的深度分析，当前交易指南（checklist）系统存在以下核心问题和对应的改进建议：

---

#### 问题 1：双 Schema 复杂度导致 LLM 输出不稳定

**现状**: 系统维护 v1（`buy_rules/sell_rules/risk_controls`）和 v2（`entries/exits/risk_controls`）两套 JSON Schema。TradeGuideCompiler Agent 被要求在单次调用中同时输出两种格式，形成 JSON-in-JSON 嵌套结构，导致 LLM 输出格式错误率高。

**改进方案**:
1. **统一到 v2 Schema，废弃 v1**。v2 的 `entries/exits` 语义更丰富，可完全覆盖 v1 的 `buy_rules/sell_rules`。
2. **对存量 v1 数据做一次性迁移**，使用现有的 `convertLegacyGuideToV2()` 批量转换，之后系统只生产和消费 v2。
3. **简化 LLM 输出要求**：编译器只需输出 v2 JSON + 自然语言文本，去掉 v1 冗余输出。
4. 参考位置: `internal/stockpicker/service.go` §1.2-§1.8, `internal/agent/tradeguidecompiler/prompt.md` §2.3

---

#### 问题 2：确定性规则过于粗糙，缺乏个股适应性

**现状**: `buildTradeGuideForStock()` 使用固定百分比阈值（买入=现价×1.02、卖出=现价×0.97、止损=现价×0.95、止盈=现价×1.10），不区分个股波动率、行业特征、市值规模。

**改进方案**:
1. **引入基于 ATR（Average True Range）的自适应阈值**：
   - 止损 = 现价 - 2×ATR（高波动股自动放宽）
   - 止盈 = 现价 + 3×ATR（高波动股自动放宽）
   - 买入信号 = 突破近期阻力 + 0.5×ATR 确认
2. **分行业设定基准参数**：创业板/科创板（高波动）vs 沪深300成分股（低波动）使用不同的默认参数集。
3. **利用已有的 `indicators.go` 模块**：ATR 计算可基于已有的技术指标计算框架实现。
4. 参考位置: `internal/stockpicker/service.go` §1.3, `internal/stockpicker/indicators.go`

---

#### 问题 3：LLM 编译器 Prompt 任务过载

**现状**: TradeGuideCompiler 的 prompt.md 要求 LLM 同时：(a) 理解策略上下文，(b) 整合多源数据，(c) 生成严格双格式 JSON，(d) 确保数值合理性。失败后回退到机械转换，质量骤降。

**改进方案**:
1. **拆分为两阶段编译**：
   - 阶段 1（推理）：LLM 输出自然语言的交易策略分析（不要求 JSON）
   - 阶段 2（结构化）：用确定性代码或轻量 LLM 调用将自然语言转换为 v2 JSON
2. **添加 Few-shot 示例**：在 prompt.md 中加入 2-3 个完整的输入→输出示例，显著提高格式正确率。
3. **引入 JSON Schema 验证**：在接收 LLM 输出后立即做 Schema 验证，不合规则自动重试（最多 2 次）。
4. 参考位置: `internal/agent/tradeguidecompiler/prompt.md` §2, `internal/stockpicker/service.go` §1.4

---

#### 问题 4：指南在决策端的消费过于被动

**现状**: `buildDecisionInput()` 将指南序列化为 JSON 文本嵌入 Decision Agent 的 prompt，但 `decision.md` 对如何使用指南信息的指导非常有限。LLM 可能忽略指南或仅表面引用。

**改进方案**:
1. **在 Decision Agent Prompt 中增加强制推理步骤**：
   ```
   决策流程（必须按顺序执行）：
   1. 先逐一检查 trade_guide 中的买入/卖出条件是否满足
   2. 标注每个条件的当前状态（已满足/未满足/数据不足）
   3. 基于条件满足比例和市场整体判断做出最终决策
   4. 在 reason 中明确引用使用了哪些指南规则
   ```
2. **引入指南遵从度评分**：在 PostTradeReview 中计算"决策与指南的一致性百分比"，作为回馈指标。
3. **结构化注入而非纯文本**：将指南条件拆解为独立的 checklist items 传入，而非一大段 JSON 文本。
4. 参考位置: `internal/decision/service.go` §1.3, `internal/agent/prompt/decision.md` §4

---

#### 问题 5：指南选择失败静默忽略

**现状**: `resolveGuideSelections()` 在指南 ID 无效或 symbol 不匹配时静默跳过。用户可能以为指南在指导决策，实际上该指南未被加载。

**改进方案**:
1. **返回指南加载状态报告**：在 `DecisionOutput` 中增加 `guide_load_status` 字段，报告每个指南的加载结果（loaded/not_found/symbol_mismatch）。
2. **在 SSE 流中推送指南加载事件**：让前端实时展示哪些指南被成功加载。
3. 参考位置: `internal/decision/service.go` §1.2

---

#### 问题 6：指南与 PolicyGuard 断裂

**现状**: PolicyGuard 检查基于量化风控预算（单笔比例、暴露度、日交易比例），完全不参考操作指南中的止损/止盈价位。两套风控逻辑独立运行，可能产生矛盾。

**改进方案**:
1. **在 PolicyGuard 中增加指南一致性检查**：
   - 如果指南设定了止损价，而 LLM 决策的卖出价低于止损价，标记为"提前止损"
   - 如果 LLM 建议的买入价高于指南的建议买入价上限，标记为"偏离指南"
2. **将指南约束作为 PolicyGuard 的软约束**：不强制拒绝，但在 `GuardedOrder` 中添加 `guide_deviation_warning`。
3. 参考位置: `internal/decision/policy_guard.go` §3, `internal/stockpicker/service.go` §1.2

---

#### 问题 7：无反馈闭环，指南不会进化

**现状**: 指南在选股阶段一次性生成后存入数据库，不会根据实际交易结果更新。即使某条规则反复失效，下次仍会生成类似规则。

**改进方案**:
1. **引入指南效果追踪表**：记录每个指南关联的交易结果（盈亏、持有期、条件命中率）。
2. **在下次选股时注入历史反馈**：将同一股票历史指南的效果数据传入 TradeGuideCompiler Agent，让 LLM 参考历史表现生成更好的指南。
3. **定期清理低效指南**：对历史命中率低于阈值的指南条件类型降权。
4. 参考位置: `internal/stockpicker/guide_repository.go`, `internal/stockpicker/service.go` §1.4

---

#### 问题 8：v2 JSON 未在数据库 Schema 中持久化

**现状**: 迁移 009 添加了 `trade_guide_json` 和 `trade_guide_version` 列，但未添加 `trade_guide_json_v2` 列。v2 格式数据可能仅存在于内存中或通过覆写 `trade_guide_json` 存储，存在数据一致性风险。

**改进方案**:
1. 添加迁移 011，为 `operation_guides` 表增加 `trade_guide_json_v2 TEXT` 列。
2. 或者在统一到 v2 之后，直接使用 `trade_guide_json` 列存储 v2 格式（通过 `trade_guide_version` 字段区分）。
3. 参考位置: `internal/db/migrations/009_add_trade_guide.up.sql`

---

### 综合改进路线图

| 优先级 | 改进项 | 预估工作量 | 预期收益 |
|--------|--------|-----------|---------|
| P0 | 统一到 v2 Schema，简化 LLM 输出 | 2-3天 | 显著降低编译失败率 |
| P0 | Decision Prompt 增加指南强制推理步骤 | 0.5天 | 显著提升指南利用率 |
| P1 | 引入 ATR 自适应阈值替代固定百分比 | 1-2天 | 提升确定性规则质量 |
| P1 | 拆分编译为推理+结构化两阶段 | 2-3天 | 提升 LLM 编译成功率 |
| P1 | 添加 Few-shot 示例和 Schema 验证 | 0.5天 | 提升输出格式正确率 |
| P2 | PolicyGuard 集成指南约束 | 1天 | 消除风控与指南矛盾 |
| P2 | 指南加载状态透明化 | 0.5天 | 改善调试和用户体验 |
| P3 | 引入指南效果追踪和反馈闭环 | 3-5天 | 长期提升指南质量 |
| P3 | 修复 v2 JSON 持久化缺失 | 0.5天 | 消除数据一致性风险 |

## 元信息

本索引覆盖 genFu 项目在提交 `04bd5e16fa7d7052415b46d957b85dac5d212faf`（2026-03-31）时的完整源码分析。重点深入分析了交易指南整合（checklist）系统的实现和问题，涵盖指南生成（stockpicker）、指南编译（tradeguidecompiler）、指南消费（decision）和指南执行（trade_signal）的完整数据流。
