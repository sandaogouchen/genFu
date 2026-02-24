import { Link, useLocation } from "react-router-dom";
import { ExternalLink, Moon, PanelLeft, Sun } from "lucide-react";

import { cn } from "@/lib/utils";

const TITLES: Record<string, string> = {
  "/": "概览",
  "/analyze": "分析",
  "/decision": "决策",
  "/chat": "聊天",
  "/investment": "投资管理",
  "/market": "行情",
  "/news": "新闻/资讯",
  "/workflow": "工作流",
  "/docs": "文档/调试",
  "/reports": "报告库",
  "/stockpicker": "智能选股",
};

function toggleTheme() {
  const html = document.documentElement;
  const newIsDark = !html.classList.contains("dark");

  if (newIsDark) {
    html.classList.add("dark");
    localStorage.setItem("genfu.ui.theme", "dark");
  } else {
    html.classList.remove("dark");
    localStorage.setItem("genfu.ui.theme", "light");
  }
}

function isDark() {
  return document.documentElement.classList.contains("dark");
}

export default function TopBar({ onToggleSidebar }: { onToggleSidebar?: () => void }) {
  const { pathname } = useLocation();
  const title = TITLES[pathname] ?? "GenFu";

  return (
    <header className="sticky top-0 z-10 border-b border-border/50 bg-background/80 backdrop-blur-sm">
      <div className="mx-auto flex w-full max-w-[1100px] items-center justify-between gap-3 px-4 py-3 md:px-6">
        <div className="flex min-w-0 items-center gap-3">
          <button
            type="button"
            className="inline-flex h-9 w-9 items-center justify-center rounded-xl text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            aria-label="toggle-sidebar"
            onClick={() => onToggleSidebar?.()}
          >
            <PanelLeft className="h-5 w-5" />
          </button>
          <div className="min-w-0">
            <div className="truncate text-base font-semibold text-foreground">{title}</div>
          </div>
        </div>
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={toggleTheme}
            className="inline-flex h-9 w-9 items-center justify-center rounded-xl text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            aria-label={isDark() ? "Switch to light mode" : "Switch to dark mode"}
          >
            {isDark() ? <Sun className="h-5 w-5" /> : <Moon className="h-5 w-5" />}
          </button>
          <Link
            to="/docs"
            className={cn(
              "inline-flex items-center gap-2 rounded-xl px-3 py-2 text-sm font-medium",
              "text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            )}
          >
            <ExternalLink className="h-4 w-4" />
            <span className="hidden sm:inline">API 文档</span>
          </Link>
        </div>
      </div>
    </header>
  );
}
