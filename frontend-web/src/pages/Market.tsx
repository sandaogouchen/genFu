import { useMemo, useRef, useState } from "react";

import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import Skeleton from "@/components/ui/Skeleton";
import CodeBlock from "@/components/conversation/CodeBlock";
import CollapsibleSection from "@/components/conversation/CollapsibleSection";
import MarketChart from "@/components/market/MarketChart";
import { toast } from "@/hooks/useToast";
import { MarketDataRequest, postJson } from "@/utils/genfuApi";
import type { UTCTimestamp } from "lightweight-charts";
import { cn } from "@/lib/utils";

type KlinePoint = {
  time?: string;
  open?: number;
  high?: number;
  low?: number;
  close?: number;
  volume?: number;
};

type IntradayPoint = {
  time?: string;
  price?: number;
  avg_price?: number;
};

type HoldingMarketData = {
  instrument?: { symbol?: string; name?: string };
  kline?: KlinePoint[];
  intraday?: IntradayPoint[];
  error?: string;
};

type CandlestickSeries = {
  title: string;
  type: "candlestick";
  data: Array<{ time: UTCTimestamp; open: number; high: number; low: number; close: number }>;
};

type LineSeries = {
  title: string;
  type: "line";
  data: Array<{ time: UTCTimestamp; value: number }>;
};

type ChartSeriesUnion = CandlestickSeries | LineSeries;

function toTimestamp(input?: string): UTCTimestamp | null {
  if (!input) return null;
  const value = input.trim();
  if (!value) return null;
  const dateValue = value.includes(" ") ? value.replace(" ", "T") : value;
  const ms = Date.parse(dateValue);
  if (Number.isNaN(ms)) return null;
  return Math.floor(ms / 1000) as UTCTimestamp;
}

function isKlineArray(value: unknown): value is KlinePoint[] {
  return Array.isArray(value) && value.length > 0 && typeof (value[0] as KlinePoint).open === "number";
}

function isIntradayArray(value: unknown): value is IntradayPoint[] {
  return Array.isArray(value) && value.length > 0 && typeof (value[0] as IntradayPoint).price === "number";
}

function isHoldingsArray(value: unknown): value is HoldingMarketData[] {
  return Array.isArray(value) && value.length > 0 && typeof (value[0] as HoldingMarketData).instrument === "object";
}

function buildKlineSeries(title: string, points: KlinePoint[]): CandlestickSeries | null {
  const data = points
    .map((p) => {
      const time = toTimestamp(p.time);
      if (!time || p.open == null || p.high == null || p.low == null || p.close == null) return null;
      return { time, open: p.open, high: p.high, low: p.low, close: p.close };
    })
    .filter(Boolean) as Array<{ time: UTCTimestamp; open: number; high: number; low: number; close: number }>;
  if (!data.length) return null;
  return { title, type: "candlestick", data };
}

function buildLineSeries(title: string, points: IntradayPoint[], fallbackLabel?: string): LineSeries | null {
  const data = points
    .map((p) => {
      const time = toTimestamp(p.time);
      const value = p.price ?? p.avg_price;
      if (!time || value == null) return null;
      return { time, value };
    })
    .filter(Boolean) as Array<{ time: UTCTimestamp; value: number }>;
  if (!data.length) return null;
  return { title: fallbackLabel ? `${title} · ${fallbackLabel}` : title, type: "line", data };
}

