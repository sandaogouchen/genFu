import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";

import Button from "@/components/ui/Button";
import { Card, CardBody } from "@/components/ui/Card";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import Skeleton from "@/components/ui/Skeleton";
import { toast } from "@/hooks/useToast";
import type { ReportListResponse } from "@/utils/genfuApi";
import { getJson } from "@/utils/genfuApi";
import { cn } from "@/lib/utils";

const REPORT_TYPES = [
  { value: "", label: "全部类型" },
  { value: "fund", label: "基金分析" },
  { value: "stock", label: "股票分析" },
  { value: "daily_review", label: "每日复盘" },
  { value: "next_open_guide", label: "次日开盘指导" },
];

export default function Reports() {
  const navigate = useNavigate();
  const [reportType, setReportType] = useState<string>("");
  const [search, setSearch] = useState<string>("");
  const [page, setPage] = useState<number>(1);
  const [data, setData] = useState<ReportListResponse | null>(null);
  const [loading, setLoading] = useState<boolean>(false);
  const [error, setError] = useState<string>("");

  const pageSize = 20;

  useEffect(() => {
    setLoading(true);
    setError("");
    const params = new URLSearchParams();
    if (reportType) params.append("type", reportType);
    if (search) params.append("search", search);
    params.append("page", page.toString());
    params.append("page_size", pageSize.toString());

    getJson<ReportListResponse>(`/api/reports?${params.toString()}`)
      .then(setData)
      .catch((err) => {
        const msg = err instanceof Error ? err.message : "unknown_error";
        setError(msg);
        toast({ title: "加载失败", description: msg, durationMs: 5200 });
      })
      .finally(() => setLoading(false));
  }, [reportType, search, page]);

  const totalPages = data ? Math.ceil(data.total / pageSize) : 0;

  return (
    <div className="space-y-5">
      {/* Page Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground">报告库</h1>
          <p className="text-sm text-muted-foreground mt-1">查看所有分析报告</p>
        </div>
      </div>

      {/* Filters */}
      <div className="flex flex-col gap-3 md:flex-row md:items-center">
        <div className="w-full md:w-48">
          <Select value={reportType} onChange={(e) => setReportType(e.target.value)}>
            {REPORT_TYPES.map((t) => (
              <option key={t.value} value={t.value}>
                {t.label}
              </option>
            ))}
          </Select>
        </div>
        <div className="flex-1 max-w-md">
          <Input
            value={search}
            onChange={(e) => setSearch(e.target.value)}
            placeholder="搜索报告标题或名称..."
          />
        </div>
      </div>

      {/* Loading state */}
      {loading && !data ? (
        <div className="space-y-3">
          {[...Array(5)].map((_, i) => (
            <Skeleton key={i} className="h-24 w-full" />
          ))}
        </div>
      ) : null}

      {/* Error state */}
      {error && !loading ? (
        <Card className="border-destructive/30 bg-destructive/5">
          <CardBody className="p-6 text-center">
            <div className="text-destructive">{error}</div>
          </CardBody>
        </Card>
      ) : null}

      {/* Report list */}
      {data && !loading ? (
        <div className="space-y-3">
          {data.items.length === 0 ? (
            <Card className="bg-muted/20">
              <CardBody className="p-12 text-center">
                <div className="text-muted-foreground text-base">暂无报告</div>
                <div className="text-sm text-muted-foreground mt-1">
                  {search || reportType ? "尝试调整筛选条件" : "还没有创建任何报告"}
                </div>
              </CardBody>
            </Card>
          ) : (
            <>
              {data.items.map((report) => (
                <Card
                  key={report.id}
                  className={cn(
                    "cursor-pointer transition-all duration-200",
                    "hover:border-accent/30 hover:shadow-md"
                  )}
                  onClick={() => navigate(`/reports/${report.id}`)}
                >
                  <CardBody className="p-5">
                    <div className="flex items-start justify-between gap-4">
                      <div className="flex-1 min-w-0">
                        <div className="flex items-center gap-2 mb-2">
                          <span className="px-2.5 py-0.5 rounded-lg text-xs font-medium bg-accent/10 text-accent border border-accent/20">
                            {report.report_type}
                          </span>
                          <span className="text-xs text-muted-foreground">
                            {report.symbol}
                          </span>
                        </div>
                        <div className="text-base font-medium text-foreground mb-1 truncate">
                          {report.title || report.name || "未命名报告"}
                        </div>
                        {report.title && report.name ? (
                          <div className="text-sm text-muted-foreground truncate">
                            {report.name}
                          </div>
                        ) : null}
                      </div>
                      <div className="text-xs text-muted-foreground shrink-0">
                        {new Date(report.created_at).toLocaleDateString()}
                      </div>
                    </div>
                  </CardBody>
                </Card>
              ))}

              {/* Pagination */}
              <div className="flex items-center justify-between pt-4">
                <div className="text-sm text-muted-foreground">
                  共 {data.total} 条报告
                </div>
                <div className="flex items-center gap-2">
                  <Button
                    size="sm"
                    variant="secondary"
                    disabled={page <= 1}
                    onClick={() => setPage((p) => p - 1)}
                  >
                    上一页
                  </Button>
                  <div className="text-sm text-muted-foreground px-2">
                    {page} / {totalPages}
                  </div>
                  <Button
                    size="sm"
                    variant="secondary"
                    disabled={page >= totalPages}
                    onClick={() => setPage((p) => p + 1)}
                  >
                    下一页
                  </Button>
                </div>
              </div>
            </>
          )}
        </div>
      ) : null}
    </div>
  );
}
