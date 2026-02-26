import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import AssistantCard from "@/components/conversation/AssistantCard";
import CollapsibleSection from "@/components/conversation/CollapsibleSection";
import Markdown from "@/components/conversation/Markdown";
import TipCard from "@/components/conversation/TipCard";
import UserBubble from "@/components/conversation/UserBubble";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import { toast } from "@/hooks/useToast";
import { usePageSessionStore } from "@/stores/pageSessionStore";
import type {
  ConversationRun,
  DecisionOutput,
  ExecutionResult,
  TradeSignal,
} from "@/utils/genfuApi";
import {
  createConversationSession,
  listConversationRuns,
  postSSE,
} from "@/utils/genfuApi";

type DecisionRun = {
  id: string;
  prompt: string;
  decision?: DecisionOutput;
  signals?: TradeSignal[];
  executions?: ExecutionResult[];
  error?: string;
};

function getRecord(v: unknown): Record<string, unknown> {
  if (!v || typeof v !== "object") return {};
  return v as Record<string, unknown>;
}

function toDecisionRun(run: ConversationRun): DecisionRun {
  const result = getRecord(run.result);
  return {
    id: String(run.id),
    prompt: run.prompt || "决策请求",
    decision: (result.decision as DecisionOutput) ?? undefined,
    signals: Array.isArray(result.signals) ? (result.signals as TradeSignal[]) : [],
    executions: Array.isArray(result.executions) ? (result.executions as ExecutionResult[]) : [],
    error: run.error,
  };
}

export default function Decision() {
  const abortRef = useRef<AbortController | null>(null);
  const activeByScene = usePageSessionStore((s) => s.activeByScene);
  const setActive = usePageSessionStore((s) => s.setActive);
  const sessionId = activeByScene.decision ?? "";

  const [accountId, setAccountId] = useState<string>("");
  const [reportIds, setReportIds] = useState<string>("");
  const [loading, setLoading] = useState(false);
  const [runs, setRuns] = useState<DecisionRun[]>([]);

  const loadRuns = useCallback(async (sid: string) => {
    if (!sid) {
      setRuns([]);
      return;
    }
    try {
      const data = await listConversationRuns(sid, 100);
      setRuns((data.items ?? []).map(toDecisionRun));
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
        const created = await createConversationSession({ scene: "decision", title: "决策日志 1" });
        if (!canceled) {
          setActive("decision", created.id);
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

  const parsedReportIDs = useMemo(
    () =>
      reportIds
        .split(",")
        .map((x) => x.trim())
        .filter(Boolean)
        .map((x) => Number(x))
        .filter((x) => Number.isFinite(x)),
    [reportIds]
  );

  return (
    <div className="space-y-4">
      <TipCard title="提示：决策日志会保存在当前页面会话中" dismissKey="decision">
        默认只需要点击“开始决策”；`report_ids` 已收纳为高级选项，按需填写。
      </TipCard>

      <div className="rounded-2xl border border-border/50 bg-card p-4">
        <div className="flex flex-col gap-3 md:flex-row md:items-center">
          <div className="flex-1">
            <div className="text-base font-semibold text-foreground">投资决策系统</div>
            <div className="text-sm text-muted-foreground mt-0.5">结果将写入当前页面会话</div>
          </div>
          <div className="flex items-center gap-2">
            <Input
              value={accountId}
              onChange={(e) => setAccountId(e.target.value)}
              placeholder="account_id（可选）"
              className="w-44"
            />
            <Button
              disabled={loading || !sessionId}
              onClick={async () => {
                if (!sessionId) return;
                abortRef.current?.abort();
                const ac = new AbortController();
                abortRef.current = ac;

                const prompt = `开始决策 account_id=${accountId || "default"} report_ids=${parsedReportIDs.join(",") || "none"}`;
                const tempId = `temp_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`;
                setRuns((xs) => [...xs, { id: tempId, prompt }]);
                setLoading(true);

                try {
                  for await (const msg of postSSE(
                    "/sse/decision",
                    {
                      account_id: accountId ? Number(accountId) : undefined,
                      report_ids: parsedReportIDs,
                      session_id: sessionId,
                      session_title: prompt.slice(0, 20),
                      prompt,
                    },
                    { signal: ac.signal }
                  )) {
                    if (msg.event === "decision") {
                      const decision = JSON.parse(msg.data) as DecisionOutput;
                      setRuns((xs) => xs.map((x) => (x.id === tempId ? { ...x, decision } : x)));
                    } else if (msg.event === "signals") {
                      const signals = JSON.parse(msg.data) as TradeSignal[];
                      setRuns((xs) => xs.map((x) => (x.id === tempId ? { ...x, signals } : x)));
                    } else if (msg.event === "executions") {
                      const executions = JSON.parse(msg.data) as ExecutionResult[];
                      setRuns((xs) => xs.map((x) => (x.id === tempId ? { ...x, executions } : x)));
                    } else if (msg.event === "error") {
                      const payload = JSON.parse(msg.data) as { error?: string };
                      throw new Error(payload.error ?? "error");
                    }
                  }
                  await loadRuns(sessionId);
                  window.dispatchEvent(new Event("genfu:conversation-updated"));
                  toast({ title: "决策完成", description: "已写入会话日志" });
                } catch (err) {
                  const msg = err instanceof Error ? err.message : "unknown_error";
                  setRuns((xs) => xs.map((x) => (x.id === tempId ? { ...x, error: msg } : x)));
                  await loadRuns(sessionId);
                  window.dispatchEvent(new Event("genfu:conversation-updated"));
                  toast({ title: "决策失败", description: msg, durationMs: 5200 });
                } finally {
                  setLoading(false);
                }
              }}
            >
              {loading ? "生成中…" : "开始决策"}
            </Button>
            <Button
              variant="secondary"
              disabled={!loading}
              onClick={() => {
                abortRef.current?.abort();
                setLoading(false);
              }}
            >
              取消
            </Button>
          </div>
        </div>

        <CollapsibleSection title="高级选项">
          <Input
            value={reportIds}
            onChange={(e) => setReportIds(e.target.value)}
            placeholder="report_ids（可选，逗号分隔）"
          />
        </CollapsibleSection>
      </div>

      <div className="space-y-5">
        {runs.map((run) => (
          <div key={run.id} className="space-y-4">
            <UserBubble content={run.prompt} />
            <AssistantCard badge="GenFu" title="决策结果">
              {run.error ? <div className="text-sm text-destructive">{run.error}</div> : null}

              {run.decision ? (
                <CollapsibleSection title="decision" defaultOpen>
                  <Markdown source={`\`\`\`json\n${JSON.stringify(run.decision, null, 2)}\n\`\`\``} />
                </CollapsibleSection>
              ) : null}
              {run.signals && run.signals.length > 0 ? (
                <CollapsibleSection title="signals">
                  <Markdown source={`\`\`\`json\n${JSON.stringify(run.signals, null, 2)}\n\`\`\``} />
                </CollapsibleSection>
              ) : null}
              {run.executions && run.executions.length > 0 ? (
                <CollapsibleSection title="executions">
                  <Markdown source={`\`\`\`json\n${JSON.stringify(run.executions, null, 2)}\n\`\`\``} />
                </CollapsibleSection>
              ) : null}
            </AssistantCard>
          </div>
        ))}
      </div>
    </div>
  );
}
