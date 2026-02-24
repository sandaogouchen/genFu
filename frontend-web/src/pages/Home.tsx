import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router-dom";
import {
  Activity,
  ArrowRight,
  BarChart3,
  CheckCircle2,
  FileText,
  MessageSquare,
  Target,
  TrendingUp,
  XCircle,
  Zap,
} from "lucide-react";

import { cn } from "@/lib/utils";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/Card";

type HealthState = "idle" | "ok" | "error";

// Stat Card Component
function StatCard({
  label,
  value,
  icon: Icon,
  trend,
  className,
}: {
  label: string;
  value: string | number;
  icon: React.ElementType;
  trend?: { value: number; label: string };
  className?: string;
}) {
  return (
    <Card className={cn("group cursor-pointer transition-all duration-200 hover:shadow-md", className)}>
      <CardBody className="p-5">
        <div className="flex items-start justify-between">
          <div className="flex-1">
            <div className="text-sm text-muted-foreground">{label}</div>
            <div className="mt-2 text-2xl font-bold tracking-tight text-foreground">{value}</div>
            {trend ? (
              <div
                className={cn(
                  "mt-2 flex items-center gap-1 text-xs font-medium",
                  trend.value >= 0 ? "text-emerald-500" : "text-destructive"
                )}
              >
                <TrendingUp className={cn("h-3.5 w-3.5", trend.value < 0 && "rotate-180")} />
                {trend.value >= 0 ? "+" : ""}
                {trend.value}% {trend.label}
              </div>
            ) : null}
          </div>
          <div className="grid h-11 w-11 place-items-center rounded-xl bg-accent/10 text-accent transition-colors group-hover:bg-accent group-hover:text-accent-foreground">
            <Icon className="h-5 w-5" />
          </div>
        </div>
      </CardBody>
    </Card>
  );
}

// Quick Action Card Component
function QuickActionCard({
  title,
  desc,
  to,
  icon: Icon,
  color,
}: {
  title: string;
  desc: string;
  to: string;
  icon: React.ElementType;
  color: "blue" | "green" | "purple" | "orange";
}) {
  const colorClasses = {
    blue: "bg-sky-500/10 text-sky-500 group-hover:bg-sky-500 group-hover:text-white",
    green: "bg-emerald-500/10 text-emerald-500 group-hover:bg-emerald-500 group-hover:text-white",
    purple: "bg-violet-500/10 text-violet-500 group-hover:bg-violet-500 group-hover:text-white",
    orange: "bg-amber-500/10 text-amber-500 group-hover:bg-amber-500 group-hover:text-white",
  };

  return (
    <Link to={to} className="block">
      <Card className="group cursor-pointer transition-all duration-200 hover:shadow-md hover:border-accent/30">
        <CardBody className="p-5">
          <div className="flex items-start gap-4">
            <div
              className={cn(
                "grid h-12 w-12 shrink-0 place-items-center rounded-xl transition-colors",
                colorClasses[color]
              )}
            >
              <Icon className="h-6 w-6" />
            </div>
            <div className="flex-1 min-w-0">
              <div className="text-base font-semibold text-foreground">{title}</div>
              <div className="mt-1 text-sm text-muted-foreground">{desc}</div>
            </div>
            <ArrowRight className="h-5 w-5 text-muted-foreground opacity-0 transition-opacity group-hover:opacity-100" />
          </div>
        </CardBody>
      </Card>
    </Link>
  );
}

// Activity Item Component
function ActivityItem({
  type,
  title,
  time,
  status,
}: {
  type: "analyze" | "decision" | "chat" | "report";
  title: string;
  time: string;
  status: "success" | "pending" | "error";
}) {
  const typeConfig = {
    analyze: { icon: BarChart3, label: "分析" },
    decision: { icon: Target, label: "决策" },
    chat: { icon: MessageSquare, label: "聊天" },
    report: { icon: FileText, label: "报告" },
  };

  const statusConfig = {
    success: "text-emerald-500",
    pending: "text-amber-500",
    error: "text-destructive",
  };

  const config = typeConfig[type];
  const Icon = config.icon;

  return (
    <div className="flex items-center gap-3 py-3 transition-colors hover:bg-muted/30 rounded-lg px-2 -mx-2 cursor-pointer">
      <div className="grid h-9 w-9 shrink-0 place-items-center rounded-lg bg-muted/50 text-muted-foreground">
        <Icon className="h-4 w-4" />
      </div>
      <div className="flex-1 min-w-0">
        <div className="truncate text-sm font-medium text-foreground">{title}</div>
        <div className="flex items-center gap-2 mt-0.5">
          <span className="text-xs text-muted-foreground">{config.label}</span>
          <span className={cn("text-xs", statusConfig[status])}>
            {status === "success" ? "已完成" : status === "pending" ? "进行中" : "失败"}
          </span>
        </div>
      </div>
      <div className="text-xs text-muted-foreground">{time}</div>
    </div>
  );
}

