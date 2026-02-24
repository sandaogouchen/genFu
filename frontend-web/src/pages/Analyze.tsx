import { useMemo, useRef, useState } from "react";

import AssistantCard from "@/components/conversation/AssistantCard";
import CollapsibleSection from "@/components/conversation/CollapsibleSection";
import Composer from "@/components/conversation/Composer";
import Markdown from "@/components/conversation/Markdown";
import type { PromptTemplate } from "@/components/conversation/TemplateChips";
import TipCard from "@/components/conversation/TipCard";
import UserBubble from "@/components/conversation/UserBubble";
import { toast } from "@/hooks/useToast";
import type { AnalyzeResponse, AnalyzeStep } from "@/utils/genfuApi";
import { postSSE } from "@/utils/genfuApi";

type AnalyzeDraft = { type: "fund" | "stock"; symbol: string; name?: string };
type AnalyzeRun = {
  id: string;
  prompt: string;
  req: AnalyzeDraft;
  steps: AnalyzeStep[];
  summary: AnalyzeResponse | null;
  error?: string;
};

function parseAnalyzePrompt(prompt: string): AnalyzeDraft {
  const raw = prompt.trim();
  if (!raw) throw new Error("empty_prompt");

  if (raw.startsWith("{")) {
    try {
      const obj = JSON.parse(raw) as Partial<AnalyzeDraft> & { type?: unknown; symbol?: unknown; name?: unknown };
      const type = String(obj.type ?? "").trim() as "fund" | "stock";
      const symbol = String(obj.symbol ?? "").trim();
      const name = typeof obj.name === "string" ? obj.name.trim() : "";
      if (type !== "stock" && type !== "fund") throw new Error("invalid_type");
      if (!symbol) throw new Error("missing_symbol");
      return { type, symbol, name: name || undefined };
    } catch {
      throw new Error("invalid_json_prompt");
    }
  }

  const lower = raw.toLowerCase();
  const type: "fund" | "stock" = /\b(fund|基金)\b/.test(lower) || raw.includes("基金") ? "fund" : "stock";

  const symbolMatch = raw.match(/\b\d{6}\b/);
  const symbol = symbolMatch?.[0] ?? "";
  if (!symbol) throw new Error("missing_symbol");

  const rest = raw.replace(symbol, " ").replace(/分析|analyze|股票|stock|基金|fund/gi, " ").trim();
  const name = rest.replace(/\s+/g, " ").trim();
  return { type, symbol, name: name || undefined };
}

export default function Analyze() {
  const abortRef = useRef<AbortController | null>(null);
  const [input, setInput] = useState<string>("");
  const [loading, setLoading] = useState(false);
  const [runs, setRuns] = useState<AnalyzeRun[]>([]);

  const templates = useMemo<PromptTemplate[]>(
    () => [
      { label: "分析股票 600519", value: "分析股票 600519 贵州茅台\n关注：估值/催化/风险/操作建议" },
      { label: "分析基金 161725", value: "分析基金 161725\n关注：持仓结构/回撤/适合人群/仓位建议" },
      { label: "JSON 模板", value: '{"type":"stock","symbol":"600519","name":"贵州茅台"}' },
    ],
    []
  );

  const canSubmit = useMemo(() => input.trim().length > 0 && !loading, [input, loading]);

  return (
    <div className="space-y-5">
      <TipCard title="提示：把问题当成对话提问即可（无表单）" dismissKey="analyze">
        推荐格式：<span className="font-mono text-accent">分析股票 600519 贵州茅台</span> 或 <span className="font-mono text-accent">分析基金 161725</span>。
        也支持直接粘贴 JSON：<span className="font-mono text-accent">{"{\"type\":\"stock\",\"symbol\":\"600519\"}"}</span>。
      </TipCard>

      <div className="space-y-5">
        {runs.map((r) => (
          <div key={r.id} className="space-y-4">
            <UserBubble content={r.prompt} />
            <AssistantCard badge="GenFu" title={`分析：${r.req.symbol}${r.req.name ? `（${r.req.name}）` : ""}`}>
              <div className="space-y-3">
                <div className="flex items-center gap-4 text-sm text-muted-foreground">
                  <span>
                    type: <span className="font-mono text-foreground">{r.req.type}</span>
                  </span>
                  {r.summary?.report_id != null ? (
                    <span>report_id: <span className="text-foreground">{r.summary.report_id}</span></span>
                  ) : null}
                </div>
                {r.error ? <div className="text-sm text-destructive">{r.error}</div> : null}
              </div>

              {r.steps.length > 0 ? (
                <CollapsibleSection title="steps">
                  <div className="space-y-2">
                    {r.steps.map((s, idx) => (
                      <CollapsibleSection key={idx} title={(s.name || `step_${idx + 1}`).toLowerCase()}>
                        {s.output ? <Markdown source={s.output} /> : <div className="text-sm text-muted-foreground">等待输出…</div>}
                      </CollapsibleSection>
                    ))}
                  </div>
                </CollapsibleSection>
              ) : null}

              {r.summary?.summary ? (
                <CollapsibleSection title="summary" defaultOpen>
                  <Markdown source={r.summary.summary} />
                </CollapsibleSection>
              ) : null}

              {r.summary && !r.summary.summary ? (
                <CollapsibleSection title="raw" defaultOpen>
                  <Markdown source={`\`\`\`json\n${JSON.stringify(r.summary, null, 2)}\n\`\`\``} />
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

          let req: AnalyzeDraft;
          try {
            req = parseAnalyzePrompt(prompt);
          } catch (e) {
            const msg = e instanceof Error ? e.message : "invalid_prompt";
            toast({ title: "无法解析", description: msg, durationMs: 5200 });
            return;
          }

          const id = `run_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`;
          setRuns((xs) => [...xs, { id, prompt, req, steps: [], summary: null }]);

          abortRef.current?.abort();
          const ac = new AbortController();
          abortRef.current = ac;
          setLoading(true);
          try {
            for await (const msg of postSSE("/sse/analyze", req, { signal: ac.signal })) {
              if (msg.event === "step") {
                const step = JSON.parse(msg.data) as AnalyzeStep;
                setRuns((xs) => xs.map((x) => (x.id === id ? { ...x, steps: [...x.steps, step] } : x)));
              } else if (msg.event === "summary") {
                const resp = JSON.parse(msg.data) as AnalyzeResponse;
                setRuns((xs) => xs.map((x) => (x.id === id ? { ...x, summary: resp } : x)));
              } else if (msg.event === "error") {
                const payload = JSON.parse(msg.data) as { error?: string; step?: string };
                throw new Error(payload.step ? `${payload.step}: ${payload.error ?? "error"}` : payload.error ?? "error");
              }
            }
            toast({ title: "分析完成", description: "已完成流式分析" });
          } catch (e) {
            const err = e instanceof Error ? e.message : "unknown_error";
            setRuns((xs) => xs.map((x) => (x.id === id ? { ...x, error: err } : x)));
            toast({ title: "分析失败", description: err, durationMs: 5200 });
          } finally {
            setLoading(false);
          }
        }}
      />
    </div>
  );
}
