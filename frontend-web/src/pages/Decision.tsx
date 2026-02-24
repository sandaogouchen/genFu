import { useMemo, useRef, useState } from "react";

import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Skeleton from "@/components/ui/Skeleton";
import { Card, CardBody } from "@/components/ui/Card";
import { toast } from "@/hooks/useToast";
import type { DecisionOutput, ExecutionResult, TradeSignal } from "@/utils/genfuApi";
import { postSSE } from "@/utils/genfuApi";
import { cn } from "@/lib/utils";

export default function Decision() {
  const abortRef = useRef<AbortController | null>(null);
  const [reportIds, setReportIds] = useState<string>("");
  const [loading, setLoading] = useState(false);
  const [decision, setDecision] = useState<DecisionOutput | null>(null);
  const [signals, setSignals] = useState<TradeSignal[]>([]);
  const [executions, setExecutions] = useState<ExecutionResult[]>([]);
  const [error, setError] = useState<string>("");

  const canSubmit = useMemo(() => !loading, [loading]);

  return (
    <div className="flex flex-col h-full -m-4 md:-m-6">
      {/* Top action bar */}
      <div className="border-b border-border/50 bg-card/50 px-4 py-4 md:px-6">
        <div className="flex flex-col gap-3 md:flex-row md:items-center md:gap-4">
          <div className="flex-1">
            <div className="text-base font-semibold text-foreground">投资决策系统</div>
            <div className="text-sm text-muted-foreground mt-0.5">输入报告ID（可选），点击开始决策</div>
          </div>
          <div className="flex flex-col gap-2 md:flex-row md:items-center md:gap-3">
            <Input
              value={reportIds}
              onChange={(e) => setReportIds(e.target.value)}
              placeholder="report_ids（可选，逗号分隔）"
              className="w-full md:w-64"
            />
            <div className="flex gap-2">
              <Button
                disabled={!canSubmit}
                onClick={async () => {
                  abortRef.current?.abort();
                  const ac = new AbortController();
                  abortRef.current = ac;

                  setError("");
                  setDecision(null);
                  setSignals([]);
                  setExecutions([]);
                  setLoading(true);

                  const req = {
                    report_ids: reportIds
                      .split(",")
                      .map((x) => x.trim())
                      .filter(Boolean)
                      .map((x) => Number(x))
                      .filter((x) => Number.isFinite(x)),
                  };

                  try {
                    for await (const msg of postSSE("/sse/decision", req, { signal: ac.signal })) {
                      if (msg.event === "decision") {
                        setDecision(JSON.parse(msg.data) as DecisionOutput);
                      } else if (msg.event === "signals") {
                        setSignals(JSON.parse(msg.data) as TradeSignal[]);
                      } else if (msg.event === "executions") {
                        setExecutions(JSON.parse(msg.data) as ExecutionResult[]);
                      } else if (msg.event === "error") {
                        const payload = JSON.parse(msg.data) as { error?: string };
                        throw new Error(payload.error ?? "error");
                      }
                    }
                    toast({ title: "决策完成", description: "已输出交易信号与执行建议" });
                  } catch (e) {
                    const msg = e instanceof Error ? e.message : "unknown_error";
                    setError(msg);
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
                  toast({ title: "已取消", description: "已中止本次决策请求" });
                }}
              >
                取消
              </Button>
            </div>
          </div>
        </div>
        {error ? <div className="mt-2 text-sm text-destructive">{error}</div> : null}
      </div>

      {/* Main content area */}
      <div className="flex-1 overflow-auto p-4 md:p-6">
        {loading && !decision && signals.length === 0 && executions.length === 0 ? (
          <div className="space-y-6">
            <Skeleton className="h-32 w-full" />
            <Skeleton className="h-48 w-full" />
            <Skeleton className="h-32 w-full" />
          </div>
        ) : (
          <div className="space-y-6">
            {/* Decision section */}
            {decision ? (
              <Card className="overflow-hidden">
                <div className="bg-accent/5 px-5 py-4 border-b border-border/50">
                  <div className="flex items-center justify-between">
                    <div>
                      <div className="text-sm font-semibold text-foreground">市场观点</div>
                      <div className="text-xs text-muted-foreground mt-0.5">Decision ID: {decision.decision_id || "-"}</div>
                    </div>
                  </div>
                </div>
                <CardBody className="p-5">
                  <div className="text-sm leading-relaxed text-foreground">{decision.market_view || "(无市场观点)"}</div>
                  {decision.risk_notes ? (
                    <div className="mt-4 pt-4 border-t border-border/50">
                      <div className="text-xs font-medium text-amber-500 mb-2">风险提示</div>
                      <div className="whitespace-pre-wrap text-sm text-muted-foreground leading-relaxed">{decision.risk_notes}</div>
                    </div>
                  ) : null}
                </CardBody>
              </Card>
            ) : null}

            {/* Signals section */}
            {signals.length > 0 ? (
              <Card className="overflow-hidden">
                <div className="bg-accent/5 px-5 py-4 border-b border-border/50">
                  <div className="text-sm font-semibold text-foreground">交易信号 ({signals.length})</div>
                </div>
                <CardBody className="p-5">
                  <div className="grid gap-4">
                    {signals.map((s, idx) => {
                      const actionConfig = {
                        buy: {
                          bg: "bg-emerald-500/10",
                          text: "text-emerald-600 dark:text-emerald-400",
                          border: "border-emerald-500/20",
                        },
                        sell: {
                          bg: "bg-destructive/10",
                          text: "text-destructive",
                          border: "border-destructive/20",
                        },
                        hold: {
                          bg: "bg-accent/10",
                          text: "text-accent",
                          border: "border-accent/20",
                        },
                      };
                      const config = actionConfig[s.action as keyof typeof actionConfig] || actionConfig.hold;

                      return (
                        <div key={idx} className="rounded-xl border border-border/50 bg-muted/20 p-5">
                          <div className="flex items-start justify-between mb-3">
                            <div className="flex items-center gap-3">
                              <div className={cn("px-2.5 py-1 rounded-lg text-xs font-semibold border", config.bg, config.text, config.border)}>
                                {s.action?.toUpperCase() || "-"}
                              </div>
                              <div>
                                <div className="text-base font-medium text-foreground">
                                  {s.symbol || ""} {s.name ? `· ${s.name}` : ""}
                                </div>
                                <div className="text-xs text-muted-foreground mt-0.5">
                                  {s.asset_type || "unknown"} · Account #{s.account_id || "-"}
                                </div>
                              </div>
                            </div>
                            {s.confidence !== undefined && s.confidence !== null ? (
                              <div className="text-right">
                                <div className="text-xs text-muted-foreground">置信度</div>
                                <div className="text-sm font-semibold text-foreground">{(s.confidence * 100).toFixed(0)}%</div>
                              </div>
                            ) : null}
                          </div>

                          <div className="grid grid-cols-3 gap-4 mb-3">
                            {s.quantity !== undefined && s.quantity > 0 ? (
                              <div>
                                <div className="text-xs text-muted-foreground">数量</div>
                                <div className="text-sm font-medium text-foreground mt-0.5">{s.quantity}</div>
                              </div>
                            ) : null}
                            {s.price !== undefined && s.price > 0 ? (
                              <div>
                                <div className="text-xs text-muted-foreground">价格</div>
                                <div className="text-sm font-medium text-foreground mt-0.5">¥{s.price.toFixed(2)}</div>
                              </div>
                            ) : null}
                            {s.valid_until ? (
                              <div>
                                <div className="text-xs text-muted-foreground">有效期至</div>
                                <div className="text-sm font-medium text-foreground mt-0.5">{new Date(s.valid_until).toLocaleString()}</div>
                              </div>
                            ) : null}
                          </div>

                          {s.reason ? (
                            <div className="mt-3 pt-3 border-t border-border/50">
                              <div className="whitespace-pre-wrap text-sm text-muted-foreground leading-relaxed">{s.reason}</div>
                            </div>
                          ) : null}
                        </div>
                      );
                    })}
                  </div>
                </CardBody>
              </Card>
            ) : null}

            {/* Executions section */}
            {executions.length > 0 ? (
              <Card className="overflow-hidden">
                <div className="bg-emerald-500/5 px-5 py-4 border-b border-border/50">
                  <div className="text-sm font-semibold text-foreground">执行结果 ({executions.length})</div>
                </div>
                <CardBody className="p-5">
                  <div className="grid gap-4">
                    {executions.map((x, idx) => {
                      const statusConfig = {
                        executed: {
                          bg: "bg-emerald-500/5",
                          border: "border-emerald-500/30",
                          badge: {
                            bg: "bg-emerald-500/10",
                            text: "text-emerald-600 dark:text-emerald-400",
                            border: "border-emerald-500/20",
                          },
                        },
                        skipped: {
                          bg: "bg-accent/5",
                          border: "border-accent/30",
                          badge: {
                            bg: "bg-accent/10",
                            text: "text-accent",
                            border: "border-accent/20",
                          },
                        },
                        failed: {
                          bg: "bg-destructive/5",
                          border: "border-destructive/30",
                          badge: {
                            bg: "bg-destructive/10",
                            text: "text-destructive",
                            border: "border-destructive/20",
                          },
                        },
                      };

                      const config = statusConfig[x.status as keyof typeof statusConfig] || statusConfig.failed;

                      return (
                        <div key={idx} className={cn("rounded-xl border p-5", config.bg, config.border)}>
                          <div className="flex items-start justify-between mb-3">
                            <div className="flex items-center gap-3">
                              <div className={cn("px-2.5 py-1 rounded-lg text-xs font-semibold border", config.badge.bg, config.badge.text, config.badge.border)}>
                                {x.status?.toUpperCase() || "UNKNOWN"}
                              </div>
                              <div>
                                <div className="text-base font-medium text-foreground">
                                  {x.signal.symbol || ""} {x.signal.name ? `· ${x.signal.name}` : ""}
                                </div>
                                <div className="text-xs text-muted-foreground mt-0.5">
                                  {x.signal.action?.toUpperCase()} · {x.signal.asset_type || "unknown"}
                                </div>
                              </div>
                            </div>
                          </div>

                          {/* Successful execution trade info */}
                          {x.status === "executed" && x.trade && x.position ? (
                            <div className="grid grid-cols-2 md:grid-cols-4 gap-4 mb-3">
                              <div>
                                <div className="text-xs text-muted-foreground">交易ID</div>
                                <div className="text-sm font-medium text-foreground mt-0.5">#{x.trade.id}</div>
                              </div>
                              <div>
                                <div className="text-xs text-muted-foreground">交易价格</div>
                                <div className="text-sm font-medium text-foreground mt-0.5">¥{x.trade.price.toFixed(2)}</div>
                              </div>
                              <div>
                                <div className="text-xs text-muted-foreground">持仓数量</div>
                                <div className="text-sm font-medium text-foreground mt-0.5">{x.position.quantity}</div>
                              </div>
                              <div>
                                <div className="text-xs text-muted-foreground">当前市值</div>
                                <div className="text-sm font-medium text-foreground mt-0.5">¥{(x.position.quantity * x.position.average_cost).toFixed(2)}</div>
                              </div>
                            </div>
                          ) : null}

                          {/* Skipped execution note */}
                          {x.status === "skipped" ? (
                            <div className="text-sm text-muted-foreground italic">该信号为持有建议，未执行交易操作</div>
                          ) : null}

                          {/* Error info */}
                          {x.error ? (
                            <div className="mt-3 pt-3 border-t border-destructive/30">
                              <div className="text-xs font-medium text-destructive mb-1">错误信息</div>
                              <div className="text-sm text-destructive">{x.error}</div>
                            </div>
                          ) : null}

                          {/* Trade reason */}
                          {x.signal.reason ? (
                            <div className="mt-3 pt-3 border-t border-border/50">
                              <div className="text-xs font-medium text-muted-foreground mb-1">交易理由</div>
                              <div className="whitespace-pre-wrap text-sm text-muted-foreground leading-relaxed">{x.signal.reason}</div>
                            </div>
                          ) : null}
                        </div>
                      );
                    })}
                  </div>
                </CardBody>
              </Card>
            ) : null}

            {!loading && !decision && signals.length === 0 && executions.length === 0 ? (
              <div className="flex flex-col items-center justify-center py-20 text-center">
                <div className="text-muted-foreground text-base">等待决策生成</div>
                <div className="text-sm text-muted-foreground mt-1">输入报告ID后点击"开始决策"按钮</div>
              </div>
            ) : null}
          </div>
        )}
      </div>
    </div>
  );
}
