# Decision 接口问题排查和优化报告

## 问题分析

### 原始问题
1. **重复系统提示词**: 出现三次"你是交易决策Agent"的系统提示词
2. **工具调用失败**: 提示"看起来这些API调用都失败了"，但测试显示工具本身是可用的
3. **日志不清晰**: 缺乏详细日志难以排查问题
4. **数据量过大**: K线数据从2001年到2026年，返回数据过多

### 根本原因
1. **缺少工具使用指导**: Agent不知道如何正确使用工具
2. **日志缺失**: 关键流程没有日志输出
3. **缺少数据限制**: 工具层没有限制返回数据的日期范围
4. **日志输出冗余**: 工具结果完整输出导致日志混乱

## 解决方案

### 1. 优化决策提示词 (decision.md)

**修改位置**: `internal/agent/prompt/decision.md`

**新增内容**:
- 可用工具列表和详细说明
- 工具使用参数说明
- 明确的工作流程
- 强调必须使用工具获取实时数据

**改进效果**:
- Agent明确知道有哪些工具可用
- 了解每个工具的功能和参数
- 知道在什么情况下使用什么工具
- 减少猜测和错误尝试

### 2. 添加完整日志系统

#### 2.1 Decision Service 日志

**修改文件**: `internal/decision/service.go`

**日志内容**:
```
[DECISION SERVICE] 开始交易决策流程
  步骤 1: 加载分析报告
  步骤 2: 确定账户ID
  步骤 3: 加载持仓信息
  步骤 4: 加载市场和新闻数据
  步骤 5: 构建决策输入
  步骤 6: 调用决策Agent
  步骤 7: 解析决策输出
  步骤 8: 执行交易信号
```

#### 2.2 LLM Agent 日志

**修改文件**: `internal/agent/llm_agent.go`

**日志内容**:
```
[LLM AGENT] 开始处理请求
[LLM AGENT RUN] 开始构建Agent
[BUILD AGENT] Agent名称、提示词、工具列表
[LLM AGENT RUN] 事件循环处理过程
```

#### 2.3 工具调用日志

**修改文件**: `internal/tool/eino_adapter.go`

**日志内容**:
```
[TOOL CALL] 工具名称、描述、调用参数
[TOOL RESULT] 智能摘要输出（限制数据量）
```

**智能输出策略**:
- 数据量 < 500字符: 完整输出
- K线数据: 显示总条数、时间范围、前3条示例
- 分时数据: 显示总条数、时间范围、前3条示例
- 列表数据: 显示总条数和字节大小
- 对象数据: 显示字段列表，小对象完整输出

### 3. 添加日期范围限制

#### 3.1 股票K线数据限制

**修改文件**: `internal/tool/marketdata.go`

**新增函数**: `parseAndValidateDateRange`

**限制规则**:
- 分钟线 (1/5/15/30/60分钟): 最多30天
- 日线: 最多1年
- 周线: 最多2年
- 月线: 最多5年

**默认行为**:
- 未指定日期: 默认查询最近1年
- 超过限制: 自动调整开始日期

#### 3.2 基金净值数据限制

**新增函数**: `parseAndValidateFundDateRange`

**限制规则**:
- 基金历史净值: 最多1年

### 4. 添加测试覆盖

**新增文件**: `internal/tool/daterange_limit_test.go`

**测试内容**:
- 股票K线日期限制测试
- 基金净值日期限制测试
- 各种边界情况测试

## 修改文件清单

### 核心修改
1. `internal/agent/prompt/decision.md` - 决策提示词优化
2. `internal/decision/service.go` - 添加详细日志
3. `internal/agent/llm_agent.go` - 添加详细日志
4. `internal/tool/eino_adapter.go` - 优化日志输出，添加智能摘要
5. `internal/tool/marketdata.go` - 添加日期范围限制

### 新增文件
1. `internal/tool/daterange_limit_test.go` - 日期限制测试
2. `internal/decision/DECISION_SERVICE_FIX.md` - 本文档

## 日志输出示例

