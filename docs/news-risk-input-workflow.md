# 新闻风控输入工作流（/api/news/analyze）

## 目标

将新闻分析输出从“摘要信息”升级为“可执行风控输入”，在兼容旧字段的前提下新增结构化风险字段。

## 新链路

1. 事件识别：`Tagger + Funnel L2` 识别事件与基础相关度
2. 实体影响：`EventImpactAgent` 输出 `event_entities + impact_mapping`
3. 暴露映射：将影响映射到 `holdings + watchlist`，输出 `exposure_mapping`
4. 因果校验：`CausalVerifierAgent` 输出 `causal_verification`
5. 监控信号：输出 `monitoring_signals_v2`（结构化+文本）

## 兼容策略

- 保留旧字段：
  - `l2_relevance`
  - `l2_affected_assets`
  - `l2_causal_sketch`
  - `l2_priority`
  - `l2_needs_deep`
  - `l2_pass`
  - `l3_analysis`
- 新字段通过 `funnel_result` JSON 扩展，不新增数据库列
- 老记录缺少新字段时按零值处理

## 新字段摘要

- `event_entities`: 事件实体列表
- `impact_mapping`: 事件到实体方向/强度映射（`impact_score + impact_level`）
- `exposure_mapping`: 组合暴露映射（持仓+观察池）
- `causal_verification`: 因果链校验结论（`passed|weak|invalid`）
- `monitoring_signals_v2`: 结构化监控信号

## 校验降权策略

`CausalVerifierAgent` 不硬拦截事件，只做降权：

- `passed -> 1.0`
- `weak -> 0.7`
- `invalid -> 0.4`

降权影响：

- `l2_priority`
- `l2_affected_assets[].confidence`
- `exposure_mapping[].confidence/exposure_score`

## 配置项

位于 `news.pipeline`：

- `event_impact_enabled`（默认 `true`）
- `causal_verifier_enabled`（默认 `true`）
- `event_impact_batch_size`（默认 `10`）
- `verifier_max_analyze`（默认 `5`）
- `verifier_weak_threshold`（默认 `0.6`）
- `verifier_invalid_threshold`（默认 `0.4`）

## 消费建议

1. 风控引擎优先读 `exposure_mapping` 和 `monitoring_signals_v2`
2. 老组件继续读 `l2_* / l3_analysis`
3. 需要回滚时关闭上述开关即可，无需迁移数据库
