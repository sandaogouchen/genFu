# 新闻分析系统降级策略

## 概述

当系统未配置 Embedding API Key 时，新闻分析流水线会自动进入降级模式，仍然能够完成基本的新闻分析流程，只是跳过需要 Embedding 的功能。

## 降级行为

### 0. 风控输入升级（新增）

当前系统在原有 L2/L3 之上新增了两条可配置能力：

- `EventImpactAgent`：事件 -> 实体 -> 方向/强度 -> 组合暴露映射 -> 结构化监控信号
- `CausalVerifierAgent`：因果链置信度校验，执行“降权不拦截”策略

两项能力均可通过 `news.pipeline` 配置开关动态启停，不影响历史 `funnel_result.l2_* / l3_analysis` 字段兼容性。

### 1. 新闻采集器 (Collector)

**正常模式：**
- 两级去重：MD5精确去重 + Embedding语义去重（相似度>0.85）

**降级模式：**
- 只使用MD5精确去重
- 跳过语义去重步骤

相关代码：`internal/news/collector.go:282-285`
```go
func (c *Collector) semanticDedup(ctx context.Context, news []RawNews) ([]RawNews, error) {
    if len(news) == 0 || c.embedSvc == nil {
        return news, nil  // 降级：跳过语义去重
    }
    // ... embedding去重逻辑
}
```

### 2. 分类打标器 (Tagger)

**正常模式：**
- 两级分类：关键词规则 + Embedding补充分类
- LLM多维标签生成

**降级模式：**
- 只使用关键词规则分类
- 仍然使用LLM生成多维标签（如果LLM可用）

相关代码：`internal/news/tagger.go:366-403`
```go
// L2降级：只在embedClassifier不为nil时才使用
if t.embedClassifier != nil && (len(kwResult.Domains) == 0 || kwResult.Confidence < 0.6) {
    // Embedding补充分类
}
```

### 3. 漏斗筛选器 (Funnel)

**正常模式：**
- L1: Embedding语义粗筛（锚点匹配，阈值0.40，TopN 30）
- L2: LLM批量关联判断
- L3: 深度因果链分析

**降级模式：**
- 跳过L1（Embedding语义粗筛）
- 直接进入L2（LLM关联判断）
- 仍然执行L3（深度分析）

相关代码：`internal/news/funnel.go:262-291`
```go
func (f *Funnel) Filter(ctx context.Context, events []*NewsEvent) ([]*NewsEvent, error) {
    // 降级：如果embedding不可用，跳过L1
    if f.embedSvc == nil {
        // Fallback: 直接使用L2
        l2Passed, err := f.layerTwo(ctx, events)
        if err != nil {
            return events, nil  // L2失败，返回所有事件
        }
        _ = f.layerThree(ctx, l2Passed)
        return l2Passed, nil
    }
    // ... 正常L1->L2->L3流程
}
```

### 4. 锚点池构建

**正常模式：**
- 从投资组合生成锚点文本
- 调用Embedding API生成向量
- 存储带向量的锚点

**降级模式：**
- 从投资组合生成锚点文本
- 不生成向量（无Embedding）
- L1筛选会被自动跳过

相关代码：`internal/news/funnel.go:110-194`

## 配置示例

### 正常模式配置

```yaml
embedding:
  provider: openai
  api_key: ${EMBEDDING_API_KEY}  # 必须配置
  model: text-embedding-3-small
  base_url: https://api.openai.com/v1
  timeout: 30s

news:
  pipeline_enabled: true
```

### 降级模式配置

```yaml
# embedding配置段可以省略，或api_key留空
# embedding:
#   api_key: ""  # 留空或不配置

news:
  pipeline_enabled: true  # 仍然可以启用
  pipeline:
    event_impact_enabled: true
    causal_verifier_enabled: true
    event_impact_batch_size: 10
    verifier_max_analyze: 5
    verifier_weak_threshold: 0.6
    verifier_invalid_threshold: 0.4
```

## 启动日志

### 正常模式

```
Embedding服务已启用: provider=openai model=text-embedding-3-small
新闻分析流水线已启动
```

### 降级模式

```
警告: 未配置Embedding API Key，将使用降级模式（跳过语义分析和去重）
新闻分析流水线已启动
```

## 功能对比

| 功能 | 正常模式 | 降级模式 |
|------|---------|---------|
| RSSHub新闻抓取 | ✅ | ✅ |
| 噪音过滤 | ✅ | ✅ |
| MD5精确去重 | ✅ | ✅ |
| Embedding语义去重 | ✅ | ❌ |
| 关键词规则分类 | ✅ | ✅ |
| Embedding补充分类 | ✅ | ❌ |
| LLM多维标签 | ✅ | ✅ |
| L1 Embedding筛选 | ✅ | ❌ |
| L2 LLM关联判断 | ✅ | ✅ |
| L3 深度因果分析 | ✅ | ✅ |
| 简报生成 | ✅ | ✅ |

## 性能影响

### 降级模式优势
- **零Embedding成本**：不需要调用Embedding API
- **更快的处理速度**：跳过了向量编码步骤
- **更少的网络依赖**：不依赖外部Embedding服务

### 降级模式劣势
- **可能重复新闻**：无法识别语义相似的不同表述
- **分类准确度降低**：长尾事件类型可能无法准确分类
- **筛选精度降低**：无法通过语义相似度进行精准筛选

## 推荐使用场景

### 适合降级模式
- 测试和开发环境
- 成本敏感场景
- 新闻量较小的投资组合
- 只关注关键词可识别的事件类型

### 推荐正常模式
- 生产环境
- 需要高精度筛选的场景
- 投资组合较为复杂（多产品、竞品、供应链）
- 需要识别长尾事件类型

## 注意事项

1. **首次运行建议**：首次部署建议使用降级模式测试基本流程，确认无误后再配置Embedding API

2. **监控告警**：生产环境建议监控启动日志，确认Embedding服务状态

3. **成本控制**：Embedding API调用会产生费用，建议根据新闻量评估成本

4. **切换模式**：可以随时添加或移除Embedding配置，重启服务即可切换模式

5. **风险输入回退**：若新链路引入噪音，可将 `event_impact_enabled` 或 `causal_verifier_enabled` 设为 `false`，立即回退到旧 L2/L3 行为
