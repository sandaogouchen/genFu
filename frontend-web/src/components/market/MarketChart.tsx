import { useEffect, useMemo, useRef } from "react";
import { CandlestickData, ColorType, LineData, UTCTimestamp, createChart } from "lightweight-charts";

type CandlestickDatum = CandlestickData<UTCTimestamp>;
type LineDatum = LineData<UTCTimestamp>;

export default function MarketChart({
  title,
  type,
  data,
  height,
}: {
  title?: string;
  type: "candlestick" | "line";
  data: CandlestickDatum[] | LineDatum[];
  height?: number;
}) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const chartHeight = height ?? 320;
  const safeData = useMemo(() => data ?? [], [data]);

  useEffect(() => {
    const container = containerRef.current;
    if (!container) return;
    const chart = createChart(container, {
      height: chartHeight,
      layout: {
        background: { type: ColorType.Solid, color: "transparent" },
        textColor: "#18181b",
      },
      grid: {
        vertLines: { color: "#f4f4f5" },
        horzLines: { color: "#f4f4f5" },
      },
      rightPriceScale: { borderColor: "#e4e4e7" },
      timeScale: { borderColor: "#e4e4e7" },
    });

    if (type === "candlestick") {
      const series = chart.addCandlestickSeries({
        upColor: "#16a34a",
        downColor: "#dc2626",
        borderUpColor: "#16a34a",
        borderDownColor: "#dc2626",
        wickUpColor: "#16a34a",
        wickDownColor: "#dc2626",
      });
      series.setData(safeData as CandlestickDatum[]);
    } else {
      const series = chart.addLineSeries({ color: "#18181b", lineWidth: 2 });
      series.setData(safeData as LineDatum[]);
    }

    const resize = () => {
      if (!container) return;
      chart.applyOptions({ width: container.clientWidth });
    };
    resize();
    const observer = new ResizeObserver(() => resize());
    observer.observe(container);
    return () => {
      observer.disconnect();
      chart.remove();
    };
  }, [chartHeight, safeData, type]);

  return (
    <div className="rounded-2xl border border-zinc-200 bg-white p-3">
      {title ? <div className="mb-2 text-sm font-semibold text-zinc-900">{title}</div> : null}
      <div ref={containerRef} />
    </div>
  );
}