export default function Market() {
  const abortRef = useRef<AbortController | null>(null);
  const [action, setAction] = useState<string>("get_stock_kline");
  const [code, setCode] = useState<string>("600519");
  const [period, setPeriod] = useState<string>("day");
  const [klt, setKlt] = useState<string>("101");
  const [adjust, setAdjust] = useState<string>("qfq");
  const [start, setStart] = useState<string>("");
  const [end, setEnd] = useState<string>("");
  const [days, setDays] = useState<string>("1");
  const [limit, setLimit] = useState<string>("120");
  const [assetType, setAssetType] = useState<string>("");
  const [loading, setLoading] = useState(false);
  const [resp, setResp] = useState<unknown>(null);
  const [error, setError] = useState<string>("");

  const isKline = useMemo(() => action.includes("kline"), [action]);
  const isIntraday = useMemo(() => action.includes("intraday"), [action]);
  const isHoldings = useMemo(() => action.includes("holdings"), [action]);

  const requestBody = useMemo<MarketDataRequest>(() => {
    const body: MarketDataRequest = { action };
    if (code.trim()) body.code = code.trim();
    if (isKline) {
      if (period.trim()) body.period = period.trim();
      if (klt.trim()) body.klt = Number(klt);
      if (adjust.trim()) body.adjust = adjust.trim();
      if (start.trim()) body.start = start.trim();
      if (end.trim()) body.end = end.trim();
    }
    if (isIntraday && days.trim()) body.days = Number(days);
    if (isHoldings) {
      if (assetType.trim()) body.asset_type = assetType.trim();
      if (limit.trim()) body.limit = Number(limit);
    }
    return body;
  }, [action, assetType, code, days, end, isHoldings, isIntraday, isKline, klt, limit, period, start, adjust]);

  const normalized = useMemo(() => {
    if (!resp || typeof resp !== "object") return resp;
    if ("output" in resp) {
      const output = (resp as { output?: unknown }).output;
      return output ?? resp;
    }
    return resp;
  }, [resp]);

  const toolError = useMemo(() => {
    if (!resp || typeof resp !== "object") return "";
    if ("error" in resp) {
      const value = (resp as { error?: unknown }).error;
      return typeof value === "string" ? value : "";
    }
    return "";
  }, [resp]);

  const charts = useMemo<ChartSeriesUnion[]>(() => {
    if (!normalized) return [];
    if (isHoldingsArray(normalized)) {
      const series: ChartSeriesUnion[] = [];
      normalized.forEach((item) => {
        const label = [item.instrument?.name, item.instrument?.symbol].filter(Boolean).join(" ");
        if (item.kline && item.kline.length) {
          const s = buildKlineSeries(label || "持仓K线", item.kline);
          if (s) series.push(s);
        }
        if (item.intraday && item.intraday.length) {
          const s = buildLineSeries(label || "持仓分时", item.intraday);
          if (s) series.push(s);
        }
      });
      return series;
    }
    if (isKlineArray(normalized)) {
      const title = action.includes("fund") ? "基金净值走势" : "K线走势";
      const s = buildKlineSeries(title, normalized);
      return s ? [s] : [];
    }
    if (isIntradayArray(normalized)) {
      const title = action.includes("fund") ? "基金实时估值" : "分时走势";
      const s = buildLineSeries(title, normalized);
      return s ? [s] : [];
    }
    return [];
  }, [action, normalized]);

  return (
    <div className="space-y-5">
      {/* Page Header */}
      <div>
        <h1 className="text-2xl font-bold tracking-tight text-foreground">行情数据</h1>
        <p className="text-sm text-muted-foreground mt-1">股票/基金行情查询与可视化</p>
      </div>

      {/* Controls */}
      <div className="rounded-2xl border border-border/50 bg-card p-4">
        <div className="grid gap-3 md:grid-cols-4 lg:grid-cols-6">
          <Select value={action} onChange={(e) => setAction(e.target.value)}>
            <option value="get_stock_kline">股票K线</option>
            <option value="get_stock_intraday">股票分时</option>
            <option value="get_fund_kline">基金净值</option>
            <option value="get_fund_intraday">基金估值</option>
            <option value="get_fund_holdings_kline">持仓K线</option>
            <option value="get_fund_holdings_intraday">持仓分时</option>
          </Select>
          <Input placeholder="代码" value={code} onChange={(e) => setCode(e.target.value)} />
          {isKline && (
            <>
              <Select value={period} onChange={(e) => setPeriod(e.target.value)}>
                <option value="day">日K</option>
                <option value="week">周K</option>
                <option value="month">月K</option>
                <option value="1m">1分</option>
                <option value="5m">5分</option>
                <option value="15m">15分</option>
                <option value="30m">30分</option>
                <option value="60m">60分</option>
              </Select>
              <Select value={adjust} onChange={(e) => setAdjust(e.target.value)}>
                <option value="qfq">前复权</option>
                <option value="hfq">后复权</option>
                <option value="">不复权</option>
              </Select>
              <Input placeholder="开始日期" value={start} onChange={(e) => setStart(e.target.value)} />
              <Input placeholder="结束日期" value={end} onChange={(e) => setEnd(e.target.value)} />
            </>
          )}
          {isIntraday && <Input placeholder="天数" value={days} onChange={(e) => setDays(e.target.value)} />}
          {isHoldings && (
            <>
              <Input placeholder="asset_type" value={assetType} onChange={(e) => setAssetType(e.target.value)} />
              <Input placeholder="limit" value={limit} onChange={(e) => setLimit(e.target.value)} />
            </>
          )}
        </div>
        <div className="mt-4 flex justify-end gap-2">
          <Button
            disabled={loading}
            onClick={async () => {
              abortRef.current?.abort();
              const ac = new AbortController();
              abortRef.current = ac;
              setError("");
              setResp(null);
              setLoading(true);
              try {
                const data = await postJson<unknown>("/api/marketdata", requestBody, { signal: ac.signal });
                setResp(data);
                toast({ title: "请求完成", description: "行情数据返回成功" });
              } catch (e) {
                const msg = e instanceof Error ? e.message : "unknown_error";
                setError(msg);
                toast({ title: "请求失败", description: msg, durationMs: 5200 });
              } finally {
                setLoading(false);
              }
            }}
          >
            {loading ? "查询中…" : "查询"}
          </Button>
          <Button
            variant="secondary"
            disabled={!loading}
            onClick={() => {
              abortRef.current?.abort();
              setLoading(false);
              toast({ title: "已取消", description: "已中止本次请求" });
            }}
          >
            取消
          </Button>
        </div>
      </div>

      {/* Results */}
      <div className="rounded-2xl border border-border/50 bg-card p-5">
        {loading && !resp ? (
          <div className="space-y-3">
            <Skeleton className="h-6 w-40" />
            <Skeleton className="h-80 w-full" />
          </div>
        ) : resp ? (
          <div className="space-y-4">
            {charts.length > 0 ? (
              <div className="grid gap-4">
                {charts.map((chart, idx) => (
                  <MarketChart key={`${chart.title}-${idx}`} title={chart.title} type={chart.type} data={chart.data} />
                ))}
              </div>
            ) : (
              <div className="text-sm text-muted-foreground">暂无可绘制的数据</div>
            )}
            <CollapsibleSection title="原始数据">
              <CodeBlock language="json" code={JSON.stringify(resp, null, 2)} />
            </CollapsibleSection>
          </div>
        ) : (
          <div className="text-sm text-muted-foreground text-center py-8">选择参数后点击查询</div>
        )}
        {error ? <div className="mt-3 text-sm text-destructive">{error}</div> : null}
        {!error && toolError ? <div className="mt-3 text-sm text-destructive">{toolError}</div> : null}
      </div>
    </div>
  );
}
