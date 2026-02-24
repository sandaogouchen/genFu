# 工具集成测试文档

## 概述

本测试套件用于验证所有金融工具（股票和基金）的完整功能，确保数据获取、处理和输出的正确性。

## 测试文件

- `tools_stock_integration_test.go` - 股票工具集成测试
- `tools_integration_test.go` - 基金工具集成测试

## 测试功能

### 股票工具测试 (TestAllToolsWithStock_Complete)

测试股票代码：`600000` (浦发银行)

#### EastMoney 工具
- `get_stock_quote` - 获取股票实时行情
  - 功能：获取股票代码、名称、价格、涨跌额、涨跌幅、成交额
  - 验证：所有字段完整且有效

#### MarketData 工具
- `get_stock_kline` - 获取股票历史K线数据
  - 功能：获取开盘价、收盘价、最高价、最低价、成交量
  - 验证：OHLC数据完整性，时间字段存在，价格合理性

- `get_stock_intraday` - 获取股票分时行情
  - 功能：获取每分钟价格、成交量、均价
  - 验证：时间字段存在，价格有效

#### Investment 工具
- `create_user` - 创建投资用户账户
- `create_account` - 创建投资账户
- `upsert_instrument` - 创建或更新股票工具
- `set_position` - 设置股票持仓
- `list_positions` - 查询持仓列表
- `get_portfolio_summary` - 获取投资组合摘要

### 基金工具测试 (TestAllToolsWithFund)

测试基金代码：`000001` (华夏成长)

#### MarketData 工具
- `get_fund_kline` - 获取基金历史净值数据
  - 功能：获取日期、单位净值、累计净值
  - 验证：净值数据完整，时间连续

- `get_fund_intraday` - 获取基金实时估值
  - 功能：获取估算净值、实际净值、估值时间
  - 验证：净值数据有效

#### Investment 工具
- `create_user` - 创建投资用户账户
- `create_account` - 创建投资账户
- `upsert_instrument` - 创建或更新基金工具
- `set_position` - 设置基金持仓
- `list_fund_holdings` - 查询基金持仓列表
- `record_trade` - 记录交易
- `list_trades` - 查询交易记录
- `get_portfolio_summary` - 获取投资组合摘要

## 测试输出格式

### 单个测试输出
```
✓ [工具名] 功能描述 - 成功获取 N 条数据
✗ [工具名] 功能描述 - 失败: 错误原因
```

### 详细数据输出
```
功能说明: ...
示例数据 (前N条):
[
  {
    "field1": "value1",
    "field2": "value2",
    ...
  }
]
```

### 总结报告
```
================================================================================
测试总结报告
================================================================================

[工具名] M/N 功能可用
  不可用功能: func1, func2
  详细结果:
    ✓ function1 (描述) - 数据点: N
    ✗ function2 (描述) - 错误: error message

--------------------------------------------------------------------------------
总计: X/Y 功能可用
成功: X, 失败: Y
================================================================================
```

## 测试结果

### 股票工具测试结果 (最近运行)
```
总计: 9/9 功能可用
成功: 9, 失败: 0

[eastmoney] 1/1 功能可用
  ✓ get_stock_quote - 获取股票实时行情

[marketdata] 2/2 功能可用
  ✓ get_stock_kline - 获取股票历史K线数据
  ✓ get_stock_intraday - 获取股票分时行情

[investment] 6/6 功能可用
  ✓ create_user - 创建投资用户账户
  ✓ create_account - 创建投资账户
  ✓ upsert_instrument - 创建或更新投资工具
  ✓ set_position - 设置股票持仓
  ✓ list_positions - 查询账户持仓
  ✓ get_portfolio_summary - 获取投资组合摘要
```

### 基金工具测试结果 (最近运行)
```
总计: 10/10 功能可用
成功: 10, 失败: 0

[marketdata] 2/2 功能可用
  ✓ get_fund_kline - 获取基金历史净值数据
  ✓ get_fund_intraday - 获取基金实时估值

[investment] 8/8 功能可用
  ✓ create_user - 创建投资用户账户
  ✓ create_account - 创建投资账户
  ✓ upsert_instrument - 创建或更新投资工具
  ✓ set_position - 设置基金持仓
  ✓ list_fund_holdings - 查询基金持仓
  ✓ record_trade - 记录交易
  ✓ list_trades - 查询交易记录
  ✓ get_portfolio_summary - 获取投资组合摘要
```

## 运行测试

### 运行股票测试
```bash
go test -v ./internal/tool -run TestAllToolsWithStock_Complete -timeout 60s
```

### 运行基金测试
```bash
go test -v ./internal/tool -run TestAllToolsWithFund -timeout 60s
```

### 运行所有集成测试
```bash
go test -v ./internal/tool -run "TestAllToolsWith" -timeout 120s
```

### 跳过集成测试（短模式）
```bash
go test -short ./internal/tool
```

## 数据验证规则

### 股票数据验证
1. 股票代码和名称必须存在
2. 价格必须大于0
3. K线数据必须包含有效的 OHLC (开高低收)
4. 最高价 >= 最低价
5. 时间字段必须存在

### 基金数据验证
1. 净值必须大于0
2. 时间字段必须存在
3. 数据连续性检查

### 投资数据验证
1. 用户、账户创建成功
2. 持仓数据完整性
3. 交易记录正确性
4. 投资组合摘要计算正确

## 测试覆盖的工具

### EastMoney Tool
- 股票实时行情查询

### MarketData Tool
- 股票K线数据获取
- 股票分时数据获取
- 基金净值数据获取
- 基金实时估值获取

### Investment Tool
- 用户和账户管理
- 投资工具管理
- 持仓管理
- 交易记录
- 投资组合分析

## 注意事项

1. 测试需要网络连接（访问东方财富等数据源）
2. 测试使用临时数据库，测试完成后自动清理
3. 测试包含真实的股票和基金代码
4. 测试超时设置为60秒
5. 数据验证严格，任何字段缺失或无效都会导致测试失败

## 故障排查

### 常见错误

1. **网络超时**
   - 检查网络连接
   - 增加超时时间

2. **数据源不可用**
   - 东方财富API可能临时不可用
   - 稍后重试

3. **数据库迁移失败**
   - 检查工作目录设置
   - 确保数据库配置正确

4. **数据验证失败**
   - 检查API返回格式是否变化
   - 更新数据解析逻辑

## 扩展测试

可以添加更多测试用例：
- 多个股票/基金代码测试
- 错误代码测试（无效代码）
- 边界条件测试
- 性能测试（大数据量）

## 维护

当API或数据格式发生变化时：
1. 更新对应的数据结构
2. 更新验证逻辑
3. 更新测试文档
4. 重新运行所有测试确保通过
