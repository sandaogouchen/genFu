import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import AssistantCard from "@/components/conversation/AssistantCard";
import CollapsibleSection from "@/components/conversation/CollapsibleSection";
import Composer from "@/components/conversation/Composer";
import Markdown from "@/components/conversation/Markdown";
import SectionedMarkdown from "@/components/conversation/SectionedMarkdown";
import type { PromptTemplate } from "@/components/conversation/TemplateChips";
import TipCard from "@/components/conversation/TipCard";
import UserBubble from "@/components/conversation/UserBubble";
import { toast } from "@/hooks/useToast";
import { usePageSessionStore } from "@/stores/pageSessionStore";
import type {
  ConversationRun,
  HoldingsOutput,
  MarketMove,
  NewsSummaryOutput,
  StockWorkflowOutput,
} from "@/utils/genfuApi";
import {
  createConversationSession,
  listConversationRuns,
  postSSE,
} from "@/utils/genfuApi";

type WorkflowDraft = { symbol: string; news_limit?: number };
type WorkflowPlan = {
  order?: string[];
  enabled?: string[];
  skipped?: Array<{ node?: string; reason?: string }>;
};
type WorkflowNodeStatus = Record<string, "running" | "done" | "skipped">;
type WorkflowRun = {
  id: string;
  prompt: string;
  req: WorkflowDraft;
  error?: string;
  plan?: WorkflowPlan;
  node_status?: WorkflowNodeStatus;
  holdings?: HoldingsOutput;
  holdings_market?: MarketMove[];
  target_market?: MarketMove;
  news?: NewsSummaryOutput;
  bull?: string;
  bear?: string;
  debate?: string;
  summary?: string;
};

function parseWorkflowPrompt(prompt: string): WorkflowDraft {
  const raw = prompt.trim();
  if (!raw) throw new Error("empty_prompt");

  if (raw.startsWith("{")) {
    try {
      const obj = JSON.parse(raw) as Partial<WorkflowDraft> & { symbol?: unknown; news_limit?: unknown };
      const symbol = String(obj.symbol ?? "").trim();
      const newsLimitRaw = obj.news_limit;
      const news_limit = typeof newsLimitRaw === "number" ? newsLimitRaw : typeof newsLimitRaw === "string" ? Number(newsLimitRaw) : undefined;
      if (!symbol) throw new Error("missing_symbol");
      return { symbol, news_limit: Number.isFinite(news_limit ?? NaN) ? (news_limit as number) : undefined };
    } catch {
      throw new Error("invalid_json_prompt");
    }
  }

  const symbolMatch = raw.match(/\b\d{6}\b/);
  const newsMatch = raw.match(/news_limit\s*[:=]\s*(\d+)/i) ?? raw.match(/新闻\s*(\d+)/);
  const news_limit = newsMatch?.[1] ? Number(newsMatch[1]) : undefined;

  if (symbolMatch?.[0]) {
    return { symbol: symbolMatch[0], news_limit: Number.isFinite(news_limit ?? NaN) ? news_limit : undefined };
  }

  let normalized = raw;
  normalized = normalized.replace(/news_limit\s*[:=]\s*\d+/gi, " ");
  normalized = normalized.replace(/新闻\s*\d+/g, " ");
  normalized = normalized.replace(/运行|启动|执行|工作流|workflow/gi, " ");
  normalized = normalized.replace(/输出[:：].*$/g, " ");
  normalized = normalized.replace(/[，,]/g, " ");
  const symbol = normalized.trim();
  if (!symbol) throw new Error("missing_symbol");

  return { symbol, news_limit: Number.isFinite(news_limit ?? NaN) ? news_limit : undefined };
}

function getRecord(v: unknown): Record<string, unknown> {
  if (!v || typeof v !== "object") return {};
  return v as Record<string, unknown>;
}