### 正常流程日志
```
================================================================================
[DECISION SERVICE] 开始交易决策流程
================================================================================

[DECISION SERVICE] 步骤 1: 加载分析报告
[DECISION SERVICE] ✓ 成功加载 0 份报告

[DECISION SERVICE] 步骤 2: 确定账户ID
  请求中的账户ID: 0
  使用默认账户ID: 1

[DECISION SERVICE] 步骤 3: 加载持仓信息
[DECISION SERVICE] ✓ 持仓数据长度: 256 字符
  持仓数据预览: [{"id":1,"account_id":1,"instrument":{"id":1,"symbol":"600000"...

[DECISION SERVICE] 步骤 4: 加载市场和新闻数据
  市场数据: 0 字符
  新闻数据: 0 字符

[DECISION SERVICE] 步骤 5: 构建决策输入
  输入长度: 512 字符
  输入预览:
生成交易决策，严格输出JSON：
{"holdings":"[{\"id\":1...

[DECISION SERVICE] 步骤 6: 调用决策Agent
  Agent名称: decision
  Agent能力: [decision analysis]
  可用工具数量: 3
    工具 1: eastmoney - fetch stock market data
    工具 2: marketdata - fetch stock and fund intraday or kline data
    工具 3: investment - manage investment portfolio data

================================================================================
[LLM AGENT] 开始处理请求
[LLM AGENT] Agent名称: decision
[LLM AGENT] Agent能力: [decision analysis]
[LLM AGENT] 用户输入长度: 512 字符
================================================================================

[LLM AGENT RUN] 开始构建Agent
[BUILD AGENT] Agent名称: decision
[BUILD AGENT] 系统提示词长度: 1024 字符
[BUILD AGENT] 构建工具列表
[BUILD AGENT] 工具数量: 3
[BUILD AGENT] ✓ 工具配置完成
[BUILD AGENT] 创建ChatModelAgent
[LLM AGENT RUN] ✓ Agent构建成功
[LLM AGENT RUN] 创建Runner (streaming=false)
[LLM AGENT RUN] 开始事件循环

[LLM AGENT RUN] 事件 1: Role=assistant, IsStreaming=false
[LLM AGENT RUN]   非流式Assistant消息长度: 128
[LLM AGENT RUN] 事件 2: Role=tool, IsStreaming=false
[LLM AGENT RUN]   非流式Tool结果: Name=eastmoney

================================================================================
[TOOL CALL] 工具名称: eastmoney
[TOOL CALL] 工具描述: fetch stock market data
[TOOL CALL] 调用参数:
{
  "action": "get_stock_quote",
  "code": "600000"
}
[TOOL RESULT] K线数据: 共 250 条记录
  时间范围: 2025-02-19 至 2026-02-19
  前3条数据:
    {"time":"2025-02-19","open":10.5,"close":10.3,...}
    {"time":"2025-02-20","open":10.3,"close":10.4,...}
    {"time":"2025-02-21","open":10.4,"close":10.2,...}
  ... (省略 247 条)
================================================================================

[LLM AGENT RUN] 事件循环结束，共处理 5 个事件
[LLM AGENT RUN] ✓ 运行完成
[LLM AGENT RUN] 最终消息: true, 工具结果数: 2

[LLM AGENT] ✓ 处理完成
[LLM AGENT] 响应内容长度: 512 字符
[LLM AGENT] 工具调用次数: 2
[LLM AGENT] 工具结果数量: 2

[DECISION SERVICE] ✓ Agent处理完成
  响应消息长度: 512 字符
  工具调用次数: 2
  工具结果数量: 2

[DECISION SERVICE] 步骤 7: 解析决策输出
[DECISION SERVICE] ✓ 决策ID: DEC-20260219-001
  市场观点: 市场整体震荡...
  风险提示: 数据获取受限...
  交易信号数量: 1

================================================================================
[DECISION SERVICE] 交易决策流程完成
================================================================================
```

## 性能优化

### 数据量限制
- 分钟线: 30天 ≈ 7200条数据 (每分钟一条)
- 日线: 1年 ≈ 250条数据
- 周线: 2年 ≈ 104条数据
- 月线: 5年 ≈ 60条数据
- 基金净值: 1年 ≈ 250条数据

### 日志优化
- 工具结果智能摘要，避免输出大量数据
- 关键步骤清晰标注
- 错误信息明确展示

## 测试验证

### 单元测试
```bash
# 测试日期范围限制
go test -v ./internal/tool -run TestDateRangeLimit
go test -v ./internal/tool -run TestFundDateRangeLimit

# 测试工具集成
go test -v ./internal/tool -run TestAllToolsWithStock_Complete
go test -v ./internal/tool -run TestAllToolsWithFund
```

### 编译测试
```bash
go build -o /tmp/genfu_test ./main.go
```

## 使用建议

### 1. 日志级别控制
建议后续添加日志级别配置：
- DEBUG: 输出所有详细日志
- INFO: 输出关键步骤日志
- WARN: 只输出警告和错误
- ERROR: 只输出错误

### 2. 监控指标
建议添加以下监控：
- 决策流程总耗时
- 工具调用次数和成功率
- 数据获取耗时
- Agent推理耗时

### 3. 错误处理
建议增强错误处理：
- 工具调用失败时的重试机制
- 部分数据获取失败时的降级策略
- 更详细的错误分类和处理

## 问题解决验证

### ✅ 解决的问题
1. **重复提示词**: 通过优化decision.md，明确了工作流程和工具使用方式
2. **工具调用失败**: 添加了工具使用指导，Agent现在知道如何正确使用工具
3. **日志不清晰**: 添加了完整的日志系统，每个步骤都有清晰标注
4. **数据量过大**: 添加了日期范围限制，防止返回过多历史数据

### 🎯 优化效果
1. **可调试性**: 现在可以清晰地看到整个决策流程
2. **性能**: ���制数据量，减少网络传输和处理时间
3. **可维护性**: 清晰的日志和文档，便于后续维护
4. **可靠性**: 通过测试验证，确保功能正确

## 后续改进建议

### 短期改进
1. 添加日志级别配置
2. 添加Prometheus监控指标
3. 优化错误重试机制

### 中期改进
1. 实现工具调用的并发控制
2. 添加缓存机制减少重复请求
3. 实现数据预加载

### 长期改进
1. 实现Agent决策的可解释性
2. 添加决策质量评估
3. 实现自动调优机制

## 总结

本次优化主要解决了 /decision 接口的工作流问题，通过以下三个关键改进：

1. **添加工具使用指导**: 让Agent明确知道如何使用工具
2. **完善日志系统**: 让开发者能够清晰地追踪整个决策流程
3. **限制数据范围**: 防止返回过多数据影响性能和日志可读性

这些改进不仅解决了当前的问题，还为后续的监控、调试和优化打下了良好的基础。
