import { useState } from "react";
import {
  TrendingUp,
  TrendingDown,
  AlertTriangle,
  Loader2,
  RefreshCw,
  ChevronDown,
  ChevronUp,
} from "lucide-react";

import Button from "@/components/ui/Button";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/Card";
import { pickStocks } from "@/utils/genfuApi";
import type { StockPick, StockPickResponse } from "@/utils/genfuApi";
import { cn } from "@/lib/utils";

const sentimentMap: Record<string, { label: string; color: string }> = {
  very_bullish: { label: "极度乐观", color: "text-emerald-600 dark:text-emerald-400" },
  bullish: { label: "乐观", color: "text-emerald-500 dark:text-emerald-400" },
  neutral: { label: "中性", color: "text-muted-foreground" },
  bearish: { label: "悲观", color: "text-destructive" },
  very_bearish: { label: "极度悲观", color: "text-destructive" },
};

const riskLevelMap: Record<string, { label: string; className: string }> = {
  low: { label: "低风险", className: "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20" },
  medium: { label: "中风险", className: "bg-warning/10 text-warning border-warning/20" },
  high: { label: "高风险", className: "bg-destructive/10 text-destructive border-destructive/20" },
};

const recommendationMap: Record<string, { label: string; className: string }> = {
  buy: { label: "买入", className: "bg-accent text-accent-foreground" },
  watch: { label: "观察", className: "bg-muted text-muted-foreground" },
};

