# 组合决策三层接口设计

## 背景

当前系统已经具备以下能力：

- 持仓与交易数据管理
- 新闻简报分析
- 单标的分析与买卖指南生成
- 智能选股选基

现阶段缺口不在于单标的分析能力，而在于缺少一层将“老持仓 + 新候选标的”收束为组合级建议的编排接口。结果是：

- `stockpicker` 能生成候选标的与标的级指南
- 单标的 workflow 能重新分析某个股票/基金
- 但系统无法自然产出账户级调仓建议、目标仓位和动作优先级

因此需要把接口分成三层：

1. 机会发现层：`POST /api/stockpicker`
2. 组合编排层：`POST /api/workflow/portfolio`
3. 人工确认层：`POST /api/decision/portfolio/confirm`

## 设计目标

- 把系统对外定义收束为“决策支持系统”
- 让候选发现、组合建议、人工确认三种职责分离
- 保证老持仓每次都能重新分析，满足实时性要求
- 输出统一的结构化结果，便于 UI 展示、持久化和后续接自动交易

## 非目标

- 当前版本不直接自动下单
- 当前版本不做高频事件驱动调仓
- 当前版本不引入复杂优化器或强化学习仓位分配

## 术语

- `candidate`：候选标的，由智能选股产生
- `existing_position`：当前账户已有持仓标的
- `guide`：标的级买卖指南
- `portfolio_plan`：组合级调仓建议
- `decision_confirmation`：人工确认后的正式决策记录

## 总体调用链

### 手动触发

1. 用户请求 `POST /api/workflow/portfolio`
2. 组合 workflow 拉取账户持仓、现金、历史交易摘要
3. 组合 workflow 拉取最新新闻简报
4. 组合 workflow 调用 `POST /api/stockpicker` 生成候选池
5. 组合 workflow 对“老持仓 + 候选池”逐标的重新分析
6. 组合 workflow 输出调仓建议、目标仓位、优先级、关联指南
7. 用户确认后调用 `POST /api/decision/portfolio/confirm`
8. 系统持久化正式决策记录

### 定时触发

1. 定时任务按账户调用 `POST /api/workflow/portfolio`
2. 返回结果存为组合建议草案
3. 前端展示为“待确认调仓建议”
4. 用户人工确认后再进入确认接口

## 接口一：POST /api/stockpicker

### 职责

生成候选标的，并输出每个候选标的的：

- 候选评分
- 与当前组合的适配度
- 替换提示
- 标的级分析结果
- 标的级买卖指南

### 输入

```json
{
  "account_id": 1,
  "style_profile": {
    "risk_level": "medium",
    "holding_period": "swing",
    "preferred_assets": ["stock", "fund"],
    "sector_preferences": ["消费", "科技"]
  },
  "candidate_scope": {
    "asset_types": ["stock", "fund"],
    "max_candidates": 10,
    "include_existing_positions": false
  },
  "market_context": {
    "news_briefing_id": "brief_20260309_am",
    "regime": "neutral"
  }
}
```

### 输出

```json
{
  "run_id": "sp_20260309_001",
  "generated_at": "2026-03-09T09:30:00+08:00",
  "candidates": [
    {
      "symbol": "510300",
      "name": "沪深300ETF",
      "asset_type": "fund",
      "candidate_score": 0.82,
      "portfolio_fit_score": 0.76,
      "replacement_hint": {
        "can_replace": true,
        "replace_targets": ["600519"]
      },
      "analysis": {
        "technical_view": "趋势修复",
        "news_view": "宏观预期改善",
        "thesis": "适合作为中风险风格下的进攻型候选",
        "risk_tags": ["beta较高"]
      },
      "guide": {
        "guide_id": "guide_510300_20260309",
        "action_bias": "buy",
        "entry_strategy": "分两笔建仓",
        "stop_loss_rule": "跌破关键支撑减半",
        "take_profit_rule": "达到阶段目标后分批止盈"
      }
    }
  ]
}
```

### 内部依赖

- `POST /api/investment`
  - `list_positions`
  - `analyze_pnl`
- `GET /api/news/briefing`
- 现有 stock/fund 分析 workflow

### 内部流程

1. 读取账户当前持仓摘要
2. 读取简报、市场状态、风格参数
3. 生成候选池
4. 对每个候选标的执行标的级分析
5. 计算 `candidate_score` 和 `portfolio_fit_score`
6. 生成标的级指南并返回

### 边界

- 不输出目标仓位
- 不输出最终调仓动作
- 只回答“哪些标的值得关注，以及怎么操作”

## 接口二：POST /api/workflow/portfolio