export default function Home() {
  const [health, setHealth] = useState<HealthState>("idle");
  const [healthText, setHealthText] = useState<string>("");

  useEffect(() => {
    let canceled = false;
    (async () => {
      try {
        const resp = await fetch("/healthz");
        const text = await resp.text();
        if (canceled) return;
        if (resp.ok) {
          setHealth("ok");
          setHealthText(text || "ok");
        } else {
          setHealth("error");
          setHealthText(`HTTP ${resp.status}`);
        }
      } catch (e) {
        if (canceled) return;
        setHealth("error");
        setHealthText(e instanceof Error ? e.message : "network_error");
      }
    })();
    return () => {
      canceled = true;
    };
  }, []);

  const healthStatus = useMemo(() => {
    if (health === "idle") return { label: "检测中", color: "text-muted-foreground" };
    if (health === "ok") return { label: "正常", color: "text-emerald-500" };
    return { label: "异常", color: "text-destructive" };
  }, [health]);

  // Mock data for dashboard
  const stats = {
    reports: 128,
    signals: 47,
    sessions: 23,
  };

  const recentActivities = [
    { type: "analyze" as const, title: "贵州茅台 600519 分析报告", time: "5 分钟前", status: "success" as const },
    { type: "decision" as const, title: "交易信号生成 - 买入信号", time: "12 分钟前", status: "success" as const },
    { type: "chat" as const, title: "基金持仓分析对话", time: "1 小时前", status: "success" as const },
    { type: "report" as const, title: "每日复盘报告", time: "2 小时前", status: "success" as const },
    { type: "analyze" as const, title: "沪深300ETF 510300 分析", time: "3 小时前", status: "success" as const },
  ];

  return (
    <div className="space-y-6">
      {/* Page Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-bold tracking-tight text-foreground">概览</h1>
          <p className="text-sm text-muted-foreground mt-1">AI 驱动的投资分析与决策系统</p>
        </div>
        <div className="flex items-center gap-2">
          <div
            className={cn(
              "inline-flex items-center gap-2 rounded-full px-3 py-1.5 text-xs font-medium",
              health === "ok"
                ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
                : health === "error"
                  ? "bg-destructive/10 text-destructive"
                  : "bg-muted text-muted-foreground"
            )}
          >
            <Activity className="h-3.5 w-3.5" />
            后端服务: <span className={healthStatus.color}>{healthStatus.label}</span>
          </div>
        </div>
      </div>

      {/* Stats Row */}
      <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
        <StatCard label="分析报告" value={stats.reports} icon={FileText} />
        <StatCard label="交易信号" value={stats.signals} icon={Target} />
        <StatCard label="会话记录" value={stats.sessions} icon={MessageSquare} />
        <StatCard
          label="系统状态"
          value={health === "ok" ? "正常" : health === "error" ? "异常" : "检测中"}
          icon={health === "ok" ? CheckCircle2 : health === "error" ? XCircle : Activity}
          className={cn(
            health === "ok" && "border-emerald-500/20",
            health === "error" && "border-destructive/20"
          )}
        />
      </div>

      {/* Quick Actions */}
      <div>
        <h2 className="mb-4 text-lg font-semibold text-foreground">快捷操作</h2>
        <div className="grid grid-cols-1 gap-4 md:grid-cols-3">
          <QuickActionCard
            title="股票/基金分析"
            desc="SSE 流式分析，支持多维度评估"
            to="/analyze"
            icon={BarChart3}
            color="blue"
          />
          <QuickActionCard
            title="交易决策"
            desc="基于报告生成交易信号"
            to="/decision"
            icon={Target}
            color="green"
          />
          <QuickActionCard
            title="AI 助手"
            desc="智能对话，解答投资问题"
            to="/chat"
            icon={MessageSquare}
            color="purple"
          />
        </div>
      </div>

      {/* Recent Activity */}
      <Card>
        <CardHeader className="flex flex-row items-center justify-between border-b border-border/50 px-5 py-4">
          <CardTitle className="text-base">最近活动</CardTitle>
          <Link
            to="/reports"
            className="inline-flex items-center gap-1 text-sm text-accent hover:text-accent/80 transition-colors"
          >
            查看全部
            <ArrowRight className="h-4 w-4" />
          </Link>
        </CardHeader>
        <CardBody className="p-4">
          <div className="divide-y divide-border/50">
            {recentActivities.map((activity, idx) => (
              <ActivityItem key={idx} {...activity} />
            ))}
          </div>
        </CardBody>
      </Card>

      {/* Feature Grid */}
      <div className="grid grid-cols-2 gap-4 md:grid-cols-4">
        <Link
          to="/reports"
          className="flex flex-col items-center gap-2 rounded-xl border border-border/50 bg-card p-4 text-center transition-all hover:border-accent/30 hover:shadow-sm cursor-pointer"
        >
          <FileText className="h-6 w-6 text-muted-foreground" />
          <span className="text-sm font-medium text-foreground">报告库</span>
        </Link>
        <Link
          to="/stockpicker"
          className="flex flex-col items-center gap-2 rounded-xl border border-border/50 bg-card p-4 text-center transition-all hover:border-accent/30 hover:shadow-sm cursor-pointer"
        >
          <TrendingUp className="h-6 w-6 text-muted-foreground" />
          <span className="text-sm font-medium text-foreground">智能选股</span>
        </Link>
        <Link
          to="/market"
          className="flex flex-col items-center gap-2 rounded-xl border border-border/50 bg-card p-4 text-center transition-all hover:border-accent/30 hover:shadow-sm cursor-pointer"
        >
          <Activity className="h-6 w-6 text-muted-foreground" />
          <span className="text-sm font-medium text-foreground">行情数据</span>
        </Link>
        <Link
          to="/workflow"
          className="flex flex-col items-center gap-2 rounded-xl border border-border/50 bg-card p-4 text-center transition-all hover:border-accent/30 hover:shadow-sm cursor-pointer"
        >
          <Zap className="h-6 w-6 text-muted-foreground" />
          <span className="text-sm font-medium text-foreground">工作流</span>
        </Link>
      </div>
    </div>
  );
}