export default function StockPicker() {
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [result, setResult] = useState<StockPickResponse | null>(null);
  const [expandedStocks, setExpandedStocks] = useState<Set<string>>(new Set());

  const [accountId, setAccountId] = useState<number>(1);
  const [topN, setTopN] = useState<number>(5);

  const handlePick = async () => {
    setLoading(true);
    setError(null);

    try {
      const response = await pickStocks({
        account_id: accountId,
        top_n: topN,
      });
      setResult(response);
    } catch (err) {
      setError(err instanceof Error ? err.message : "选股失败");
    } finally {
      setLoading(false);
    }
  };

  const toggleStock = (symbol: string) => {
    setExpandedStocks((prev) => {
      const next = new Set(prev);
      if (next.has(symbol)) {
        next.delete(symbol);
      } else {
        next.add(symbol);
      }
      return next;
    });
  };

  return (
    <div className="space-y-4">
      {/* Title */}
      <div>
        <h1 className="text-2xl font-semibold text-foreground">智能选股</h1>
        <p className="mt-1 text-sm text-muted-foreground">
          基于大盘数据、重大新闻和技术面分析的智能选股系统
        </p>
      </div>

      {/* Parameters */}
      <Card>
        <CardHeader>
          <CardTitle>选股参数</CardTitle>
        </CardHeader>
        <CardBody>
          <div className="flex flex-wrap gap-4 items-end">
            <div>
              <label className="block text-sm font-medium text-foreground mb-1">
                账户ID
              </label>
              <input
                type="number"
                value={accountId}
                onChange={(e) => setAccountId(Number(e.target.value))}
                className="w-24 px-3 py-2 border border-input bg-background rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring text-sm"
              />
            </div>
            <div>
              <label className="block text-sm font-medium text-foreground mb-1">
                选股数量
              </label>
              <select
                value={topN}
                onChange={(e) => setTopN(Number(e.target.value))}
                className="w-24 px-3 py-2 border border-input bg-background rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring text-sm"
              >
                <option value={3}>3只</option>
                <option value={5}>5只</option>
                <option value={7}>7只</option>
              </select>
            </div>
            <Button onClick={handlePick} disabled={loading}>
              {loading ? (
                <>
                  <Loader2 className="h-4 w-4 animate-spin mr-2" />
                  分析中...
                </>
              ) : (
                <>
                  <RefreshCw className="h-4 w-4 mr-2" />
                  开始选股
                </>
              )}
            </Button>
          </div>
        </CardBody>
      </Card>

      {/* Error */}
      {error && (
        <div className="bg-destructive/10 border border-destructive/20 rounded-lg p-4 flex items-start gap-3">
          <AlertTriangle className="h-5 w-5 text-destructive mt-0.5 flex-shrink-0" />
          <div>
            <h3 className="font-medium text-destructive">选股失败</h3>
            <p className="text-destructive/80 text-sm mt-1">{error}</p>
          </div>
        </div>
      )}

      {/* Results */}
      {result && (
        <div className="space-y-4">
          {/* Market data */}
          <Card>
            <CardHeader>
              <CardTitle>大盘概况</CardTitle>
            </CardHeader>
            <CardBody>
              <div className="grid grid-cols-2 md:grid-cols-4 gap-3 mb-4">
                {result.market_data.index_quotes.map((index) => (
                  <div
                    key={index.code}
                    className="bg-muted/50 rounded-lg p-3 border border-border"
                  >
                    <div className="text-xs text-muted-foreground">{index.name}</div>
                    <div className="text-lg font-bold text-foreground mt-1">
                      {index.price.toFixed(2)}
                    </div>
                    <div
                      className={cn(
                        "text-sm font-medium mt-1 flex items-center gap-1",
                        index.change_rate >= 0 ? "text-emerald-500" : "text-destructive"
                      )}
                    >
                      {index.change_rate >= 0 ? (
                        <TrendingUp className="h-4 w-4" />
                      ) : (
                        <TrendingDown className="h-4 w-4" />
                      )}
                      {index.change >= 0 ? "+" : ""}
                      {index.change.toFixed(2)} ({index.change_rate.toFixed(2)}%)
                    </div>
                  </div>
                ))}
              </div>

              <div className="grid grid-cols-2 md:grid-cols-4 gap-3">
                <div className="bg-muted/50 rounded-lg p-2">
                  <div className="text-xs text-muted-foreground">市场情绪</div>
                  <div
                    className={cn(
                      "text-sm font-semibold mt-1",
                      sentimentMap[result.market_data.market_sentiment]?.color
                    )}
                  >
                    {sentimentMap[result.market_data.market_sentiment]?.label ||
                      result.market_data.market_sentiment}
                  </div>
                </div>
                <div className="bg-muted/50 rounded-lg p-2">
                  <div className="text-xs text-muted-foreground">涨跌比</div>
                  <div className="text-sm font-semibold mt-1">
                    <span className="text-emerald-500">{result.market_data.up_count}</span>
                    {" / "}
                    <span className="text-destructive">{result.market_data.down_count}</span>
                  </div>
                </div>
                <div className="bg-muted/50 rounded-lg p-2">
                  <div className="text-xs text-muted-foreground">涨停</div>
                  <div className="text-sm font-semibold text-emerald-500 mt-1">
                    {result.market_data.limit_up}
                  </div>
                </div>
                <div className="bg-muted/50 rounded-lg p-2">
                  <div className="text-xs text-muted-foreground">跌停</div>
                  <div className="text-sm font-semibold text-destructive mt-1">
                    {result.market_data.limit_down}
                  </div>
                </div>
              </div>
            </CardBody>
          </Card>

          {/* News summary */}
          {result.news_summary && (
            <Card>
              <CardHeader>
                <CardTitle>市场观点</CardTitle>
              </CardHeader>
              <CardBody>
                <p className="text-muted-foreground text-sm leading-relaxed">
                  {result.news_summary}
                </p>
              </CardBody>
            </Card>
          )}

          {/* Strategy guide */}
          {result.strategy_guide ? (
            <Card>
              <CardHeader>
                <CardTitle>策略行动指南</CardTitle>
              </CardHeader>
              <CardBody>
                <div className="space-y-3">
                  <div className="flex flex-wrap items-center gap-2 text-xs">
                    {result.strategy_guide.strategy_name ? (
                      <span className="px-2 py-1 rounded border border-border bg-muted/50 text-foreground">
                        {result.strategy_guide.strategy_name}
                      </span>
                    ) : null}
                    {result.strategy_guide.strategy_type ? (
                      <span className="px-2 py-1 rounded border border-border bg-muted/50 text-muted-foreground">
                        {result.strategy_guide.strategy_type}
                      </span>
                    ) : null}
                  </div>
                  <p className="text-sm text-muted-foreground leading-relaxed">
                    {result.strategy_guide.guide_text}
                  </p>
                  <div>
                    <div className="text-xs text-muted-foreground mb-1">
                      买卖信息（严格 JSON 字符串）
                    </div>
                    <pre className="text-xs text-foreground bg-muted/40 border border-border rounded-lg p-3 overflow-auto whitespace-pre-wrap break-all">
                      {formatStrictJson(result.strategy_guide.trade_signals_json)}
                    </pre>
                  </div>
                </div>
              </CardBody>
            </Card>
          ) : null}

          {/* Warnings */}
          {result.warnings && result.warnings.length > 0 && (
            <div className="bg-warning/10 border border-warning/20 rounded-lg p-4">
              <div className="flex items-start gap-3">
                <AlertTriangle className="h-5 w-5 text-warning mt-0.5 flex-shrink-0" />
                <div>
                  <h3 className="font-medium text-warning">风险提示</h3>
                  <ul className="text-warning/80 text-sm mt-2 space-y-1">
                    {result.warnings.map((warning, i) => (
                      <li key={i}>• {warning}</li>
                    ))}
                  </ul>
                </div>
              </div>
            </div>
          )}

          {/* Stock picks */}
          <Card>
            <CardHeader>
              <CardTitle>精选股票 ({result.stocks.length}只)</CardTitle>
            </CardHeader>
            <CardBody>
              <div className="space-y-3">
                {result.stocks.map((stock) => (
                  <StockCard
                    key={stock.symbol}
                    stock={stock}
                    expanded={expandedStocks.has(stock.symbol)}
                    onToggle={() => toggleStock(stock.symbol)}
                  />
                ))}
              </div>
            </CardBody>
          </Card>

          {/* Generated time */}
          <div className="text-center text-xs text-muted-foreground">
            生成时间: {new Date(result.generated_at).toLocaleString("zh-CN")}
          </div>
        </div>
      )}
    </div>
  );
}