### 职责

对整个账户执行一次组合级编排，产出：

- 持仓保留/加仓/减仓/清仓建议
- 候选标的新开仓建议
- 目标仓位
- 动作优先级
- 每个动作的理由
- 关联标的指南

### 输入

```json
{
  "account_id": 1,
  "style_profile": {
    "risk_level": "medium",
    "holding_period": "swing",
    "preferred_assets": ["stock", "fund"]
  },
  "constraints": {
    "max_position_ratio": 0.2,
    "max_industry_ratio": 0.4,
    "min_cash_ratio": 0.1,
    "max_turnover_ratio": 0.3
  },
  "trigger": {
    "source": "manual",
    "reason": "user_rebalance_request"
  },
  "candidate_source": {
    "mode": "auto",
    "stockpicker_run_id": ""
  }
}
```

### 输出

```json
{
  "run_id": "pf_20260309_001",
  "generated_at": "2026-03-09T09:35:00+08:00",
  "summary": {
    "portfolio_action": "rebalance",
    "reason": "旧持仓赔率下降，候选池存在更高分替代标的"
  },
  "actions": [
    {
      "symbol": "600519",
      "name": "贵州茅台",
      "source": "existing_position",
      "action": "reduce",
      "priority": 1,
      "current_ratio": 0.25,
      "target_ratio": 0.15,
      "score": 0.58,
      "confidence": 0.71,
      "reason": "集中度过高且短期性价比下降",
      "linked_guide_id": "guide_600519_20260309"
    },
    {
      "symbol": "510300",
      "name": "沪深300ETF",
      "source": "candidate",
      "action": "add",
      "priority": 2,
      "current_ratio": 0.00,
      "target_ratio": 0.10,
      "score": 0.82,
      "confidence": 0.78,
      "reason": "风格匹配度更高且改善组合分散度",
      "linked_guide_id": "guide_510300_20260309"
    }
  ],
  "guides": [
    {
      "guide_id": "guide_600519_20260309",
      "symbol": "600519"
    },
    {
      "guide_id": "guide_510300_20260309",
      "symbol": "510300"
    }
  ]
}
```

### 关键约束

- 老持仓必须重新分析，不允许直接沿用历史结论
- 候选池与老持仓使用同一套标的分析结构
- 组合 workflow 不重复生成标的级指南，只引用已有指南
- 若老持仓缺少最新指南，则 workflow 需要补调标的级分析生成指南

### 内部依赖

- `POST /api/investment`
  - `list_positions`
  - `list_trades`
  - `analyze_pnl`
- `GET /api/news/briefing`
- `POST /api/stockpicker`
- 单标的分析 workflow：对老持仓和候选池统一重分析

### 内部步骤

1. 读取账户事实数据
   - 当前持仓
   - 现金比例
   - 历史交易摘要
   - 当前组合暴露
2. 读取最新新闻简报与市场状态
3. 调用 `stockpicker` 获取候选池
4. 对老持仓标的逐个重新分析
5. 对候选池标的逐个分析
6. 为所有标的形成统一分析卡片
   - `score`
   - `confidence`
   - `risk_tags`
   - `action_hint`
   - `guide_id`
7. 应用组合规则裁决
   - 单标的仓位上限
   - 行业暴露上限
   - 现金下限
   - 最大换手约束
   - 新标的必须显著优于旧持仓才允许换仓
8. 生成调仓动作列表
9. 写入组合建议草案并返回

### 规则层职责

规则层负责最终仓位分配，避免完全把仓位控制交给 LLM：

- LLM 负责解释、thesis、风险标签、置信度
- 规则层负责排序、限仓、替换、目标仓位分配

### 组合动作枚举

- `hold`
- `add`
- `reduce`
- `exit`
- `open`
- `watch`

### 调仓触发条件

硬触发：

- 持仓 thesis 失效
- 风险暴露超限
- 止损或风控条件触发

软触发：

- 候选标的评分显著高于现持仓
- 组合过度集中
- 现金利用率偏低
- 定时复盘发现风格漂移

## 接口三：POST /api/decision/portfolio/confirm

### 职责

接收用户确认后的组合动作，形成正式决策记录。当前版本默认不自动执行交易。

### 输入

```json
{
  "portfolio_run_id": "pf_20260309_001",
  "account_id": 1,
  "confirmed_actions": [
    {
      "symbol": "600519",
      "action": "reduce",
      "target_ratio": 0.15
    },
    {
      "symbol": "510300",
      "action": "add",
      "target_ratio": 0.10
    }
  ],
  "operator": {
    "user_id": "u_001",
    "confirm_note": "接受建议，分批执行"
  },
  "execution_mode": "manual"
}
```

