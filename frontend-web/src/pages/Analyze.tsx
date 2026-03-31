import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import AssistantCard from "@/components/conversation/AssistantCard";
import CollapsibleSection from "@/components/conversation/CollapsibleSection";
import Composer from "@/components/conversation/Composer";
import Markdown from "@/components/conversation/Markdown";
import type { PromptTemplate } from "@/components/conversation/TemplateChips";
import TipCard from "@/components/conversation/TipCard";
import UserBubble from "@/components/conversation/UserBubble";
import Button from "@/components/ui/Button";
import { toast } from "@/hooks/useToast";
import { usePageSessionStore } from "@/stores/pageSessionStore";
import type { AnalyzeResponse, AnalyzeStep, ConversationRun } from "@/utils/genfuApi";
import {
  createConversationSession,
  listConversationRuns,
  postSSE,
} from "@/utils/genfuApi";

type AnalyzeDraft = { type: "fund" | "stock"; symbol: string; name?: string };
type AnalyzeRun = {
  id: string;
  prompt: string;
  req: AnalyzeDraft;
  steps: AnalyzeStep[];
  summary: AnalyzeResponse | null;
  error?: string;
};

const LOCAL_ANALYZE_SESSION_PREFIX = "local_analyze_";

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

function buildLocalAnalyzeSessionId() {
  return `${LOCAL_ANALYZE_SESSION_PREFIX}${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`;
}

function isLocalAnalyzeSession(sessionId: string) {
  return sessionId.startsWith(LOCAL_ANALYZE_SESSION_PREFIX);
}

function isSessionNotFoundError(message: string) {
  const m = message.trim().toLowerCase();
  return m.includes("session_not_found") || m.includes("http 404");
}

function getRecord(v: unknown): Record<string, unknown> {
  if (!v || typeof v !== "object") return {};
  return v as Record<string, unknown>;
}

function toAnalyzeRun(run: ConversationRun): AnalyzeRun {
  const reqObj = getRecord(run.request);
  const resultObj = getRecord(run.result);
  const stepsRaw = Array.isArray(resultObj.steps) ? (resultObj.steps as AnalyzeStep[]) : [];
  const req: AnalyzeDraft = {
    type: reqObj.type === "fund" ? "fund" : "stock",
    symbol: String(reqObj.symbol ?? ""),
    name: typeof reqObj.name === "string" ? reqObj.name : undefined,
  };
  const summary: AnalyzeResponse | null =
    Object.keys(resultObj).length > 0
      ? ({
          type: String(resultObj.type ?? req.type),
          symbol: String(resultObj.symbol ?? req.symbol),
          name: typeof resultObj.name === "string" ? resultObj.name : req.name,
          steps: stepsRaw,
          summary: typeof resultObj.summary === "string" ? resultObj.summary : "",
        } as AnalyzeResponse)
      : null;
  return {
    id: String(run.id),
    prompt: run.prompt || "分析请求",
    req,
    steps: stepsRaw,
    summary,
    error: run.error,
  };
}

