# Agent 评测方案与实现骨架

## 目标

为单 Agent、固定 workflow、多 Agent 三种方案提供统一的离线评测框架，回答两个问题：

1. 多 Agent 是否在复杂冲突任务上真的更好
2. 这种提升是否值得额外的成本和复杂度

## 当前实现范围

本次已落地最小可运行骨架：

- `internal/eval`
  - 场景数据集模型
  - 预测结果模型
  - 自动评分 rubric
  - 多系统聚合报告
  - Markdown 摘要输出
- `cmd/eval-report`
  - 从 JSON 读入场景和预测
  - 生成评测报告
- `testdata/eval`
  - 样例场景
  - 样例预测

## 评测对象

统一把每个系统输出转换为 `Prediction`：

- `system`
- `scenario_id`
- `task_type`
- `summary`
- `risk_flags`
- `actions`

其中 `actions` 是结构化动作列表：

- `symbol`
- `action`
- `target_ratio`
- `reasons`

## 场景数据集

每个 `Scenario` 定义：

- `id`
- `task_type`
- `prompt`
- `constraints`
- `expectations`

当前自动评分支持：

- 任务类型是否匹配
- 动作是否违反约束
- 是否覆盖要求的标的
- 是否覆盖要求的风险标签
- 动作是否具备可执行理由

## 自动评分维度

每个场景输出四个子分数：

- `TaskMatch`
- `ConstraintCompliance`
- `Coverage`
- `Actionability`

总分为四项平均值。

这套自动评分只解决“结构正确性”和“约束遵守”，不替代人工质量评审。

## 推荐的完整评测流程

1. 准备历史样本，按任务类型分层：
   - 持仓诊断
   - 调仓替换
   - 新标的发现
   - 冲突样本
2. 用同一批样本跑三套系统：
   - 单 Agent
   - 单 Agent + 固定 workflow
   - 多 Agent
3. 将结果统一转为 `Prediction`
4. 用 `internal/eval` 先跑自动结构评分
5. 再由人工按 rubric 做质量打分
6. 对比：
   - 平均分
   - 任务类型分布
   - 成本和延迟
   - 冲突样本表现

## 为什么要保留人工评审

因为以下问题无法仅靠规则判断：

- 推理是否真正覆盖关键冲突
- 风险分析是否有洞察
- 解释是否自洽
- 动作是否真的符合投资逻辑

所以推荐把自动评分看成第一层 gate，把人工评审看成第二层质量判断。

## 当前命令

Markdown 摘要：

```bash
go run ./cmd/eval-report \
  -scenarios testdata/eval/scenarios.json \
  -predictions testdata/eval/predictions.json
```

JSON 报告：

```bash
go run ./cmd/eval-report \
  -scenarios testdata/eval/scenarios.json \
  -predictions testdata/eval/predictions.json \
  -format json
```

## 当前已知限制

- 还没有真实 runner 直接调用现有 Agent 服务
- 还没有 token/耗时采集
- 还没有人工 rubric 导入
- 还没有 ablation 模式

## 下一步建议

1. 为单 Agent、固定 workflow、多 Agent 分别加适配器
2. 在 `Prediction` 中记录 latency、token、tool calls
3. 引入人工评分表
4. 增加 ablation：去掉 bull/bear/debate 后重新评测