function toWorkflowRun(run: ConversationRun): WorkflowRun {
  const reqObj = getRecord(run.request);
  const result = getRecord(run.result) as unknown as Partial<StockWorkflowOutput>;
  return {
    id: String(run.id),
    prompt: run.prompt || "工作流请求",
    req: {
      symbol: String(reqObj.symbol ?? ""),
      news_limit: typeof reqObj.news_limit === "number" ? reqObj.news_limit : undefined,
    },
    error: run.error,
    holdings: result.holdings,
    holdings_market: result.holdings_market,
    target_market: result.target_market,
    news: result.news,
    bull: result.bull_analysis,
    bear: result.bear_analysis,
    debate: result.debate_analysis,
    summary: result.summary,
  };
}

function hasMarketData(move: MarketMove | undefined | null): boolean {
  if (!move) return false;
  return Boolean(
    (move.symbol ?? "").trim() ||
      (move.name ?? "").trim() ||
      (move.error ?? "").trim() ||
      (typeof move.price === "number" && move.price > 0)
  );
}

function formatSigned(value: number | undefined, fractionDigits = 2): string {
  if (typeof value !== "number" || Number.isNaN(value)) return "--";
  const fixed = value.toFixed(fractionDigits);
  return value > 0 ? `+${fixed}` : fixed;
}

function toPrettyJson(value: unknown, fallback: unknown): string {
  return JSON.stringify(value ?? fallback, null, 2);
}

function nodeStatusClass(status: "running" | "done" | "skipped"): string {
  if (status === "done") return "border-emerald-500/40 bg-emerald-500/10 text-emerald-300";
  if (status === "skipped") return "border-zinc-500/40 bg-zinc-500/10 text-zinc-300";
  return "border-sky-500/40 bg-sky-500/10 text-sky-300";
}

function TargetMarketCard({ move }: { move?: MarketMove }) {
  const hasData = hasMarketData(move);
  const up = typeof move?.change === "number" ? move.change >= 0 : false;
  return (
    <div className="rounded-xl border border-border/50 bg-card p-3">
      <div className="text-sm font-semibold text-foreground">target_market</div>
      {!hasData ? (
        <div className="mt-2 text-sm text-muted-foreground">暂无有效数据</div>
      ) : (
        <div className="mt-2 space-y-1 text-sm">
          <div className="font-medium text-foreground">
            {(move?.name ?? "").trim() || "--"} <span className="font-mono text-xs text-muted-foreground">{(move?.symbol ?? "").trim() || "--"}</span>
          </div>
          <div className="text-muted-foreground">价格：{typeof move?.price === "number" ? move.price.toFixed(3) : "--"}</div>
          <div className={up ? "text-emerald-500" : "text-destructive"}>
            涨跌：{formatSigned(move?.change)} ({formatSigned(move?.change_rate)}%)
          </div>
          {move?.error ? <div className="text-destructive">错误：{move.error}</div> : null}
        </div>
      )}
    </div>
  );
}

function HoldingsMarketCard({ moves }: { moves?: MarketMove[] }) {
  const list = (moves ?? []).filter((item) => hasMarketData(item));
  return (
    <div className="rounded-xl border border-border/50 bg-card p-3">
      <div className="text-sm font-semibold text-foreground">holdings_market</div>
      {list.length === 0 ? (
        <div className="mt-2 text-sm text-muted-foreground">暂无有效数据</div>
      ) : (
        <div className="mt-2 space-y-2">
          {list.map((item, idx) => {
            const up = typeof item.change === "number" ? item.change >= 0 : false;
            return (
              <div key={`${item.symbol ?? "unknown"}-${idx}`} className="rounded-lg border border-border/40 bg-muted/20 p-2 text-sm">
                <div className="font-medium text-foreground">
                  {(item.name ?? "").trim() || "--"} <span className="font-mono text-xs text-muted-foreground">{(item.symbol ?? "").trim() || "--"}</span>
                </div>
                <div className="text-muted-foreground">价格：{typeof item.price === "number" ? item.price.toFixed(3) : "--"}</div>
                <div className={up ? "text-emerald-500" : "text-destructive"}>
                  涨跌：{formatSigned(item.change)} ({formatSigned(item.change_rate)}%)
                </div>
                {item.error ? <div className="text-destructive">错误：{item.error}</div> : null}
              </div>
            );
          })}
        </div>
      )}
    </div>
  );
}

