import { useCallback, useEffect, useMemo, useState } from "react";
import {
  AlertTriangle,
  ChevronDown,
  ChevronUp,
  Loader2,
  RefreshCw,
  TrendingDown,
  TrendingUp,
} from "lucide-react";

import AssistantCard from "@/components/conversation/AssistantCard";
import TipCard from "@/components/conversation/TipCard";
import UserBubble from "@/components/conversation/UserBubble";
import Button from "@/components/ui/Button";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/Card";
import { toast } from "@/hooks/useToast";
import { cn } from "@/lib/utils";
import { usePageSessionStore } from "@/stores/pageSessionStore";
import type { ConversationRun, StockPick, StockPickResponse } from "@/utils/genfuApi";
import {
  createConversationSession,
  listConversationRuns,
  pickStocks,
} from "@/utils/genfuApi";

type StockPickerRun = {
  id: string;
  prompt: string;
  result?: StockPickResponse;
  error?: string;
};

function getRecord(v: unknown): Record<string, unknown> {
  if (!v || typeof v !== "object") return {};
  return v as Record<string, unknown>;
}

function toStockPickResponse(raw: unknown): StockPickResponse | undefined {
  if (!raw) return undefined;
  if (typeof raw === "string") {
    try {
      const parsed = JSON.parse(raw) as unknown;
      const obj = getRecord(parsed);
      return Object.keys(obj).length > 0 ? (obj as StockPickResponse) : undefined;
    } catch {
      return undefined;
    }
  }
  const obj = getRecord(raw);
  return Object.keys(obj).length > 0 ? (obj as StockPickResponse) : undefined;
}

function toStockPickerRun(run: ConversationRun): StockPickerRun {
  return {
    id: String(run.id),
    prompt: run.prompt || "智能选股请求",
    result: toStockPickResponse(run.result),
    error: run.error,
  };
}

const sentimentMap: Record<string, { label: string; className: string }> = {
  very_bullish: { label: "极度乐观", className: "text-emerald-600 dark:text-emerald-400" },
  bullish: { label: "乐观", className: "text-emerald-500 dark:text-emerald-400" },
  neutral: { label: "中性", className: "text-muted-foreground" },
  bearish: { label: "悲观", className: "text-destructive" },
  very_bearish: { label: "极度悲观", className: "text-destructive" },
};

const recommendationMap: Record<string, { label: string; className: string }> = {
  buy: { label: "买入", className: "bg-accent text-accent-foreground" },
  watch: { label: "观察", className: "bg-muted text-muted-foreground" },
};

const riskLevelMap: Record<string, { label: string; className: string }> = {
  low: { label: "低风险", className: "bg-emerald-500/10 text-emerald-600 border-emerald-500/20" },
  medium: { label: "中风险", className: "bg-warning/10 text-warning border-warning/20" },
  high: { label: "高风险", className: "bg-destructive/10 text-destructive border-destructive/20" },
};

function formatPct(v: number | undefined) {
  if (typeof v !== "number" || Number.isNaN(v)) return "--";
  return `${(v * 100).toFixed(0)}%`;
}

function formatPrice(v: number | undefined) {
  if (typeof v !== "number" || Number.isNaN(v)) return "--";
  return `¥${v.toFixed(2)}`;
}

function getStockExpandKey(runId: string, stock: StockPick, idx: number) {
  return `${runId}::${stock.symbol || idx}`;
}