export default function Analyze() {
  const abortRef = useRef<AbortController | null>(null);
  const sessionInitRef = useRef<Promise<string> | null>(null);
  const activeByScene = usePageSessionStore((s) => s.activeByScene);
  const setActive = usePageSessionStore((s) => s.setActive);
  const sessionId = activeByScene.analyze ?? "";

  const [input, setInput] = useState<string>("");
  const [loading, setLoading] = useState(false);
  const [runs, setRuns] = useState<AnalyzeRun[]>([]);
  const [sessionInitLoading, setSessionInitLoading] = useState(false);
  const [sessionInitError, setSessionInitError] = useState<string>("");

  const usingLocalSession = useMemo(() => isLocalAnalyzeSession(sessionId), [sessionId]);

  const loadRuns = useCallback(async (sid: string) => {
    if (!sid) {
      setRuns([]);
      return;
    }
    try {
      const data = await listConversationRuns(sid, 100);
      setRuns((data.items ?? []).map(toAnalyzeRun));
    } catch (err) {
      const msg = err instanceof Error ? err.message : "加载失败";
      if (isLocalAnalyzeSession(sid) && isSessionNotFoundError(msg)) {
        setRuns([]);
        return;
      }
      toast({ title: "加载会话失败", description: msg, durationMs: 4200 });
      setRuns([]);
    }
  }, []);

  const activateLocalSession = useCallback(() => {
    if (sessionId && isLocalAnalyzeSession(sessionId)) {
      return sessionId;
    }
    const localId = buildLocalAnalyzeSessionId();
    setActive("analyze", localId);
    setSessionInitError("");
    window.dispatchEvent(new Event("genfu:conversation-updated"));
    toast({
      title: "已启用本地临时会话",
      description: "网络恢复后可重试创建云端会话。",
      durationMs: 4200,
    });
    return localId;
  }, [sessionId, setActive]);

  const startSessionInit = useCallback(
    async (options?: { allowFallback?: boolean; silent?: boolean; force?: boolean }) => {
      const force = options?.force ?? false;
      if (sessionId && !(force && isLocalAnalyzeSession(sessionId))) {
        return sessionId;
      }

      const allowFallback = options?.allowFallback ?? false;
      const silent = options?.silent ?? false;
      if (sessionInitRef.current) {
        const existing = await sessionInitRef.current;
        if (existing) return existing;
        return allowFallback ? activateLocalSession() : "";
      }

      const task = (async () => {
        setSessionInitLoading(true);
        setSessionInitError("");
        try {
          const created = await createConversationSession(
            { scene: "analyze", title: "分析日志 1" },
            { retries: 2, initialDelayMs: 450, maxDelayMs: 2600 }
          );
          setActive("analyze", created.id);
          window.dispatchEvent(new Event("genfu:conversation-updated"));
          return created.id;
        } catch (err) {
          const msg = err instanceof Error ? err.message : "创建会话失败";
          setSessionInitError(msg);
          if (!silent) {
            toast({ title: "创建会话失败", description: msg, durationMs: 4200 });
          }
          return "";
        } finally {
          setSessionInitLoading(false);
        }
      })();

      sessionInitRef.current = task;
      const sid = await task;
      if (sessionInitRef.current === task) {
        sessionInitRef.current = null;
      }
      if (sid) return sid;
      return allowFallback ? activateLocalSession() : "";
    },
    [sessionId, setActive, activateLocalSession]
  );

  useEffect(() => {
    if (sessionId) return;
    void startSessionInit({ allowFallback: false, silent: true });
  }, [sessionId, startSessionInit]);

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
        推荐格式：<span className="font-mono text-accent">分析股票 600519 贵州茅台</span> 或{" "}
        <span className="font-mono text-accent">分析基金 161725</span>。也支持直接粘贴 JSON：
        <span className="font-mono text-accent">{"{\"type\":\"stock\",\"symbol\":\"600519\"}"}</span>。
      </TipCard>

      {!sessionId ? (
        <div className="rounded-2xl border border-warning/40 bg-warning/10 p-4">
          <div className="text-sm font-semibold text-foreground">
            {sessionInitLoading ? "会话初始化中…" : "会话未就绪"}
          </div>
          <div className="mt-1 text-xs text-muted-foreground">
            {sessionInitError
              ? `最近一次失败：${sessionInitError}`
              : "将自动重试；也可手动重试或降级为本地临时会话。"}
          </div>
          <div className="mt-3 flex flex-wrap items-center gap-2">
            <Button
              size="sm"
              disabled={sessionInitLoading}
              onClick={() => {
                void startSessionInit({ allowFallback: false });
              }}
            >
              {sessionInitLoading ? "重试中…" : "重试创建会话"}
            </Button>
            <Button size="sm" variant="secondary" disabled={sessionInitLoading} onClick={activateLocalSession}>
              使用本地临时会话
            </Button>
          </div>
        </div>
      ) : usingLocalSession ? (
        <div className="rounded-2xl border border-warning/40 bg-warning/10 p-4">
          <div className="text-sm font-semibold text-foreground">当前使用本地临时会话</div>
          <div className="mt-1 text-xs text-muted-foreground">可继续分析；恢复网络后可重试创建云端会话。</div>
          <div className="mt-3">
            <Button
              size="sm"
              disabled={sessionInitLoading}
              onClick={() => {
                void startSessionInit({ allowFallback: false, force: true });
              }}
            >
              {sessionInitLoading ? "重试中…" : "重试创建云端会话"}
            </Button>
          </div>
        </div>
      ) : null}

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
          let sid = sessionId;
          if (!sid) {
            sid = await startSessionInit({ allowFallback: true, silent: true });
          }
          if (!sid) {
            toast({ title: "会话不可用", description: "请重试创建会话或切换本地临时会话", durationMs: 5200 });
            return;
          }

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

          const tempId = `temp_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`;
          setRuns((xs) => [...xs, { id: tempId, prompt, req, steps: [], summary: null }]);

          abortRef.current?.abort();
          const ac = new AbortController();
          abortRef.current = ac;
          setLoading(true);
          try {
            for await (const msg of postSSE(
              "/sse/analyze",
              {
                ...req,
                session_id: sid,
                session_title: prompt.slice(0, 20),
                prompt,
              },
              { signal: ac.signal }
            )) {
              if (msg.event === "step") {
                const step = JSON.parse(msg.data) as AnalyzeStep;
                setRuns((xs) => xs.map((x) => (x.id === tempId ? { ...x, steps: [...x.steps, step] } : x)));
              } else if (msg.event === "summary") {
                const resp = JSON.parse(msg.data) as AnalyzeResponse;
                setRuns((xs) => xs.map((x) => (x.id === tempId ? { ...x, summary: resp } : x)));
              } else if (msg.event === "error") {
                const payload = JSON.parse(msg.data) as { error?: string; step?: string };
                throw new Error(payload.step ? `${payload.step}: ${payload.error ?? "error"}` : payload.error ?? "error");
              }
            }
            await loadRuns(sid);
            window.dispatchEvent(new Event("genfu:conversation-updated"));
            toast({ title: "分析完成", description: "已完成流式分析" });
          } catch (e) {
            const err = e instanceof Error ? e.message : "unknown_error";
            setRuns((xs) => xs.map((x) => (x.id === tempId ? { ...x, error: err } : x)));
            await loadRuns(sid);
            window.dispatchEvent(new Event("genfu:conversation-updated"));
            toast({ title: "分析失败", description: err, durationMs: 5200 });
          } finally {
            setLoading(false);
          }
        }}
      />
    </div>
  );
}
