import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { ArrowLeft } from "lucide-react";

import Button from "@/components/ui/Button";
import { Card, CardBody } from "@/components/ui/Card";
import Skeleton from "@/components/ui/Skeleton";
import CollapsibleSection from "@/components/conversation/CollapsibleSection";
import Markdown from "@/components/conversation/Markdown";
import { toast } from "@/hooks/useToast";
import type { ReportDetail } from "@/utils/genfuApi";
import { getJson } from "@/utils/genfuApi";

const REPORT_TYPE_LABELS: Record<string, string> = {
  fund: "基金分析",
  stock: "股票分析",
  daily_review: "每日复盘",
  next_open_guide: "次日开盘指导",
};

export default function ReportDetail() {
  const { id } = useParams<{ id: string }>();
  const navigate = useNavigate();
  const [report, setReport] = useState<ReportDetail | null>(null);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>("");

  useEffect(() => {
    if (!id) return;

    setLoading(true);
    setError("");
    getJson<ReportDetail>(`/api/reports/${id}`)
      .then(setReport)
      .catch((err) => {
        const msg = err instanceof Error ? err.message : "unknown_error";
        setError(msg);
        toast({ title: "加载失败", description: msg, durationMs: 5200 });
      })
      .finally(() => setLoading(false));
  }, [id]);

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center gap-3">
        <Button
          variant="ghost"
          size="sm"
          onClick={() => navigate("/reports")}
          className="flex items-center gap-2"
        >
          <ArrowLeft className="h-4 w-4" />
          返回列表
        </Button>
      </div>

      {/* Loading state */}
      {loading ? (
        <div className="space-y-4">
          <Skeleton className="h-32 w-full" />
          <Skeleton className="h-64 w-full" />
        </div>
      ) : null}

      {/* Error state */}
      {error && !loading ? (
        <div className="rounded-xl border border-destructive/50 bg-destructive/10 p-6 text-center">
          <div className="text-destructive">{error}</div>
          <Button
            className="mt-4"
            onClick={() => navigate("/reports")}
          >
            返回报告列表
          </Button>
        </div>
      ) : null}

      {/* Report detail */}
      {report && !loading ? (
        <>
          {/* Basic info card */}
          <Card>
            <CardBody className="p-6">
              <div className="flex items-start justify-between mb-4">
                <div className="flex items-center gap-2">
                  <span className="px-2.5 py-1 rounded-md text-xs font-semibold bg-accent/10 text-accent border border-accent/20">
                    {REPORT_TYPE_LABELS[report.report_type] || report.report_type}
                  </span>
                  <span className="text-sm font-mono text-muted-foreground">
                    {report.symbol}
                  </span>
                </div>
                <div className="text-xs text-muted-foreground">
                  {new Date(report.created_at).toLocaleString()}
                </div>
              </div>

              <h1 className="text-2xl font-semibold text-foreground mb-2">
                {report.title || report.name || "未命名报告"}
              </h1>

              {report.title && report.name ? (
                <div className="text-base text-muted-foreground mb-4">
                  {report.name}
                </div>
              ) : null}

              <div className="text-sm text-muted-foreground">
                报告ID: <span className="font-mono text-foreground">{report.id}</span>
              </div>
            </CardBody>
          </Card>

          {/* Analysis steps */}
          {report.steps && report.steps.length > 0 ? (
            <CollapsibleSection title="分析步骤" className="mb-4">
              <div className="space-y-3">
                {report.steps.map((step, idx) => (
                  <CollapsibleSection
                    key={idx}
                    title={(step.name || `step_${idx + 1}`).toLowerCase()}
                  >
                    {step.output ? (
                      <Markdown source={step.output} />
                    ) : (
                      <div className="text-sm text-muted-foreground">无输出内容</div>
                    )}
                  </CollapsibleSection>
                ))}
              </div>
            </CollapsibleSection>
          ) : null}

          {/* Summary */}
          {report.summary ? (
            <Card>
              <CardBody className="p-6">
                <h2 className="text-lg font-semibold text-foreground mb-4">总结</h2>
                <Markdown source={report.summary} />
              </CardBody>
            </Card>
          ) : null}

          {/* Request details */}
          <CollapsibleSection title="原始请求">
            <div className="text-sm font-mono text-muted-foreground whitespace-pre-wrap">
              {JSON.stringify(report.request, null, 2)}
            </div>
          </CollapsibleSection>
        </>
      ) : null}
    </div>
  );
}
