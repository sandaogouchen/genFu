import { useCallback, useEffect, useMemo, useRef, useState } from "react";

import AssistantCard from "@/components/conversation/AssistantCard";
import CollapsibleSection from "@/components/conversation/CollapsibleSection";
import Markdown from "@/components/conversation/Markdown";
import TipCard from "@/components/conversation/TipCard";
import UserBubble from "@/components/conversation/UserBubble";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Modal from "@/components/ui/Modal";
import Select from "@/components/ui/Select";
import { toast } from "@/hooks/useToast";
import { usePageSessionStore } from "@/stores/pageSessionStore";
import type {
  ConversationRun,
  GuardedOrder,
  DecisionOutput,
  ExecutionResult,
  GuideSelection,
  OperationGuide,
  PlannedOrder,
  PostTradeReview,
  RiskBudget,
  TradeSignal,
} from "@/utils/genfuApi";
import {
  createConversationSession,
  listConversationRuns,
  listOperationGuides,
  postJson,
  postSSE,
} from "@/utils/genfuApi";

type DecisionRun = {
  id: string;
  prompt: string;
  decision?: DecisionOutput;
  riskBudget?: RiskBudget;
  plannedOrders?: PlannedOrder[];
  guardedOrders?: GuardedOrder[];
  signals?: TradeSignal[];
  executions?: ExecutionResult[];
  review?: PostTradeReview;
  warnings?: string[];
  error?: string;
};

type SnapshotPosition = {
  instrument?: {
    symbol?: string;
    name?: string;
    asset_type?: string;
  };
  operation_guide_id?: number;
};

type PortfolioSnapshot = {
  positions?: SnapshotPosition[];
};

type HoldingGuideRow = {
  symbol: string;
  name: string;
  assetType: string;
  defaultGuideID?: number;
  guides: OperationGuide[];
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
    riskBudget: (result.risk_budget as RiskBudget) ?? undefined,
    plannedOrders: Array.isArray(result.planned_orders) ? (result.planned_orders as PlannedOrder[]) : [],
    guardedOrders: Array.isArray(result.guarded_orders) ? (result.guarded_orders as GuardedOrder[]) : [],
    signals: Array.isArray(result.signals) ? (result.signals as TradeSignal[]) : [],
    executions: Array.isArray(result.executions) ? (result.executions as ExecutionResult[]) : [],
    review: (result.review as PostTradeReview) ?? undefined,
    warnings: Array.isArray(result.warnings) ? (result.warnings as string[]) : [],
    error: run.error,
  };
}

function buildGuideLabel(guide: OperationGuide): string {
  const dateText = guide.created_at ? new Date(guide.created_at).toLocaleDateString("zh-CN") : "--";
  const text = (guide.trade_guide_text || "").trim();
  const summary = text ? text.slice(0, 26) : guide.stop_loss || guide.take_profit || "无摘要";
  return `#${guide.id} ${dateText} ${summary}`;
}

