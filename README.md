# Go多Agent基础框架

## 运行

```bash
go run .
```

默认端口 8080，可通过 `config.yaml` 修改。

### 配置文件

配置见 [config.yaml](file:///Users/bytedance/Documents/genFu/config.yaml)。

访问控制配置：

```yaml
access:
  enabled: true
  api_keys:
    - "sk-test-123"
  allow_paths:
    - "/healthz"
    - "/docs"
    - "/openapi.json"
```

LLM 配置字段：
- `llm.endpoint`
- `llm.api_key`
- `llm.model`
- `llm.temperature`
- `llm.retry_count`
- `llm.retry_delay`
- `llm.hedge_delay`
- `llm.max_inflight`

LLM 说明：
- 分析类Agent会调用配置的模型完成输出
- 为降低尾部延迟，可配置重试与并发兜底请求（hedge）

### 真实联调测试

默认 `go test ./...` 不会运行真实联调测试（避免外部服务卡住导致超时）。如需运行：

```bash
go test -tags=live -timeout 0 ./...
```

启动时会自动执行迁移（见 [001_init.sql](file:///Users/bytedance/Documents/genFu/internal/db/migrations/001_init.sql)）。

Stock SSE 全链路集成测试：

```bash
GENFU_LIVE_TESTS=1 go test -tags=live ./internal/workflow -run TestStockSSEFullChain -count=1 -v
```

可选环境变量：
- `GENFU_STOCK_SYMBOL` 指定标的代码
- `GENFU_RSSHUB_ROUTE` 指定单条新闻路由（当 config.yaml 未配置 routes 时必填）
- `GENFU_RSSHUB_BASE_URL` 指定可用的 RSSHub base_url（建议自建）

### Docker 部署 RSSHub 并接入

1. 启动 RSSHub（默认 1200 端口）：

```bash
docker run -d --name rsshub -p 1200:1200 diygod/rsshub
```

2. 修改 config.yaml：

```yaml
rsshub:
  base_url: "http://localhost:1200"
  routes:
    - "/v2ex/topics/latest"
```

3. 验证 RSSHub 可用：

```bash
curl http://localhost:1200/v2ex/topics/latest | head -n 5
```

4. 运行集成测试：

```bash
GENFU_LIVE_TESTS=1 go test -tags=live ./internal/workflow -run TestStockSSEFullChain -count=1 -v
```

### 写入示例数据

```bash
sqlite3 genfu.db < internal/db/seed.sql
```

## HTTP

- `GET /healthz` 返回 `ok`
- `POST /api/analyze` 基金/股票走势分析（同步返回）
- `POST /sse/analyze` 基金/股票走势分析（SSE流式返回）
- `POST /api/chat` 对话（SSE默认）
- `POST /sse/chat` 对话（SSE流式返回）
- `GET /api/chat/history` 历史消息
- `POST /api/investment` 持仓与交易操作（工具动作）
- `POST /api/decision` 交易决策（同步返回）
- `POST /sse/decision` 交易决策（SSE流式返回）
- `POST /api/workflow/stock` Stock/Fund 工作流（同步）
- `POST /sse/workflow/stock` Stock/Fund 工作流（SSE流式）
- `GET /openapi.json` OpenAPI 文档
- `GET /docs` Swagger UI

### /api/investment 请求示例

新增或更新持仓

```json
{
  "action": "set_position",
  "account_id": 1,
  "instrument_id": 1,
  "quantity": 200,
  "avg_cost": 12.35,
  "market_price": 12.8
}
```

查询持仓列表

```json
{
  "action": "list_positions",
  "account_id": 1
}
```

查询单条持仓

```json
{
  "action": "get_position",
  "account_id": 1,
  "instrument_id": 1
}
```

删除持仓

```json
{
  "action": "delete_position",
  "account_id": 1,
  "instrument_id": 1
}
```

记录交易（自动更新持仓）

```json
{
  "action": "record_trade",
  "account_id": 1,
  "instrument_id": 1,
  "side": "buy",
  "quantity": 100,
  "price": 12.5,
  "fee": 1.2,
  "trade_at": "2026-02-17T10:05:00+08:00",
  "note": "加仓"
}
```

## WebSocket

连接地址：`ws://localhost:8080/ws/generate`
连接地址：`ws://localhost:8080/ws/chat`

### 请求

```json
{
  "session_id": "s1",
  "user_id": "u1",
  "messages": [
    { "role": "user", "content": "call:echo 你好" }
  ]
}
```

### 投资工具调用

通过以下前缀触发工具调用：`call:investment` / `tool:investment` / `investment:`

示例：创建用户

```json
{
  "messages": [
    { "role": "user", "content": "call:investment {\"action\":\"create_user\",\"name\":\"张三\"}" }
  ]
}
```

示例：创建账户

```json
{
  "messages": [
    { "role": "user", "content": "call:investment {\"action\":\"create_account\",\"user_id\":1,\"name\":\"主账户\",\"base_currency\":\"CNY\"}" }
  ]
}
```

示例：录入标的与交易（会自动更新持仓与均价）

```json
{
  "messages": [
    { "role": "user", "content": "call:investment {\"action\":\"upsert_instrument\",\"symbol\":\"600519\",\"name\":\"贵州茅台\",\"asset_type\":\"stock\"}" },
    { "role": "user", "content": "call:investment {\"action\":\"record_trade\",\"account_id\":1,\"instrument_id\":1,\"side\":\"buy\",\"quantity\":100,\"price\":1200,\"fee\":5,\"trade_at\":\"2026-02-09T00:00:00Z\"}" }
  ]
}
```

示例：盈亏分析

```json
{
  "messages": [
    { "role": "user", "content": "call:investment {\"action\":\"analyze_pnl\",\"account_id\":1}" }
  ]
}
```

## 持仓与交易接口

### 请求示例（同步）

```json
{
  "action": "list_positions",
  "account_id": 1
}
```

### 流式事件

```json
{ "type": "delta", "delta": "收到" }
{ "type": "message", "message": { "role": "assistant", "content": "收到: call:echo 你好" } }
{ "type": "tool_call", "tool_call": { "name": "echo", "arguments": { "text": "call:echo 你好" } } }
{ "type": "tool_result", "tool_result": { "name": "echo", "output": "call:echo 你好" } }
{ "type": "done", "done": true }
```

### 取消

```json
{ "type": "cancel" }
```

### 分析Agent

关键词触发：
- 基金经理分析：包含“基金编号”或“基金经理”
- 多头分析：包含“多头”或“看涨”
- 空头分析：包含“空头”或“看跌”
- 多空辩论：包含“辩论”或“多空”

提示词文件：
- [fund_manager.md](file:///Users/bytedance/Documents/genFu/internal/agent/prompt/fund_manager.md)
- [bull.md](file:///Users/bytedance/Documents/genFu/internal/agent/prompt/bull.md)
- [bear.md](file:///Users/bytedance/Documents/genFu/internal/agent/prompt/bear.md)
- [debate.md](file:///Users/bytedance/Documents/genFu/internal/agent/prompt/debate.md)

示例：基金经理分析

```json
{
  "messages": [
    { "role": "user", "content": "基金编号 123456，分析基金经理" }
  ]
}
```

示例：多头分析

```json
{
  "messages": [
    { "role": "user", "content": "多头观点：该基金后续会涨吗？" }
  ]
}
```

示例：空头分析

```json
{
  "messages": [
    { "role": "user", "content": "空头观点：风险点有哪些？" }
  ]
}
```

示例：多空辩论

```json
{
  "messages": [
    { "role": "user", "content": "多空辩论：多头观点xxx，空头观点yyy" }
  ]
}
```

## 基金/股票走势分析接口

### 请求示例（同步）

```json
{
  "type": "fund",
  "symbol": "110022",
  "name": "易方达消费",
  "kline": "最近20日K线摘要...",
  "manager": "基金经理信息摘要"
}
```

### 响应字段

- steps: 依次包含 kline / manager / bull / bear / debate / summary
- summary: 最终文档
- report_id: 分析报告ID（可用于后续交易决策）

### SSE示例

```bash
curl -N -X POST http://localhost:8080/sse/analyze -H 'Content-Type: application/json' -d '{
  "type":"stock",
  "symbol":"600519",
  "name":"贵州茅台",
  "kline":"最近20日K线摘要..."
}'
```

SSE错误事件会包含 `error` 与 `step` 字段，便于定位失败步骤。

## 交易决策接口

说明：前端只需提供分析报告ID（可选）与账户ID（可选），持仓/大盘/新闻由后端获取。新闻仅使用 config.news.keywords 进行关键词检索，未命中则不携带新闻。

### 请求示例（同步）

```json
{
  "report_ids": [101, 102],
  "account_id": 1
}
```

### SSE示例

```bash
curl -N -X POST http://localhost:8080/sse/decision -H 'Content-Type: application/json' -d '{
  "report_ids": [101, 102],
  "account_id": 1
}'
```

## 目录结构

- `internal/message` 消息与角色定义
- `internal/generate` 生成请求、响应与事件结构
- `internal/tool` 工具注册、调用与示例工具
- `internal/agent` Agent接口与示例实现
- `internal/router` 多Agent路由
- `internal/ws` WebSocket生成处理
- `internal/server` HTTP服务注册
- `internal/db` PostgreSQL连接与迁移
- `internal/investment` 投资域模型、仓储与服务
- `internal/agent/prompt` 分析Agent提示词
- `internal/agent/*` 各Agent代码实现目录
- `internal/llm` 模型调用客户端
- `internal/analyze` 走势分析编排与接口
- `internal/decision` 交易决策编排与接口
- `internal/trade_signal` 交易信号解析与执行

## 真实数据测试

需要联网、SQLite 与真实 LLM 配置，默认需要显式开启。

### RSSHub 真实数据测试

```bash
export GENFU_LIVE_TESTS=1
export RSSHUB_BASE_URL="https://rsshub.app"
export RSSHUB_ROUTE="/aicaijing/latest"
go test ./internal/rsshub -run TestFetchLive
```

### 每日复盘真实数据测试

```bash
export GENFU_LIVE_TESTS=1
export GENFU_DB_DSN="file:genfu.db?_foreign_keys=on"
export GENFU_LLM_ENDPOINT="https://aihubmix.com/v1"
export GENFU_LLM_API_KEY="sk-xxx"
export GENFU_LLM_MODEL="gemini-3-flash-preview"
export GENFU_LLM_TIMEOUT="60s"
export GENFU_LLM_RETRY_DELAY="300ms"
export GENFU_LLM_HEDGE_DELAY="0s"
go test ./internal/analyze -run TestDailyReviewLive
```
