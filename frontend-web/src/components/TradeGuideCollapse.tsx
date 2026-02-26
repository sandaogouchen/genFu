import { useMemo, useState } from "react";
import { ChevronDown, ChevronUp } from "lucide-react";

import { cn } from "@/lib/utils";

type TradeRule = {
  rule_id?: string;
  indicator?: string;
  operator?: string;
  trigger_value?: number;
  timeframe?: string;
  weight?: number;
  note?: string;
};

type TradeGuidePayload = {
  buy_rules?: TradeRule[];
  sell_rules?: TradeRule[];
  risk_controls?: {
    stop_loss_price?: number;
    take_profit_price?: number;
    max_position_ratio?: number;
  };
};

type Props = {
  guideText: string;
  guideJSON: string;
  className?: string;
};

export default function TradeGuideCollapse({ guideText, guideJSON, className }: Props) {
  const [expanded, setExpanded] = useState(false);
  const parsed = useMemo(() => parseGuideJSON(guideJSON), [guideJSON]);

  const buyCount = parsed?.buy_rules?.length ?? 0;
  const sellCount = parsed?.sell_rules?.length ?? 0;
  const stopLoss = parsed?.risk_controls?.stop_loss_price;
  const takeProfit = parsed?.risk_controls?.take_profit_price;

  const summary = `买入${buyCount}条 · 卖出${sellCount}条 · 止损${formatPrice(stopLoss)} · 止盈${formatPrice(takeProfit)}`;
  const displayJSON = formatStrictJson(guideJSON);

  return (
    <div className={cn("rounded-lg border border-border bg-card/70", className)}>
      <button
        type="button"
        aria-expanded={expanded}
        onClick={() => setExpanded((v) => !v)}
        className="w-full px-3 py-2 text-left hover:bg-muted/40 transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring rounded-lg"
      >
        <div className="flex items-center justify-between gap-3">
          <span className="truncate text-xs text-muted-foreground">{summary}</span>
          {expanded ? (
            <ChevronUp className="h-4 w-4 text-muted-foreground flex-shrink-0" />
          ) : (
            <ChevronDown className="h-4 w-4 text-muted-foreground flex-shrink-0" />
          )}
        </div>
      </button>

      {expanded ? (
        <div className="px-3 pb-3 space-y-2">
          <div className="text-sm text-muted-foreground leading-relaxed">{guideText || "暂无买卖指南说明"}</div>
          {!parsed ? (
            <div className="text-xs text-warning">买卖指南 JSON 解析失败，已展示原始内容。</div>
          ) : null}
          <pre className="text-xs text-foreground bg-muted/40 border border-border rounded-lg p-3 max-h-56 overflow-auto whitespace-pre-wrap break-all">
            {displayJSON}
          </pre>
        </div>
      ) : null}
    </div>
  );
}

function parseGuideJSON(raw: string): TradeGuidePayload | null {
  if (!raw) return null;
  try {
    const parsed = JSON.parse(raw) as TradeGuidePayload;
    return parsed && typeof parsed === "object" ? parsed : null;
  } catch {
    return null;
  }
}

function formatPrice(v?: number): string {
  if (!Number.isFinite(v)) return "¥--";
  return `¥${Number(v).toFixed(2)}`;
}

function formatStrictJson(raw: string): string {
  try {
    return JSON.stringify(JSON.parse(raw), null, 2);
  } catch {
    return raw;
  }
}