export default function Decision() {
  const abortRef = useRef<AbortController | null>(null);
  const activeByScene = usePageSessionStore((s) => s.activeByScene);
  const setActive = usePageSessionStore((s) => s.setActive);
  const sessionId = activeByScene.decision ?? "";

  const [accountId, setAccountId] = useState<string>("");
  const [loading, setLoading] = useState(false);
  const [runs, setRuns] = useState<DecisionRun[]>([]);
  const [guideModalOpen, setGuideModalOpen] = useState(false);
  const [guideLoading, setGuideLoading] = useState(false);
  const [guideRows, setGuideRows] = useState<HoldingGuideRow[]>([]);
  const [guideSelections, setGuideSelections] = useState<Record<string, number | undefined>>({});
  const [guideError, setGuideError] = useState<string>("");

  const parsedAccountID = useMemo(() => {
    if (!accountId.trim()) return undefined;
    const v = Number(accountId);
    if (!Number.isFinite(v) || v <= 0) return undefined;
    return v;
  }, [accountId]);

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

  const loadGuideRows = useCallback(async () => {
    setGuideLoading(true);
    setGuideError("");
    try {
      const resp = await postJson<{ output?: PortfolioSnapshot; error?: string }>("/api/investment", {
        action: "get_portfolio_snapshot",
        account_id: parsedAccountID,
      });
      if (resp.error) {
        throw new Error(resp.error);
      }
      const positions = resp.output?.positions ?? [];
      const bySymbol = new Map<string, SnapshotPosition>();
      for (const pos of positions) {
        const symbol = String(pos.instrument?.symbol || "").trim();
        if (!symbol) continue;
        if (!bySymbol.has(symbol)) {
          bySymbol.set(symbol, pos);
        }
      }
      const symbols = Array.from(bySymbol.keys());
      const guidesList = await Promise.all(
        symbols.map(async (symbol) => {
          try {
            const guides = await listOperationGuides(symbol);
            return { symbol, guides };
          } catch {
            return { symbol, guides: [] as OperationGuide[] };
          }
        })
      );
      const guideMap = new Map<string, OperationGuide[]>();
      for (const item of guidesList) {
        guideMap.set(item.symbol, item.guides);
      }

      const rows: HoldingGuideRow[] = symbols.map((symbol) => {
        const position = bySymbol.get(symbol);
        return {
          symbol,
          name: String(position?.instrument?.name || symbol),
          assetType: String(position?.instrument?.asset_type || "unknown"),
          defaultGuideID: position?.operation_guide_id,
          guides: guideMap.get(symbol) ?? [],
        };
      });
      rows.sort((a, b) => a.symbol.localeCompare(b.symbol));
      setGuideRows(rows);
      setGuideSelections((prev) => {
        const next: Record<string, number | undefined> = {};
        for (const row of rows) {
          const existing = prev[row.symbol];
          if (typeof existing === "number") {
            next[row.symbol] = existing;
          } else if (typeof row.defaultGuideID === "number") {
            next[row.symbol] = row.defaultGuideID;
          }
        }
        return next;
      });
    } catch (err) {
      const msg = err instanceof Error ? err.message : "加载失败";
      setGuideRows([]);
      setGuideError(msg);
    } finally {
      setGuideLoading(false);
    }
  }, [parsedAccountID]);

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

  useEffect(() => {
    if (!guideModalOpen) return;
    void loadGuideRows();
  }, [guideModalOpen, loadGuideRows]);

  const selectedGuidePayload = useMemo<GuideSelection[]>(() => {
    return Object.entries(guideSelections)
      .filter(([, guideID]) => typeof guideID === "number" && Number.isFinite(guideID))
      .map(([symbol, guideID]) => ({
        symbol,
        guide_id: Number(guideID),
      }));
  }, [guideSelections]);

  return (
    <div className="space-y-4">
      <TipCard title="提示：决策日志会保存在当前页面会话中" dismissKey="decision">
        可在“买卖指南”弹窗中为每个持仓选择历史指南；未选择的标的不影响决策提交。
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
              variant="secondary"
              disabled={loading}
              onClick={() => {
                setGuideModalOpen(true);
              }}
            >
              买卖指南（已选 {selectedGuidePayload.length}）
            </Button>
            <Button
              disabled={loading || !sessionId}
              onClick={async () => {
                if (!sessionId) return;
                abortRef.current?.abort();
                const ac = new AbortController();
                abortRef.current = ac;

                const prompt = `开始决策 account_id=${accountId || "default"} guide_count=${selectedGuidePayload.length}`;
                const tempId = `temp_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`;
                setRuns((xs) => [...xs, { id: tempId, prompt }]);
                setLoading(true);

                try {
                  for await (const msg of postSSE(
                    "/sse/decision",
                    {
                      account_id: parsedAccountID,
                      guide_selections: selectedGuidePayload,
                      session_id: sessionId,
                      session_title: prompt.slice(0, 20),
                      prompt,
                    },
                    { signal: ac.signal }
                  )) {
                    if (msg.event === "decision") {
                      const decision = JSON.parse(msg.data) as DecisionOutput;
                      setRuns((xs) => xs.map((x) => (x.id === tempId ? { ...x, decision } : x)));
                    } else if (msg.event === "risk_budget") {
                      const riskBudget = JSON.parse(msg.data) as RiskBudget;
                      setRuns((xs) => xs.map((x) => (x.id === tempId ? { ...x, riskBudget } : x)));
                    } else if (msg.event === "planned_orders") {
                      const plannedOrders = JSON.parse(msg.data) as PlannedOrder[];
                      setRuns((xs) => xs.map((x) => (x.id === tempId ? { ...x, plannedOrders } : x)));
                    } else if (msg.event === "guarded_orders") {
                      const guardedOrders = JSON.parse(msg.data) as GuardedOrder[];
                      setRuns((xs) => xs.map((x) => (x.id === tempId ? { ...x, guardedOrders } : x)));
                    } else if (msg.event === "signals") {
                      const signals = JSON.parse(msg.data) as TradeSignal[];
                      setRuns((xs) => xs.map((x) => (x.id === tempId ? { ...x, signals } : x)));
                    } else if (msg.event === "executions") {
                      const executions = JSON.parse(msg.data) as ExecutionResult[];
                      setRuns((xs) => xs.map((x) => (x.id === tempId ? { ...x, executions } : x)));
                    } else if (msg.event === "review") {
                      const review = JSON.parse(msg.data) as PostTradeReview;
                      setRuns((xs) => xs.map((x) => (x.id === tempId ? { ...x, review } : x)));
                    } else if (msg.event === "warnings") {
                      const warnings = JSON.parse(msg.data) as string[];
                      setRuns((xs) => xs.map((x) => (x.id === tempId ? { ...x, warnings } : x)));
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
              {run.riskBudget ? (
                <CollapsibleSection title="risk_budget">
                  <Markdown source={`\`\`\`json\n${JSON.stringify(run.riskBudget, null, 2)}\n\`\`\``} />
                </CollapsibleSection>
              ) : null}
              {run.plannedOrders && run.plannedOrders.length > 0 ? (
                <CollapsibleSection title="planned_orders">
                  <Markdown source={`\`\`\`json\n${JSON.stringify(run.plannedOrders, null, 2)}\n\`\`\``} />
                </CollapsibleSection>
              ) : null}
              {run.guardedOrders && run.guardedOrders.length > 0 ? (
                <CollapsibleSection title="guarded_orders">
                  <Markdown source={`\`\`\`json\n${JSON.stringify(run.guardedOrders, null, 2)}\n\`\`\``} />
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
              {run.review ? (
                <CollapsibleSection title="review">
                  <Markdown source={`\`\`\`json\n${JSON.stringify(run.review, null, 2)}\n\`\`\``} />
                </CollapsibleSection>
              ) : null}
              {run.warnings && run.warnings.length > 0 ? (
                <CollapsibleSection title="warnings">
                  <Markdown source={`\`\`\`json\n${JSON.stringify(run.warnings, null, 2)}\n\`\`\``} />
                </CollapsibleSection>
              ) : null}
            </AssistantCard>
          </div>
        ))}
      </div>

      <Modal open={guideModalOpen} onClose={() => setGuideModalOpen(false)} className="max-w-4xl">
        <div className="space-y-3">
          <div className="flex items-center justify-between">
            <div className="text-base font-semibold text-foreground">持仓买卖指南</div>
            <Button variant="ghost" onClick={() => setGuideModalOpen(false)}>
              关闭
            </Button>
          </div>

          {guideLoading ? <div className="text-sm text-muted-foreground">加载中...</div> : null}
          {!guideLoading && guideError ? <div className="text-sm text-destructive">{guideError}</div> : null}
          {!guideLoading && !guideError && guideRows.length === 0 ? (
            <div className="text-sm text-muted-foreground">当前账户暂无持仓</div>
          ) : null}

          {!guideLoading && !guideError && guideRows.length > 0 ? (
            <div className="max-h-[60vh] overflow-auto rounded-xl border border-border/60">
              <table className="w-full text-left text-sm">
                <thead className="bg-muted/30 text-xs text-muted-foreground">
                  <tr>
                    <th className="px-3 py-2">标的</th>
                    <th className="px-3 py-2">代码</th>
                    <th className="px-3 py-2">类型</th>
                    <th className="px-3 py-2">买卖指南</th>
                  </tr>
                </thead>
                <tbody>
                  {guideRows.map((row) => (
                    <tr key={row.symbol} className="border-t border-border/50">
                      <td className="px-3 py-2 text-foreground">{row.name}</td>
                      <td className="px-3 py-2 font-mono text-xs text-muted-foreground">{row.symbol}</td>
                      <td className="px-3 py-2 text-muted-foreground">{row.assetType}</td>
                      <td className="px-3 py-2">
                        <Select
                          value={guideSelections[row.symbol] ? String(guideSelections[row.symbol]) : ""}
                          onChange={(e) => {
                            const raw = e.target.value;
                            setGuideSelections((prev) => ({
                              ...prev,
                              [row.symbol]: raw ? Number(raw) : undefined,
                            }));
                          }}
                        >
                          <option value="">不选择（默认）</option>
                          {row.guides.map((guide) => (
                            <option key={guide.id} value={String(guide.id)}>
                              {buildGuideLabel(guide)}
                            </option>
                          ))}
                        </Select>
                        {row.guides.length === 0 ? (
                          <div className="mt-1 text-xs text-muted-foreground">暂无可选指南</div>
                        ) : null}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          ) : null}
        </div>
      </Modal>
    </div>
  );
}
