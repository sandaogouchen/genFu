import { Link, useLocation, useNavigate } from "react-router-dom";
import {
  Activity,
  BarChart3,
  BriefcaseBusiness,
  FileText,
  LineChart,
  MessageSquare,
  Newspaper,
  Route,
  ShieldCheck,
  Target,
  TrendingUp,
} from "lucide-react";

import { cn } from "@/lib/utils";
import { useChatStore } from "@/stores/chatStore";

const NAV = [
  { to: "/", label: "概览", icon: Activity },
  { to: "/analyze", label: "分析", icon: BarChart3 },
  { to: "/reports", label: "报告库", icon: FileText },
  { to: "/decision", label: "决策", icon: Target },
  { to: "/stockpicker", label: "智能选股", icon: TrendingUp },
  { to: "/chat", label: "聊天", icon: MessageSquare },
  { to: "/investment", label: "投资管理", icon: BriefcaseBusiness },
  { to: "/market", label: "行情", icon: LineChart },
  { to: "/news", label: "新闻/资讯", icon: Newspaper },
  { to: "/workflow", label: "工作流", icon: Route },
  { to: "/docs", label: "文档/调试", icon: FileText },
];

export default function SidebarNav({ collapsed }: { collapsed?: boolean }) {
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const sessions = useChatStore((s) => s.sessions);
  const activeSessionId = useChatStore((s) => s.activeSessionId);
  const setActiveSessionId = useChatStore((s) => s.setActiveSessionId);
  const newSession = useChatStore((s) => s.newSession);

  const inChat = pathname === "/chat";

  return (
    <div className="flex h-full flex-col">
      {/* Logo/Brand */}
      <div className="p-4">
        <div
          className={cn(
            "flex items-center gap-3 rounded-xl bg-muted/30 px-3 py-3",
            collapsed ? "justify-center" : ""
          )}
        >
          <div className="grid h-9 w-9 shrink-0 place-items-center rounded-xl bg-accent text-accent-foreground">
            <ShieldCheck className="h-5 w-5" />
          </div>
          {!collapsed ? (
            <div className="min-w-0">
              <div className="truncate text-base font-semibold tracking-tight">GenFu</div>
              <div className="truncate text-xs text-muted-foreground">AI 投资分析</div>
            </div>
          ) : null}
        </div>
      </div>

      {/* Navigation */}
      <nav className="flex-1 overflow-auto px-3">
        <div className="space-y-1">
          {NAV.map((item) => {
            const active = pathname === item.to;
            const Icon = item.icon;
            return (
              <Link
                key={item.to}
                to={item.to}
                className={cn(
                  "group flex items-center gap-3 rounded-xl px-3 py-2.5 text-sm font-medium cursor-pointer",
                  "transition-all duration-200",
                  active
                    ? "bg-accent/10 text-accent"
                    : "text-foreground/70 hover:bg-muted/50 hover:text-foreground"
                )}
                title={collapsed ? item.label : undefined}
              >
                <Icon
                  className={cn(
                    "h-5 w-5 shrink-0 transition-colors duration-200",
                    active ? "text-accent" : "text-muted-foreground group-hover:text-foreground"
                  )}
                />
                {!collapsed ? <span className="truncate">{item.label}</span> : null}
              </Link>
            );
          })}
        </div>

        {/* Chat History */}
        {collapsed ? null : inChat ? (
          <div className="mt-6 rounded-xl border border-border/50 bg-muted/20">
            <div className="flex items-center justify-between gap-2 border-b border-border/50 px-3 py-2.5">
              <div className="text-xs font-semibold text-foreground">会话记录</div>
              <button
                type="button"
                className="rounded-lg bg-accent/10 px-2.5 py-1 text-xs font-medium text-accent transition-colors hover:bg-accent/20"
                onClick={() => {
                  const id = newSession();
                  navigate("/chat");
                  setActiveSessionId(id);
                }}
              >
                新建
              </button>
            </div>
            <div className="max-h-[40vh] overflow-auto p-2">
              <div className="space-y-1">
                {sessions.map((s) => {
                  const active = s.id === activeSessionId;
                  const label = s.title || "未命名会话";
                  return (
                    <button
                      key={s.id}
                      type="button"
                      className={cn(
                        "w-full rounded-lg px-3 py-2 text-left text-sm transition-colors duration-200 cursor-pointer",
                        active
                          ? "bg-accent/10 text-accent"
                          : "text-foreground/70 hover:bg-muted/50 hover:text-foreground"
                      )}
                      onClick={() => {
                        setActiveSessionId(s.id);
                        navigate("/chat");
                      }}
                      title={s.id}
                    >
                      <div className="truncate">{label}</div>
                      <div className="mt-0.5 truncate text-xs text-muted-foreground">{s.id}</div>
                    </button>
                  );
                })}
              </div>
            </div>
          </div>
        ) : null}
      </nav>

      {/* Footer */}
      <div className="border-t border-border/50 p-3">
        {!collapsed ? (
          <div className="text-xs text-muted-foreground">
            <div className="font-medium text-foreground/80">后端服务</div>
            <div className="mt-0.5">localhost:8080</div>
          </div>
        ) : null}
      </div>
    </div>
  );
}