function NewsSummaryCard({ news }: { news?: NewsSummaryOutput }) {
  const items = news?.items ?? [];
  const hasData = Boolean((news?.summary ?? "").trim() || (news?.sentiment ?? "").trim() || items.length > 0);
  return (
    <div className="rounded-xl border border-border/50 bg-card p-3">
      <div className="text-sm font-semibold text-foreground">news_summary</div>
      {!hasData ? (
        <div className="mt-2 text-sm text-muted-foreground">暂无有效数据</div>
      ) : (
        <div className="mt-2 space-y-2 text-sm">
          <div>
            <span className="text-muted-foreground">情绪：</span>
            <span className="text-foreground">{news?.sentiment?.trim() || "--"}</span>
          </div>
          <div>
            <span className="text-muted-foreground">摘要：</span>
            <span className="text-foreground">{news?.summary?.trim() || "--"}</span>
          </div>
          <div className="text-muted-foreground">新闻条数：{items.length}</div>
          {items.length > 0 ? (
            <div className="space-y-1">
              {items.slice(0, 3).map((item, idx) => (
                <div key={`${item.link ?? item.title ?? "item"}-${idx}`} className="text-muted-foreground">
                  • {item.title?.trim() || item.link?.trim() || "无标题"}
                </div>
              ))}
            </div>
          ) : null}
        </div>
      )}
    </div>
  );
}

