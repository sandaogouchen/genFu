# genFu 面试准备报告

## 1. 项目定位

`genFu` 不是单纯的聊天机器人，而是一个面向投资决策支持的 Go 多 Agent 系统。它把以下能力放到同一个后端里：

- 对话入口与意图路由
- 投资组合数据管理
- 股票/基金分析工作流
- 交易决策与风控执行
- 选股与交易指南生成
- 新闻采集、事件识别、漏斗筛选与简报生成

从工程实现看，这个项目的核心价值不在“接了一个大模型”，而在于把 LLM、工具、确定性规则、持久化、SSE 流式观测、降级策略整合成了一个可运行的决策系统。

## 2. 总体架构

启动入口在 [main.go](/Users/bytedance/Documents/genFu/main.go#L47)。启动流程大致是：

1. 初始化 `tool.Registry`
2. 加载配置、连接数据库、执行迁移
3. 创建 `investment/news/analyze/decision/stockpicker/chat` 等 repository 与 service
4. 构建 `EinoChatModel`
5. 构建多个基于 prompt 的 LLM agent
6. 注册工具，例如 `investment`、`eastmoney`、`marketdata`、`rsshub`
7. 组装上层服务：
   - `chat.Service`
   - `decision.Service`
   - `stockpicker.Service`
   - `workflow.StockWorkflow`
   - `news.Pipeline`
8. 注册 HTTP / SSE / WS 路由

这个启动方式的优点是依赖关系非常清楚：底层是数据库和工具，中层是领域 service，上层是 workflow、chat 和 API。

## 3. “LangGraph” 在这个项目里的真实形态

这个项目没有直接使用 Python 版 `LangGraph`。它在 Go 中自己实现了“图式编排”的核心思想。

### 3.1 Graph 定义

最明确的 graph 定义在 [internal/workflow/workflow_planner.go](/Users/bytedance/Documents/genFu/internal/workflow/workflow_planner.go#L5)：

- `holdings`
- `holdings_market`
- `target_market`
- `news_fetch`
- `news_summary`
- `bull`
- `bear`
- `debate`
- `summary`

这里的 `workflowNodeOrder` 就是图的固定拓扑顺序，[Plan](/Users/bytedance/Documents/genFu/internal/workflow/workflow_planner.go#L61) 会根据输入和 prompt 动态裁剪节点：

- 没有持仓仓库或用户要求“忽略持仓”时，跳过持仓节点
- 没有新闻源或用户要求“忽略新闻”时，跳过新闻节点
- 用户要求只看多头/空头时，关闭另一侧与 debate 节点

所以它不是“全动态 DAG”，而是“固定骨架 + 条件裁剪”的 workflow graph。

### 3.2 Graph 执行

执行入口在 [internal/workflow/stock_workflow.go](/Users/bytedance/Documents/genFu/internal/workflow/stock_workflow.go#L81)。

执行方式非常直白：

1. 先 `resolveWorkflowInstrument`
2. 再 `planner.Plan`
3. 按节点顺序逐个执行
4. 每个节点执行前后通过 `NodeStreamer` 发事件

这段实现很像 LangGraph 的 `plan -> node execution -> conditional edge -> stream events`，但没有引入复杂 runtime，而是用 Go 代码把控制流写死，换来更强的可控性和更低的调试成本。

### 3.3 为什么这样设计

这是很好的面试点：

- 投资分析链路的节点结构相对稳定，不需要通用图引擎的复杂度
- 业务希望严格控制节点顺序、跳过条件和输出格式
- SSE 需要把每个 node 的开始、跳过、增量、完成显式暴露给前端
- 直接手写 orchestration 比通用框架更容易做日志、回放和排障

一句话总结就是：它采用了 LangGraph 的思想，但没有为“框架感”牺牲工程可控性。

## 4. Workflow 细节实现

### 4.1 节点输入准备

`StockWorkflow` 会先做标的归一化，[resolveWorkflowInstrument](/Users/bytedance/Documents/genFu/internal/workflow/stock_workflow.go#L575) 支持两种输入：

- 如果输入本身像证券代码，直接使用
- 如果更像基金名，则调用 `investment.search_instruments` 做检索，再找精确匹配

这一步很重要，因为它把“用户自然输入”和“后续工具可执行参数”桥接起来了。

### 4.2 工具节点

工作流前半段主要是确定性工具节点：

- [buildHoldings](/Users/bytedance/Documents/genFu/internal/workflow/stock_workflow.go#L313): 从仓库拿持仓并计算组合占比
- [buildHoldingsMarket](/Users/bytedance/Documents/genFu/internal/workflow/stock_workflow.go#L356): 为持仓逐个补行情
- [buildTargetMarket](/Users/bytedance/Documents/genFu/internal/workflow/stock_workflow.go#L364): 查询目标标的行情
- [fetchNews](/Users/bytedance/Documents/genFu/internal/workflow/stock_workflow.go#L464): 通过 `rsshub` 工具按 route 拉新闻
- [summarizeNews](/Users/bytedance/Documents/genFu/internal/workflow/stock_workflow.go#L725): 再用 LLM 摘要成 `summary + sentiment`

其中 [fetchQuoteByAsset](/Users/bytedance/Documents/genFu/internal/workflow/stock_workflow.go#L438) 体现了一个很实用的工程技巧：如果 `asset_type` 不明确，会先尝试股票报价，再尝试基金估值，最后把双失败信息拼成可诊断错误。

### 4.3 Agent 节点

工作流后半段是多 Agent 协作：

- `bullAgent` 负责多头观点
- `bearAgent` 负责空头观点
- `debateAgent` 负责多空交锋
- `summaryAgent` 负责收敛最终结论

这些 agent 本质上都是 `prompt file + Eino model + tool registry` 的组合，构建方式见 [internal/agent/llm_agent.go](/Users/bytedance/Documents/genFu/internal/agent/llm_agent.go#L213)。

这比“一个超长 prompt 全干完”更容易做角色隔离，也更方便做 ablation 和评测。

## 5. Agent 体系设计

### 5.1 Agent 抽象

最基础接口非常轻量，见 [internal/agent/agent.go](/Users/bytedance/Documents/genFu/internal/agent/agent.go#L8)：

- `Name()`
- `Capabilities()`
- `Handle()`

这意味着项目没有把 agent 设计成一个过度抽象的框架，而是让它保持“可替换的处理单元”。

### 5.2 LLM Agent 的通用实现

通用实现是 [LLMAgent](/Users/bytedance/Documents/genFu/internal/agent/llm_agent.go#L22)：

- 从 prompt 文件加载系统提示词
- 通过 `tool.BuildEinoTools` 绑定工具
- 用 `adk.NewChatModelAgent` 创建 agent
- 统一处理同步/流式输出
- 解析工具调用与工具结果

比较关键的点：

- [buildAgent](/Users/bytedance/Documents/genFu/internal/agent/llm_agent.go#L213) 会把整个 registry 转成 Eino Tools
- [streamAssistant](/Users/bytedance/Documents/genFu/internal/agent/llm_agent.go#L317) 支持边输出文本边组装流式 tool call
- [buildToolMeta](/Users/bytedance/Documents/genFu/internal/agent/llm_agent.go#L418) 会把工具执行结果挂到 response meta，供上层 service 继续使用

### 5.3 Prompt 驱动 + 角色隔离

例如 [bull prompt](/Users/bytedance/Documents/genFu/internal/agent/prompt/bull.md#L1) 明确限制了：

- 只从看多立场分析
- 可调用行情/资讯工具
- 不要调用持仓查询工具

这类约束能减少多 agent 之间的信息串味，是典型的“角色 prompt 工程”。

## 6. Tool 工具注册与调用链

### 6.1 工具注册

底层工具中心是 [Registry](/Users/bytedance/Documents/genFu/internal/tool/registry.go#L32)：

- `Register`
- `Get`
- `List`
- `Execute`

启动阶段在 [main.go](/Users/bytedance/Documents/genFu/main.go#L48) 统一注册：

- `investment`
- `eastmoney`
- `marketdata`
- `cninfo`
- `brief_search`
- `rsshub`
- `stockpicker` 的策略工具

这个设计的优点是所有工具对上层都暴露同一个接口：`ToolSpec + Execute(ctx,args)`。

### 6.2 Tool Calling 适配

项目没有直接把本地工具暴露给 LLM，而是先做了一层 Eino 适配，见 [internal/tool/eino_adapter.go](/Users/bytedance/Documents/genFu/internal/tool/eino_adapter.go#L13)。

链路是：

1. `ToolSpec` 转 `schema.ToolInfo`
2. `BuildEinoTools` 按 allowlist 生成 Eino tool 集合
3. `InvokableRun` 把 LLM 传来的 JSON 参数转成 `map[string]interface{}`
4. 调用本地 `Tool.Execute`
5. 把 `ToolResult` 再编码成 JSON 字符串返回给模型 runtime

这里最有价值的实现点有两个：

- [BuildEinoTools](/Users/bytedance/Documents/genFu/internal/tool/eino_adapter.go#L138) 支持工具白名单过滤
- [InvokableRun](/Users/bytedance/Documents/genFu/internal/tool/eino_adapter.go#L30) 做了统一日志和错误归一

### 6.3 Tool 调用的上层约束

在 chat 场景里，并不是所有意图都能随便调工具。`chat.Service` 会先根据意图确定允许工具，再用 [resolveAllowedTools](/Users/bytedance/Documents/genFu/internal/chat/service.go#L450) 进行过滤，最后在 [newAgent](/Users/bytedance/Documents/genFu/internal/chat/service.go#L527) 里只把被允许的工具注入 agent。

这说明项目已经考虑了“能力最小化暴露”。

### 6.4 典型工具实现

`investment` 是最核心的业务工具之一，见 [internal/tool/investment.go](/Users/bytedance/Documents/genFu/internal/tool/investment.go#L113)。

它的特点是一个 tool 下面挂很多 action：

- 持仓增删改查
- 交易记录
- 组合快照
- 基金持仓列表
- `search_instruments`

其中 [search_instruments](/Users/bytedance/Documents/genFu/internal/tool/investment.go#L402) 同时融合基金搜索和股票代码搜索，是 workflow 标的归一化的重要支撑。

## 7. Chat 编排与会话记忆

### 7.1 意图路由

`chat.Service` 不是直接把消息喂给大模型，而是先走意图路由，见 [routeTurn](/Users/bytedance/Documents/genFu/internal/chat/service.go#L260) 和 [IntentRouterAgent](/Users/bytedance/Documents/genFu/internal/chat/intent_router_agent.go#L62)。

路由能力包括：

- `general_chat`
- `portfolio_ops`
- `decision`
- `stockpicker`

如果输入是 `call:` / `tool:` 前缀，会直接判成 portfolio tool call，这是很实用的 shortcut。

### 7.2 会话记忆压缩

项目没有无限拼接历史消息，而是用 [SessionMemoryAgent](/Users/bytedance/Documents/genFu/internal/chat/session_memory_agent.go#L21) 做摘要压缩，再在下一轮通过系统提示词注入摘要，[buildModelInput](/Users/bytedance/Documents/genFu/internal/chat/service.go#L291)。

这个设计解决了两个问题：

- 控制上下文长度
- 保留用户目标、关键约束、未完成事项

这是一种典型的“memory summarization”模式，面试里可以作为 token 成本优化点来讲。

## 8. Decision Service 设计

`decision.Service` 是一个很完整的“决策到执行”闭环，入口在 [Decide](/Users/bytedance/Documents/genFu/internal/decision/service.go#L130)。

执行步骤可以直接背：

1. 加载分析报告
2. 确定账户
3. 加载持仓与已选指南
4. 加载市场和新闻摘要
5. 调用决策 Agent
6. 解析标准化决策 JSON
7. 调用 planner agent 生成可执行订单
8. 调用 policy guard 做风控门禁
9. 调用交易引擎执行
10. 调用 review agent 做执行后复盘

这条链路体现了很强的工程意识：LLM 只负责生成“候选决策”，真正落地前还有 planner、policy guard、execution、review 四层。

几个关键点：

- [planExecution](/Users/bytedance/Documents/genFu/internal/decision/service.go#L315) 支持 planner agent 存在时做二次结构化，否则回退到信号直转订单
- [guardOrders](/Users/bytedance/Documents/genFu/internal/decision/service.go#L451) 支持无风控时默认批准，但保留 GuardedOrder 抽象
- [executeGuardedOrders](/Users/bytedance/Documents/genFu/internal/decision/service.go#L469) 对被 block 的订单与 engine 缺失的场景都做了状态落盘
- [generateReview](/Users/bytedance/Documents/genFu/internal/decision/service.go#L558) 把 `tool_results` 也纳入复盘上下文

这套设计很适合回答“如何避免 LLM 直接控制交易”的问题。

## 9. StockPicker 多 Agent 设计

`stockpicker.Service` 是另一个非常适合面试展开的模块，入口在 [PickStocks](/Users/bytedance/Documents/genFu/internal/stockpicker/service.go#L63)。

它的链路是：

1. 准备市场数据、新闻、持仓、股票池
2. `RegimeAgent` 判断市场状态
3. `ScreenerAgent` 生成筛选策略
4. 执行确定性筛选，得到候选池
5. `AnalyzerAgent` 深度分析候选
6. `PortfolioFitAgent` 做组合约束重排
7. `TradeGuideCompilerAgent` 编译交易指南
8. 持久化 run snapshot 与 operation guide

这里的核心不是“多 agent 数量多”，而是“生成式节点 + 确定性节点交替”：

- 市场 regime、策略生成、深度分析、组合适配、交易指南编译由 LLM 做
- 候选筛选、仓位建议、持久化、回退策略由代码控制

这比完全让模型自己筛股更稳。

## 10. 新闻 Pipeline 设计

新闻主链路在 [internal/news/pipeline.go](/Users/bytedance/Documents/genFu/internal/news/pipeline.go#L18)：

- `Collector`
- `Tagger`
- `Funnel`
- `Briefing Generator`

[Run](/Users/bytedance/Documents/genFu/internal/news/pipeline.go#L115) 的标准路径是：

1. 采集新闻
2. 分类打标
3. 三层漏斗过滤
4. 生成 briefing
5. 内存存储 + DB 持久化

### 10.1 Collector 优化

新闻采集器做了三件事：

- 多源并发采集 [Collect](/Users/bytedance/Documents/genFu/internal/news/collector.go#L143)
- 噪音过滤 [defaultNoiseRules](/Users/bytedance/Documents/genFu/internal/news/collector.go#L203)
- 两级去重：MD5 精确去重 + Embedding 语义去重 [semanticDedup](/Users/bytedance/Documents/genFu/internal/news/collector.go#L295)

语义去重不是简单 pairwise 去重，而是贪心聚类后选择信息量最长的代表，并记录 `RelatedSources`，这点很有工程含量。

### 10.2 Funnel 设计

`Funnel` 的关键思想是三层过滤：

- L1: Embedding 粗筛
- L2: LLM 相关性判断
- L3: 深度因果分析

而且在 [Filter](/Users/bytedance/Documents/genFu/internal/news/funnel.go#L381) 中内建了降级策略：

- 没有 embedding 时直接跳过 L1
- L2 失败时退回 L1 结果
- L3 失败不阻断主链路

这说明项目在一开始就考虑了“模型能力可选、主流程不能中断”。

## 11. 主要优化点

### 11.1 LLM 客户端层优化

`EinoChatModel` 在 [internal/llm/eino_model.go](/Users/bytedance/Documents/genFu/internal/llm/eino_model.go#L24) 做了几类优化：

- `retryCount + retryDelay` 控制重试
- `sem chan struct{}` 限制最大并发 inflight
- 同步和流式各自实现重试策略
- 自己组 OpenAI 风格 `tools` 和 `tool_choice`

尤其值得讲的是流式 tool call 处理：

- [mergeEinoToolCalls](/Users/bytedance/Documents/genFu/internal/llm/eino_model.go#L405)
- [mergeToolCallArguments](/Users/bytedance/Documents/genFu/internal/llm/eino_model.go#L481)
- [normalizeToolCallArguments](/Users/bytedance/Documents/genFu/internal/llm/eino_model.go#L503)

这部分是在处理不同 provider 流式协议不一致的问题，是很典型、也很难调的兼容层。

### 11.2 工具返回数据控量

`marketdata` 在 [parseAndValidateDateRange](/Users/bytedance/Documents/genFu/internal/tool/marketdata.go#L409) 和 [parseAndValidateFundDateRange](/Users/bytedance/Documents/genFu/internal/tool/marketdata.go#L565) 对 K 线/净值查询做了时间范围硬限制，避免模型一次拉几十年的数据。

这是非常实际的 token/延迟优化：

- 分钟线最多 30 天
- 日线最多 1 年
- 周线最多 2 年
- 月线最多 5 年
- 基金历史最多 1 年

### 11.3 EastMoney 可用性优化

`eastmoney` 工具在 [fetchStockList](/Users/bytedance/Documents/genFu/internal/tool/eastmoney.go#L209) 做了分片拉取和 best-effort 降级，在 [doEastMoneyRequest](/Users/bytedance/Documents/genFu/internal/tool/eastmoney.go#L402) 做了重试，在 [candidateHostsForEndpoint](/Users/bytedance/Documents/genFu/internal/tool/eastmoney.go#L631) 做了 endpoint-aware host fallback。

这是对外部数据源不稳定性的典型防御性设计。

### 11.4 可观测性优化

工具执行日志见 [internal/tool/eino_adapter.go](/Users/bytedance/Documents/genFu/internal/tool/eino_adapter.go#L39)，Decision 日志见 [internal/decision/service.go](/Users/bytedance/Documents/genFu/internal/decision/service.go#L138)，Workflow 流式事件见 [internal/workflow/stock_workflow.go](/Users/bytedance/Documents/genFu/internal/workflow/stock_workflow.go#L93) 和 [internal/workflow/node_streamer.go](/Users/bytedance/Documents/genFu/internal/workflow/node_streamer.go#L15)。

这让系统具备：

- node 级别可见性
- tool call 级别可见性
- agent 级别可见性

对于多 agent 系统，这是非常关键的。

## 12. 我认为最有挑战的部分

### 12.1 流式 tool calling 兼容层

这是我认为最难的一块。原因不是“写代码复杂”，而是 provider 差异很大：

- 有的 provider 增量返回 arguments
- 有的 provider 累积返回 arguments
- 有的 tool call JSON 会被拆断
- 流式文本与流式 tool call 可能交错出现

项目通过 `mergeToolCallArguments + normalizeToolCallArguments` 解决了一部分兼容问题，这部分非常适合面试里讲成“踩坑经验”。

### 12.2 多 Agent 输出的结构化约束

无论是 `decision`、`regime`、`portfolio_fit` 还是 `trade_guide_compiler`，都要求 LLM 严格输出 JSON，然后代码再做解析、裁剪、补缺和 fallback。

难点在于：

- prompt 要足够严格
- parser 要容忍 markdown code fence、脏前后缀
- 失败时要能降级，而不是整条链路崩掉

### 12.3 生成式与确定性逻辑混编

这个项目最成熟的地方就是：把 LLM 放在“擅长生成判断”的环节，把“必须稳定”的环节收回代码层。

例如：

- 工作流节点执行顺序是代码定的
- 工具调用接口是代码定的
- 风控门禁和执行状态是代码定的
- operation guide 落库格式是代码定的

这类混编通常比单纯 prompt 工程更难，因为边界设计要非常清楚。

## 13. 面试时可直接输出的项目亮点

### 亮点 1

我不是把 LLM 当成黑盒问答，而是把它放到一个可控的多阶段决策系统里，外面包了 tool registry、workflow planner、policy guard、execution engine 和 review agent。

### 亮点 2

项目虽然没有直接用 LangGraph，但在 Go 里实现了固定图骨架 + 条件裁剪 + 节点流式事件的 workflow 编排，业务上更可控，调试也更直接。

### 亮点 3

我重点做了工具调用工程化，包括工具注册、Eino 适配、白名单注入、流式 tool call 解析、日志摘要、结果回写 meta。

### 亮点 4

我为外部依赖和模型能力不足设计了大量 fallback，例如 EastMoney host fallback、stock list best-effort、news embedding 降级、portfolio fit 降级、trade guide v2 回退到 v1。

## 14. 面试官可能会追问的问题

### Q1. 为什么不用真正的 LangGraph？

可以回答：

因为这里的节点拓扑相对稳定，业务更需要确定性顺序、严格的节点输出和 SSE 可观测性。手写 workflow 的维护成本反而更低，调试也更直接。

### Q2. 为什么需要多 Agent，不是一个 prompt 就行吗？

可以回答：

多 Agent 的目的不是“拆着好看”，而是隔离角色和约束。例如 bull/bear/debate 明确从不同立场给观点，decision/planner/review 也是不同职责，这样更容易做结构化输出和后续评测。

### Q3. 工具调用最难的地方是什么？

可以回答：

不是注册工具本身，而是流式 tool call 的兼容、参数组装、错误回传、日志观测和工具结果再利用。尤其不同模型服务对 tool call chunk 的协议不一致，这部分必须自己兜底。

### Q4. 你怎么控制 LLM 不乱下单？

可以回答：

LLM 只生成决策或候选订单，真正落地前还有 planner、policy guard 和 execution engine。模型不是直接执行交易，而是在受控状态机里提供结构化建议。

## 15. 一页式总结

如果只能用一分钟总结这个项目，我会这样说：

> 这是一个 Go 实现的投资决策多 Agent 系统。底层通过统一 tool registry 接入行情、持仓、新闻等实时工具；中间层用 Eino 适配 LLM 和 tool calling；上层通过手写 workflow graph 把持仓、行情、新闻、多空分析、辩论和总结串成可流式观测的决策链。同时系统还实现了意图路由、会话记忆、选股多 agent 编排、交易决策闭环，以及面向外部依赖失败和模型不稳定的多层 fallback。它的难点不在于“接模型”，而在于把生成式能力纳入一个可控、可观测、可降级的工程系统。