export default function StockPicker() {
  const activeByScene = usePageSessionStore((s) => s.activeByScene);
  const setActive = usePageSessionStore((s) => s.setActive);
  const sessionId = activeByScene.stockpicker ?? "";

  const [loading, setLoading] = useState(false);
  const [runs, setRuns] = useState<StockPickerRun[]>([]);
  const [expandedStocks, setExpandedStocks] = useState<Set<string>>(new Set());
  const [accountId, setAccountId] = useState<number>(1);
  const [topN, setTopN] = useState<number>(5);

  const readonlyLogMode = useMemo(
    () => runs.some((run) => !run.id.startsWith("temp_")),
    [runs]
  );

  const loadRuns = useCallback(async (sid: string) => {
    if (!sid) {
      setRuns([]);
      return;
    }
    try {
      const data = await listConversationRuns(sid, 100);
      setRuns((data.items ?? []).map(toStockPickerRun));
    } catch (err) {
      const msg = err instanceof Error ? err.message : "加载失败";
      toast({ title: "加载会话失败", description: msg, durationMs: 4200 });
      setRuns([]);
    }
  }, []);

  useEffect(() => {
    if (sessionId) return;
    let canceled = false;
    (async () => {
      try {
        const created = await createConversationSession({ scene: "stockpicker", title: "选股日志 1" });
        if (!canceled) {
          setActive("stockpicker", created.id);
          window.dispatchEvent(new Event("genfu:conversation-updated"));
        }
      } catch (err) {
        if (!canceled) {
          const msg = err instanceof Error ? err.message : "创建会话失败";
          toast({ title: "创建会话失败", description: msg, durationMs: 4200 });
        }
      }
    })();
    return () => {
      canceled = true;
    };
  }, [sessionId, setActive]);

  useEffect(() => {
    if (!sessionId) return;
    void loadRuns(sessionId);
  }, [sessionId, loadRuns]);

  useEffect(() => {
    const handler = () => {
      if (sessionId) {
        void loadRuns(sessionId);
      }
    };
    window.addEventListener("genfu:conversation-updated", handler);
    return () => {
      window.removeEventListener("genfu:conversation-updated", handler);
    };
  }, [sessionId, loadRuns]);

  const handlePick = async () => {
    if (!sessionId) {
      toast({ title: "请稍候", description: "会话初始化中" });
      return;
    }
    setLoading(true);
    const prompt = `智能选股 account=${accountId} topN=${topN}`;
    const tempId = `temp_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`;
    setRuns((xs) => [...xs, { id: tempId, prompt }]);
    try {
      const response = await pickStocks({
        account_id: accountId,
        top_n: topN,
        session_id: sessionId,
        session_title: prompt.slice(0, 20),
        prompt,
      });
      setRuns((xs) => xs.map((x) => (x.id === tempId ? { ...x, result: response } : x)));
      await loadRuns(sessionId);
      window.dispatchEvent(new Event("genfu:conversation-updated"));
      toast({ title: "选股完成", description: "已写入会话日志" });
    } catch (err) {
      const msg = err instanceof Error ? err.message : "选股失败";
      setRuns((xs) => xs.map((x) => (x.id === tempId ? { ...x, error: msg } : x)));
      await loadRuns(sessionId);
      window.dispatchEvent(new Event("genfu:conversation-updated"));
      toast({ title: "选股失败", description: msg, durationMs: 5200 });
    } finally {
      setLoading(false);
    }
  };

  const toggleStock = (key: string) => {
    setExpandedStocks((prev) => {
      const next = new Set(prev);
      if (next.has(key)) {
        next.delete(key);
      } else {
        next.add(key);
      }
      return next;
    });
  };

  return (
    <div className="space-y-4">
      <TipCard title="提示：选股结果会保存为当前页面会话日志" dismissKey="stockpicker">
        每次执行都会保存完整结构化结果，可在左侧切换历史会话回放。
      </TipCard>

      {readonlyLogMode ? (
        <Card>
          <CardBody>
            <div className="text-sm text-muted-foreground">
              当前为选股日志回放模式，已隐藏“账户ID / 选股数量 / 开始选股”。如需发起新任务，请在左侧点击“新建”。
            </div>
          </CardBody>
        </Card>
      ) : (
        <Card>
          <CardHeader>
            <CardTitle>智能选股参数</CardTitle>
          </CardHeader>
          <CardBody>
            <div className="flex flex-wrap gap-4 items-end">
              <div>
                <label className="block text-sm font-medium text-foreground mb-1">账户ID</label>
                <input
                  type="number"
                  value={accountId}
                  onChange={(e) => setAccountId(Number(e.target.value))}
                  className="w-24 px-3 py-2 border border-input bg-background rounded-lg focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring text-sm"
                />
              </div>
              <div>
                <label className="block text-sm font-medium text-foreground mb-1">选股数量</label>
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
              <Button onClick={handlePick} disabled={loading || !sessionId}>
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
      )}

      <div className="space-y-5">
        {runs.map((run) => (
          <div key={run.id} className="space-y-4">
            <UserBubble content={run.prompt} />
            <AssistantCard badge="GenFu" title="智能选股结果">
              {run.error ? <div className="text-sm text-destructive">{run.error}</div> : null}
              {run.result ? (
                <div className="space-y-4">
                  <div className="text-xs text-muted-foreground">
                    生成时间：{run.result.generated_at ? new Date(run.result.generated_at).toLocaleString("zh-CN") : "--"}
                  </div>

                  <div className="rounded-xl border border-border bg-muted/20 p-3">
                    <div className="text-sm font-semibold text-foreground mb-3">大盘概况</div>
                    <div className="grid grid-cols-1 gap-3 sm:grid-cols-2 lg:grid-cols-4">
                      {(run.result.market_data?.index_quotes ?? []).map((idx) => (
                        <div key={idx.code} className="rounded-lg border border-border/60 bg-card p-3">
                          <div className="text-xs text-muted-foreground">{idx.name}</div>
                          <div className="mt-1 text-base font-semibold text-foreground">
                            {typeof idx.price === "number" ? idx.price.toFixed(2) : "--"}
                          </div>
                          <div
                            className={cn(
                              "mt-1 flex items-center gap-1 text-sm font-medium",
                              (idx.change_rate ?? 0) >= 0 ? "text-emerald-500" : "text-destructive"
                            )}
                          >
                            {(idx.change_rate ?? 0) >= 0 ? (
                              <TrendingUp className="h-4 w-4" />
                            ) : (
                              <TrendingDown className="h-4 w-4" />
                            )}
                            {(idx.change ?? 0) >= 0 ? "+" : ""}
                            {typeof idx.change === "number" ? idx.change.toFixed(2) : "--"}
                            {" ("}
                            {typeof idx.change_rate === "number" ? idx.change_rate.toFixed(2) : "--"}
                            %)
                          </div>
                        </div>
                      ))}
                    </div>
                    <div className="mt-3 grid grid-cols-2 gap-2 lg:grid-cols-4">
                      <div className="rounded-lg border border-border/60 bg-card p-2">
                        <div className="text-xs text-muted-foreground">市场情绪</div>
                        <div
                          className={cn(
                            "mt-1 text-sm font-semibold",
                            sentimentMap[run.result.market_data?.market_sentiment]?.className
                          )}
                        >
                          {sentimentMap[run.result.market_data?.market_sentiment]?.label ||
                            run.result.market_data?.market_sentiment ||
                            "--"}
                        </div>
                      </div>
                      <div className="rounded-lg border border-border/60 bg-card p-2">
                        <div className="text-xs text-muted-foreground">涨跌比</div>
                        <div className="mt-1 text-sm font-semibold">
                          <span className="text-emerald-500">{run.result.market_data?.up_count ?? "--"}</span>
                          {" / "}
                          <span className="text-destructive">{run.result.market_data?.down_count ?? "--"}</span>
                        </div>
                      </div>
                      <div className="rounded-lg border border-border/60 bg-card p-2">
                        <div className="text-xs text-muted-foreground">涨停</div>
                        <div className="mt-1 text-sm font-semibold text-emerald-500">
                          {run.result.market_data?.limit_up ?? "--"}
                        </div>
                      </div>
                      <div className="rounded-lg border border-border/60 bg-card p-2">
                        <div className="text-xs text-muted-foreground">跌停</div>
                        <div className="mt-1 text-sm font-semibold text-destructive">
                          {run.result.market_data?.limit_down ?? "--"}
                        </div>
                      </div>
                    </div>
                  </div>

                  {run.result.news_summary ? (
                    <div className="rounded-xl border border-border bg-card p-3">
                      <div className="text-sm font-semibold text-foreground mb-2">市场观点</div>
                      <p className="text-sm text-muted-foreground whitespace-pre-wrap">{run.result.news_summary}</p>
                    </div>
                  ) : null}

                  {run.result.warnings && run.result.warnings.length > 0 ? (
                    <div className="rounded-xl border border-warning/30 bg-warning/10 p-3">
                      <div className="flex items-start gap-2">
                        <AlertTriangle className="mt-0.5 h-4 w-4 text-warning" />
                        <div className="min-w-0">
                          <div className="text-sm font-semibold text-warning">风险提示</div>
                          <ul className="mt-1 space-y-1 text-sm text-warning/90">
                            {run.result.warnings.map((warning, idx) => (
                              <li key={idx}>• {warning}</li>
                            ))}
                          </ul>
                        </div>
                      </div>
                    </div>
                  ) : null}

                  <div className="space-y-3">
                    <div className="text-sm font-semibold text-foreground">
                      精选股票 ({run.result.stocks?.length ?? 0}只)
                    </div>
                    {(run.result.stocks ?? []).map((stock, idx) => {
                      const stockKey = getStockExpandKey(run.id, stock, idx);
                      return (
                        <StockCard
                          key={stockKey}
                          stock={stock}
                          expanded={expandedStocks.has(stockKey)}
                          onToggle={() => toggleStock(stockKey)}
                        />
                      );
                    })}
                  </div>
                </div>
              ) : null}
            </AssistantCard>
          </div>
        ))}
      </div>
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
    <div className="overflow-hidden rounded-xl border border-border bg-card">
      <button
        type="button"
        onClick={onToggle}
        className="flex w-full items-center justify-between gap-3 px-3 py-3 text-left hover:bg-muted/50"
      >
        <div className="flex min-w-0 flex-wrap items-center gap-2">
          <span
            className={cn(
              "rounded px-2 py-0.5 text-xs font-medium",
              recommendationMap[stock.recommendation]?.className
            )}
          >
            {recommendationMap[stock.recommendation]?.label || stock.recommendation}
          </span>
          <div className="min-w-0">
            <div className="truncate text-sm font-semibold text-foreground">
              {stock.name} <span className="text-xs text-muted-foreground">{stock.symbol}</span>
            </div>
            <div className="truncate text-xs text-muted-foreground">{stock.industry}</div>
          </div>
          <div className="text-sm font-semibold text-foreground">{formatPrice(stock.current_price)}</div>
          <div className="text-xs text-muted-foreground">
            置信度 <span className="font-medium text-foreground">{formatPct(stock.confidence)}</span>
          </div>
          <span
            className={cn(
              "rounded border px-2 py-0.5 text-xs font-medium",
              riskLevelMap[stock.risk_level]?.className
            )}
          >
            {riskLevelMap[stock.risk_level]?.label || stock.risk_level}
          </span>
        </div>
        {expanded ? (
          <ChevronUp className="h-4 w-4 shrink-0 text-muted-foreground" />
        ) : (
          <ChevronDown className="h-4 w-4 shrink-0 text-muted-foreground" />
        )}
      </button>

      {expanded ? (
        <div className="space-y-3 border-t border-border bg-muted/20 px-3 py-3">
          <div className="grid gap-3 md:grid-cols-2">
            <div>
              <div className="text-xs font-medium text-muted-foreground">趋势分析</div>
              <div className="mt-1 text-sm text-foreground">{stock.technical_reasons?.trend || "--"}</div>
            </div>
            <div>
              <div className="text-xs font-medium text-muted-foreground">成交量信号</div>
              <div className="mt-1 text-sm text-foreground">{stock.technical_reasons?.volume_signal || "--"}</div>
            </div>
          </div>

          {stock.technical_reasons?.technical_indicators?.length ? (
            <div>
              <div className="text-xs font-medium text-muted-foreground">技术指标</div>
              <div className="mt-1 flex flex-wrap gap-1">
                {stock.technical_reasons.technical_indicators.map((indicator, idx) => (
                  <span key={idx} className="rounded border border-accent/20 bg-accent/10 px-2 py-0.5 text-xs text-accent">
                    {indicator}
                  </span>
                ))}
              </div>
            </div>
          ) : null}

          <div className="grid gap-3 md:grid-cols-2">
            <div>
              <div className="text-xs font-medium text-muted-foreground">关键价位</div>
              <div className="mt-1 space-y-1 text-sm text-foreground">
                {(stock.technical_reasons?.key_levels ?? []).map((level, idx) => (
                  <div key={idx}>• {level}</div>
                ))}
              </div>
            </div>
            <div>
              <div className="text-xs font-medium text-muted-foreground">风险点</div>
              <div className="mt-1 space-y-1">
                {(stock.technical_reasons?.risk_points ?? []).map((risk, idx) => (
                  <div key={idx} className="flex items-start gap-1 text-sm text-destructive">
                    <AlertTriangle className="mt-0.5 h-3 w-3 shrink-0" />
                    {risk}
                  </div>
                ))}
              </div>
            </div>
          </div>

          <div>
            <div className="text-xs font-medium text-muted-foreground">交易建议</div>
            <div className="mt-1 text-sm text-foreground whitespace-pre-wrap">
              {stock.trade_guide_text || "--"}
            </div>
          </div>

          <div className="grid grid-cols-2 gap-2 md:grid-cols-5">
            <div className="rounded-lg border border-border bg-card p-2">
              <div className="text-xs text-muted-foreground">建议权重</div>
              <div className="text-sm font-semibold text-foreground">{formatPct(stock.allocation?.suggested_weight)}</div>
            </div>
            <div className="rounded-lg border border-border bg-card p-2">
              <div className="text-xs text-muted-foreground">行业分散度</div>
              <div className="text-sm font-semibold text-foreground">{formatPct(stock.allocation?.industry_diversity)}</div>
            </div>
            <div className="rounded-lg border border-border bg-card p-2">
              <div className="text-xs text-muted-foreground">风险敞口</div>
              <div className="text-sm font-semibold text-foreground">{formatPct(stock.allocation?.risk_exposure)}</div>
            </div>
            <div className="rounded-lg border border-border bg-card p-2">
              <div className="text-xs text-muted-foreground">流动性评分</div>
              <div className="text-sm font-semibold text-foreground">{formatPct(stock.allocation?.liquidity_score)}</div>
            </div>
            <div className="rounded-lg border border-border bg-card p-2">
              <div className="text-xs text-muted-foreground">持仓相关性</div>
              <div className="text-sm font-semibold text-foreground">{formatPct(stock.allocation?.correlation_with_holding)}</div>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}
