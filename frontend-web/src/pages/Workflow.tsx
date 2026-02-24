import { useMemo, useRef, useState } from "react";

import AssistantCard from "@/components/conversation/AssistantCard";
import CollapsibleSection from "@/components/conversation/CollapsibleSection";
import Composer from "@/components/conversation/Composer";
import Markdown from "@/components/conversation/Markdown";
import SectionedMarkdown from "@/components/conversation/SectionedMarkdown";
import type { PromptTemplate } from "@/components/conversation/TemplateChips";
import TipCard from "@/components/conversation/TipCard";
import UserBubble from "@/components/conversation/UserBubble";
import { toast } from "@/hooks/useToast";
import type { HoldingsOutput, MarketMove, NewsSummaryOutput } from "@/utils/genfuApi";
import { postSSE } from "@/utils/genfuApi";

type WorkflowDraft = { symbol: string; news_limit?: number };
type WorkflowRun = {
  id: string;
  prompt: string;
  req: WorkflowDraft;
  error?: string;
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
  const symbol = symbolMatch?.[0] ?? "";
  if (!symbol) throw new Error("missing_symbol");

  const newsMatch = raw.match(/news_limit\s*[:=]\s*(\d+)/i) ?? raw.match(/新闻\s*(\d+)/);
  const news_limit = newsMatch?.[1] ? Number(newsMatch[1]) : undefined;
  return { symbol, news_limit: Number.isFinite(news_limit ?? NaN) ? news_limit : undefined };
}

export default function Workflow() {
  const abortRef = useRef<AbortController | null>(null);
  const [input, setInput] = useState<string>("");
  const [loading, setLoading] = useState(false);
  const [runs, setRuns] = useState<WorkflowRun[]>([]);

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
              {r.news ? (
                <CollapsibleSection title="news_summary">
                  <Markdown source={`\`\`\`json\n${JSON.stringify(r.news, null, 2)}\n\`\`\``} />
                </CollapsibleSection>
              ) : null}
              {r.target_market ? (
                <CollapsibleSection title="target_market">
                  <Markdown source={`\`\`\`json\n${JSON.stringify(r.target_market, null, 2)}\n\`\`\``} />
                </CollapsibleSection>
              ) : null}
              {r.holdings_market && r.holdings_market.length > 0 ? (
                <CollapsibleSection title="holdings_market">
                  <Markdown source={`\`\`\`json\n${JSON.stringify(r.holdings_market, null, 2)}\n\`\`\``} />
                </CollapsibleSection>
              ) : null}
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

          const id = `run_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`;
          setRuns((xs) => [...xs, { id, prompt, req }]);

          abortRef.current?.abort();
          const ac = new AbortController();
          abortRef.current = ac;
          setLoading(true);
          try {
            for await (const msg of postSSE("/sse/workflow/stock", req, { signal: ac.signal })) {
              if (msg.event === "holdings") {
                const payload = JSON.parse(msg.data) as HoldingsOutput;
                setRuns((xs) => xs.map((x) => (x.id === id ? { ...x, holdings: payload } : x)));
              } else if (msg.event === "holdings_market") {
                const payload = JSON.parse(msg.data) as MarketMove[];
                setRuns((xs) => xs.map((x) => (x.id === id ? { ...x, holdings_market: payload } : x)));
              } else if (msg.event === "target_market") {
                const payload = JSON.parse(msg.data) as MarketMove;
                setRuns((xs) => xs.map((x) => (x.id === id ? { ...x, target_market: payload } : x)));
              } else if (msg.event === "news_summary") {
                const payload = JSON.parse(msg.data) as NewsSummaryOutput;
                setRuns((xs) => xs.map((x) => (x.id === id ? { ...x, news: payload } : x)));
              } else if (msg.event === "bull") {
                const payload = JSON.parse(msg.data) as { content?: string };
                setRuns((xs) => xs.map((x) => (x.id === id ? { ...x, bull: payload.content ?? "" } : x)));
              } else if (msg.event === "bear") {
                const payload = JSON.parse(msg.data) as { content?: string };
                setRuns((xs) => xs.map((x) => (x.id === id ? { ...x, bear: payload.content ?? "" } : x)));
              } else if (msg.event === "debate") {
                const payload = JSON.parse(msg.data) as { content?: string };
                setRuns((xs) => xs.map((x) => (x.id === id ? { ...x, debate: payload.content ?? "" } : x)));
              } else if (msg.event === "summary") {
                const payload = JSON.parse(msg.data) as { content?: string };
                setRuns((xs) => xs.map((x) => (x.id === id ? { ...x, summary: payload.content ?? "" } : x)));
              } else if (msg.event === "error") {
                const payload = JSON.parse(msg.data) as { error?: string };
                throw new Error(payload.error ?? "error");
              }
            }
            toast({ title: "工作流完成", description: "已输出各阶段结果" });
          } catch (e) {
            const err = e instanceof Error ? e.message : "unknown_error";
            setRuns((xs) => xs.map((x) => (x.id === id ? { ...x, error: err } : x)));
            toast({ title: "工作流失败", description: err, durationMs: 5200 });
          } finally {
            setLoading(false);
          }
        }}
      />
    </div>
  );
}