function StockCard({
  stock,
  expanded,
  onToggle,
}: {
  stock: StockPick;
  expanded: boolean;
  onToggle: () => void;
}) {
  return (
    <div className="border border-border rounded-lg overflow-hidden">
      <button
        type="button"
        onClick={onToggle}
        className="w-full px-4 py-3 flex items-center justify-between hover:bg-muted/50 transition-colors"
      >
        <div className="flex items-center gap-3 flex-wrap">
          <span
            className={cn(
              "px-2 py-1 rounded text-xs font-medium",
              recommendationMap[stock.recommendation]?.className
            )}
          >
            {recommendationMap[stock.recommendation]?.label}
          </span>
          <div className="text-left">
            <div className="flex items-center gap-2">
              <span className="font-semibold text-foreground">{stock.name}</span>
              <span className="text-xs text-muted-foreground">{stock.symbol}</span>
            </div>
            <div className="text-xs text-muted-foreground">{stock.industry}</div>
          </div>
          <div className="text-right">
            <div className="font-semibold text-foreground">
              ¥{stock.current_price.toFixed(2)}
            </div>
          </div>
          <div className="w-16">
            <div className="text-xs font-medium text-foreground">
              {(stock.confidence * 100).toFixed(0)}%
            </div>
            <div className="h-1.5 bg-muted rounded-full overflow-hidden mt-1">
              <div
                className="h-full bg-accent rounded-full"
                style={{ width: `${stock.confidence * 100}%` }}
              />
            </div>
          </div>
          <span
            className={cn(
              "px-2 py-1 rounded text-xs font-medium border",
              riskLevelMap[stock.risk_level]?.className
            )}
          >
            {riskLevelMap[stock.risk_level]?.label}
          </span>
        </div>
        {expanded ? (
          <ChevronUp className="h-5 w-5 text-muted-foreground" />
        ) : (
          <ChevronDown className="h-5 w-5 text-muted-foreground" />
        )}
      </button>

      {expanded && (
        <div className="border-t border-border p-4 bg-muted/30 space-y-4">
          {/* Technical analysis */}
          <div>
            <h4 className="text-sm font-semibold text-foreground mb-2">技术面分析</h4>
            <div className="grid md:grid-cols-2 gap-4">
              <div>
                <div className="text-xs text-muted-foreground mb-1">趋势分析</div>
                <div className="text-sm text-muted-foreground">{stock.technical_reasons.trend}</div>
              </div>
              <div>
                <div className="text-xs text-muted-foreground mb-1">成交量信号</div>
                <div className="text-sm text-muted-foreground">{stock.technical_reasons.volume_signal}</div>
              </div>
            </div>
            <div className="mt-3">
              <div className="text-xs text-muted-foreground mb-1">技术指标</div>
              <div className="flex flex-wrap gap-2">
                {stock.technical_reasons.technical_indicators.map((indicator, i) => (
                  <span
                    key={i}
                    className="px-2 py-1 bg-accent/10 text-accent rounded text-xs border border-accent/20"
                  >
                    {indicator}
                  </span>
                ))}
              </div>
            </div>
            <div className="grid md:grid-cols-2 gap-4 mt-3">
              <div>
                <div className="text-xs text-muted-foreground mb-1">关键价位</div>
                <div className="space-y-1">
                  {stock.technical_reasons.key_levels.map((level, i) => (
                    <div key={i} className="text-sm text-muted-foreground">• {level}</div>
                  ))}
                </div>
              </div>
              <div>
                <div className="text-xs text-muted-foreground mb-1">风险点</div>
                <div className="space-y-1">
                  {stock.technical_reasons.risk_points.map((risk, i) => (
                    <div key={i} className="text-sm text-destructive flex items-start gap-1">
                      <AlertTriangle className="h-3 w-3 mt-0.5 flex-shrink-0" />
                      {risk}
                    </div>
                  ))}
                </div>
              </div>
            </div>
          </div>

          {/* Allocation */}
          <div>
            <h4 className="text-sm font-semibold text-foreground mb-2">资产配置建议</h4>
            <div className="grid grid-cols-2 md:grid-cols-5 gap-3">
              <div className="bg-card rounded-lg p-2 border border-border">
                <div className="text-xs text-muted-foreground">建议权重</div>
                <div className="text-sm font-semibold text-foreground">
                  {(stock.allocation.suggested_weight * 100).toFixed(0)}%
                </div>
              </div>
              <div className="bg-card rounded-lg p-2 border border-border">
                <div className="text-xs text-muted-foreground">行业分散度</div>
                <div className="text-sm font-semibold text-foreground">
                  {(stock.allocation.industry_diversity * 100).toFixed(0)}%
                </div>
              </div>
              <div className="bg-card rounded-lg p-2 border border-border">
                <div className="text-xs text-muted-foreground">风险敞口</div>
                <div className="text-sm font-semibold text-foreground">
                  {(stock.allocation.risk_exposure * 100).toFixed(0)}%
                </div>
              </div>
              <div className="bg-card rounded-lg p-2 border border-border">
                <div className="text-xs text-muted-foreground">流动性评分</div>
                <div className="text-sm font-semibold text-foreground">
                  {(stock.allocation.liquidity_score * 100).toFixed(0)}%
                </div>
              </div>
              <div className="bg-card rounded-lg p-2 border border-border">
                <div className="text-xs text-muted-foreground">持仓相关性</div>
                <div className="text-sm font-semibold text-foreground">
                  {(stock.allocation.correlation_with_holding * 100).toFixed(0)}%
                </div>
              </div>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function formatStrictJson(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}