### 输出

```json
{
  "decision_id": "dec_20260309_001",
  "status": "confirmed",
  "execution_mode": "manual",
  "final_actions": [
    {
      "symbol": "600519",
      "action": "reduce",
      "target_ratio": 0.15
    },
    {
      "symbol": "510300",
      "action": "add",
      "target_ratio": 0.10
    }
  ]
}
```

### 内部步骤

1. 校验 `portfolio_run_id` 存在且状态为 `draft`
2. 校验 `confirmed_actions` 必须是原计划动作的子集或受限修改版
3. 写入正式决策记录
4. 将组合建议状态更新为 `confirmed`
5. 若后续开启自动执行，则由独立执行接口消费 `decision_id`

### 边界

- 当前只做确认、校验、持久化
- 不重新跑全量分析
- 不直接执行真实交易

## 三层接口的职责关系

| 层级 | 接口 | 核心对象 | 核心输出 |
|---|---|---|---|
| 机会发现层 | `/api/stockpicker` | 候选标的 | 候选池 + 标的级指南 |
| 组合编排层 | `/api/workflow/portfolio` | 账户组合 | 调仓建议 + 目标仓位 |
| 人工确认层 | `/api/decision/portfolio/confirm` | 已确认动作 | 正式决策记录 |

约束关系：

- `stockpicker` 不能替代 `portfolio workflow`
- `portfolio workflow` 不能跳过人工确认直接视为最终执行
- `confirm` 不能反向修改原始分析，只能接收草案并确认

## 状态模型

### stockpicker run

- `running`
- `completed`
- `failed`

### portfolio workflow run

- `running`
- `draft`
- `confirmed`
- `cancelled`
- `failed`

### decision confirmation

- `confirmed`
- `rejected`
- `expired`

## 错误码

### 通用错误

- `invalid_request`
- `missing_account_id`
- `unsupported_asset_type`
- `style_profile_invalid`
- `constraint_invalid`
- `internal_error`

### stockpicker 错误

- `candidate_generation_failed`
- `candidate_analysis_failed`
- `news_briefing_not_found`

### portfolio workflow 错误

- `positions_not_found`
- `portfolio_analysis_failed`
- `stockpicker_run_not_found`
- `guide_generation_failed`
- `allocation_rule_conflict`

### confirm 错误

- `portfolio_run_not_found`
- `portfolio_run_not_confirmable`
- `confirmed_action_invalid`
- `decision_persist_failed`

## 持久化设计

建议新增三类表。

### 1. stockpicker_runs

- `id`
- `account_id`
- `request_json`
- `result_json`
- `status`
- `created_at`

### 2. portfolio_workflow_runs

- `id`
- `account_id`
- `trigger_source`
- `request_json`
- `result_json`
- `status`
- `created_at`
- `updated_at`

### 3. portfolio_workflow_actions

- `id`
- `run_id`
- `symbol`
- `source`
- `action`
- `priority`
- `current_ratio`
- `target_ratio`
- `score`
- `confidence`
- `reason`
- `guide_id`

现有 `decision_runs` 可继续复用为确认后的正式决策记录。

## 兼容策略

- 现有 `POST /api/stockpicker` 可扩展，不必重命名
- `POST /api/workflow/portfolio` 作为新顶层 workflow 新增
- `POST /api/decision/portfolio/confirm` 作为确认层新增
- 原有单标的 workflow 和 decision 接口保持兼容

## 降级策略

### stockpicker 失败

- `portfolio workflow` 可退化为只分析现持仓
- 输出仅持仓维持/减仓建议，不输出新开仓建议

### 新闻简报失败

- 允许只用持仓事实和技术面继续分析
- 在结果中标注 `briefing_degraded=true`

### 单标的分析部分失败

- 对失败标的标注 `analysis_unavailable`
- workflow 仍继续处理其余标的

## 面试讲解口径

当前系统的核心主线可以统一成：

1. `stockpicker` 发现机会并生成标的级指南
2. `workflow/portfolio` 结合老持仓与候选标的，输出组合级调仓建议
3. 用户人工确认后，`decision/portfolio/confirm` 将其固化为正式决策

这可以回答两个关键问题：

- 为什么智能选股和组合建议不是重复功能
- 为什么系统当前是决策支持系统，而不是自动交易系统

## 后续演进

- 增加事件触发型 `portfolio workflow`
- 引入组合相关性和行业聚类约束
- 在 `confirm` 之后增加独立执行接口
- 将组合建议与执行结果闭环，用于复盘与策略迭代