export default function Workflow() {
  const abortRef = useRef<AbortController | null>(null);
  const activeByScene = usePageSessionStore((s) => s.activeByScene);
  const setActive = usePageSessionStore((s) => s.setActive);
  const sessionId = activeByScene.workflow ?? "";

  const [input, setInput] = useState<string>("");
  const [loading, setLoading] = useState(false);
  const [runs, setRuns] = useState<WorkflowRun[]>([]);

  const loadRuns = useCallback(async (sid: string) => {
    if (!sid) {
      setRuns([]);
      return;
    }
    try {
      const data = await listConversationRuns(sid, 100);
      setRuns((data.items ?? []).map(toWorkflowRun));
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
        const created = await createConversationSession({ scene: "workflow", title: "工作流日志 1" });
        if (!canceled) {
          setActive("workflow", created.id);
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

  const templates = useMemo<PromptTemplate[]>(
    () => [
      { label: "工作流 600519", value: "运行工作流 600519\n输出：持仓影响/新闻摘要/多空辩论/最终结论" },
      { label: "工作流 + 新闻数", value: "工作流 600519 news_limit=6" },
      { label: "JSON 模板", value: '{"symbol":"600519","news_limit":6}' },
    ],
    []
  );

  const canSubmit = useMemo(() => input.trim().length > 0 && !loading, [input, loading]);

  return (
    <div className="space-y-4">
      <TipCard title="提示：直接用自然语言启动工作流（无表单）" dismissKey="workflow">
        推荐格式：<span className="font-mono">工作流 600519 news_limit=6</span>。默认会输出 holdings、新闻摘要、多空观点与总结。
      </TipCard>

      <div className="space-y-5">
        {runs.map((r) => (
          <div key={r.id} className="space-y-4">
            <UserBubble content={r.prompt} />
            <AssistantCard badge="GenFu" title={`工作流：${r.req.symbol}`}>
              <div className="text-sm text-muted-foreground">
                symbol: <span className="font-mono text-foreground">{r.req.symbol}</span>
                {r.req.news_limit != null ? <span className="ml-3">news_limit: {r.req.news_limit}</span> : null}
              </div>
              {r.error ? <div className="text-sm text-destructive">{r.error}</div> : null}
              {r.node_status && Object.keys(r.node_status).length > 0 ? (
                <div className="mt-1 flex flex-wrap gap-2">
                  {Object.entries(r.node_status).map(([node, status]) => (
                    <span
                      key={`${r.id}-${node}`}
                      className={`rounded-full border px-2 py-0.5 text-xs font-medium ${nodeStatusClass(status)}`}
                    >
                      {node}:{status}
                    </span>
                  ))}
                </div>
              ) : null}
              {r.plan ? (
                <CollapsibleSection title="workflow_plan">
                  <Markdown source={`\`\`\`json\n${toPrettyJson(r.plan, {})}\n\`\`\``} />
                </CollapsibleSection>
              ) : null}

              <div className="grid gap-3 md:grid-cols-2">
                <TargetMarketCard move={r.target_market} />
                <NewsSummaryCard news={r.news} />
              </div>
              <HoldingsMarketCard moves={r.holdings_market} />

              {r.summary ? (
                <CollapsibleSection title="summary" defaultOpen>
                  <SectionedMarkdown source={r.summary} />
                </CollapsibleSection>
              ) : null}
              {r.debate ? (
                <CollapsibleSection title="debate">
                  <Markdown source={r.debate} />
                </CollapsibleSection>
              ) : null}
              {r.bear ? (
                <CollapsibleSection title="bear">
                  <Markdown source={r.bear} />
                </CollapsibleSection>
              ) : null}
              {r.bull ? (
                <CollapsibleSection title="bull">
                  <Markdown source={r.bull} />
                </CollapsibleSection>
              ) : null}
              <CollapsibleSection title="news_summary (raw json)">
                <Markdown source={`\`\`\`json\n${toPrettyJson(r.news, { items: null, summary: "", sentiment: "" })}\n\`\`\``} />
              </CollapsibleSection>
              <CollapsibleSection title="target_market (raw json)">
                <Markdown
                  source={`\`\`\`json\n${toPrettyJson(r.target_market, {
                    symbol: "",
                    name: "",
                    price: 0,
                    change: 0,
                    change_rate: 0,
                    error: "",
                  })}\n\`\`\``}
                />
              </CollapsibleSection>
              <CollapsibleSection title="holdings_market (raw json)">
                <Markdown source={`\`\`\`json\n${toPrettyJson(r.holdings_market, [])}\n\`\`\``} />
              </CollapsibleSection>
              {r.holdings ? (
                <CollapsibleSection title="holdings">
                  <Markdown source={`\`\`\`json\n${JSON.stringify(r.holdings, null, 2)}\n\`\`\``} />
                </CollapsibleSection>
              ) : null}
            </AssistantCard>
          </div>
        ))}
      </div>

      <Composer
        value={input}
        onChange={setInput}
        disabled={loading}
        templates={templates}
        onSubmit={async () => {
          if (!canSubmit) return;
          if (!sessionId) {
            toast({ title: "请稍候", description: "会话初始化中" });
            return;
          }
          const prompt = input.trim();
          setInput("");

          let req: WorkflowDraft;
          try {
            req = parseWorkflowPrompt(prompt);
          } catch (e) {
            const msg = e instanceof Error ? e.message : "invalid_prompt";
            toast({ title: "无法解析", description: msg, durationMs: 5200 });
            return;
          }

          const tempId = `temp_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`;
          setRuns((xs) => [...xs, { id: tempId, prompt, req }]);

          abortRef.current?.abort();
          const ac = new AbortController();
          abortRef.current = ac;
          setLoading(true);
          try {
            const patchTempRun = (updater: (run: WorkflowRun) => WorkflowRun) => {
              setRuns((xs) => xs.map((x) => (x.id === tempId ? updater(x) : x)));
            };
            for await (const msg of postSSE(
              "/sse/workflow/stock",
              {
                ...req,
                session_id: sessionId,
                session_title: prompt.slice(0, 20),
                prompt,
              },
              { signal: ac.signal }
            )) {
              if (msg.event === "plan") {
                const payload = JSON.parse(msg.data) as WorkflowPlan;
                patchTempRun((x) => ({ ...x, plan: payload }));
              } else if (msg.event === "node_start") {
                const payload = JSON.parse(msg.data) as { node?: string };
                const node = String(payload.node ?? "").trim();
                if (!node) continue;
                patchTempRun((x) => ({
                  ...x,
                  node_status: { ...(x.node_status ?? {}), [node]: "running" },
                }));
              } else if (msg.event === "node_skip") {
                const payload = JSON.parse(msg.data) as { node?: string };
                const node = String(payload.node ?? "").trim();
                if (!node) continue;
                patchTempRun((x) => ({
                  ...x,
                  node_status: { ...(x.node_status ?? {}), [node]: "skipped" },
                }));
              } else if (msg.event === "node_complete") {
                const payload = JSON.parse(msg.data) as { node?: string };
                const node = String(payload.node ?? "").trim();
                if (!node) continue;
                patchTempRun((x) => ({
                  ...x,
                  node_status: { ...(x.node_status ?? {}), [node]: "done" },
                }));
              } else if (msg.event === "node_delta") {
                const payload = JSON.parse(msg.data) as { node?: string; delta?: string };
                const node = String(payload.node ?? "").trim();
                const delta = String(payload.delta ?? "");
                if (!node || !delta) continue;
                patchTempRun((x) => {
                  const next: WorkflowRun = {
                    ...x,
                    node_status: { ...(x.node_status ?? {}), [node]: "running" },
                  };
                  if (node === "bull") next.bull = (x.bull ?? "") + delta;
                  if (node === "bear") next.bear = (x.bear ?? "") + delta;
                  if (node === "debate") next.debate = (x.debate ?? "") + delta;
                  if (node === "summary") next.summary = (x.summary ?? "") + delta;
                  return next;
                });
              } else if (msg.event === "holdings") {
                const payload = JSON.parse(msg.data) as HoldingsOutput;
                patchTempRun((x) => ({ ...x, holdings: payload }));
              } else if (msg.event === "holdings_market") {
                const payload = JSON.parse(msg.data) as MarketMove[];
                patchTempRun((x) => ({ ...x, holdings_market: payload }));
              } else if (msg.event === "target_market") {
                const payload = JSON.parse(msg.data) as MarketMove;
                patchTempRun((x) => ({ ...x, target_market: payload }));
              } else if (msg.event === "news_summary") {
                const payload = JSON.parse(msg.data) as NewsSummaryOutput;
                patchTempRun((x) => ({ ...x, news: payload }));
              } else if (msg.event === "bull") {
                const payload = JSON.parse(msg.data) as { content?: string };
                patchTempRun((x) => ({ ...x, bull: payload.content ?? "" }));
              } else if (msg.event === "bear") {
                const payload = JSON.parse(msg.data) as { content?: string };
                patchTempRun((x) => ({ ...x, bear: payload.content ?? "" }));
              } else if (msg.event === "debate") {
                const payload = JSON.parse(msg.data) as { content?: string };
                patchTempRun((x) => ({ ...x, debate: payload.content ?? "" }));
              } else if (msg.event === "summary") {
                const payload = JSON.parse(msg.data) as { content?: string };
                patchTempRun((x) => ({ ...x, summary: payload.content ?? "" }));
              } else if (msg.event === "error") {
                const payload = JSON.parse(msg.data) as { error?: string };
                throw new Error(payload.error ?? "error");
              }
            }
            await loadRuns(sessionId);
            window.dispatchEvent(new Event("genfu:conversation-updated"));
            toast({ title: "工作流完成", description: "已输出各阶段结果" });
          } catch (e) {
            const err = e instanceof Error ? e.message : "unknown_error";
            setRuns((xs) => xs.map((x) => (x.id === tempId ? { ...x, error: err } : x)));
            await loadRuns(sessionId);
            window.dispatchEvent(new Event("genfu:conversation-updated"));
            toast({ title: "工作流失败", description: err, durationMs: 5200 });
          } finally {
            setLoading(false);
          }
        }}
      />
    </div>
  );
}
