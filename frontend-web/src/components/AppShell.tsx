import { ReactNode, useEffect, useMemo, useState } from "react";

import SidebarNav from "@/components/SidebarNav";
import TopBar from "@/components/TopBar";
import ToastViewport from "@/components/ToastViewport";
import { initTheme } from "@/components/ui/ThemeToggle";
import { cn } from "@/lib/utils";

export default function AppShell({ children }: { children: ReactNode }) {
  const initial = useMemo(() => {
    try {
      return localStorage.getItem("genfu.ui.sidebarCollapsed") === "1";
    } catch {
      return false;
    }
  }, []);
  const [collapsed, setCollapsed] = useState<boolean>(initial);

  useEffect(() => {
    initTheme();
  }, []);

  return (
    <div className="min-h-dvh bg-background text-foreground">
      {/* Floating Sidebar */}
      <aside
        className={cn(
          "fixed left-4 top-4 bottom-4 z-20 hidden shrink-0 rounded-2xl border border-border/50 bg-card/95 shadow-lg backdrop-blur-sm",
          "transition-all duration-300 ease-in-out md:block",
          collapsed ? "w-[72px]" : "w-[260px]"
        )}
      >
        <SidebarNav collapsed={collapsed} />
      </aside>

      {/* Main content area */}
      <div
        className={cn(
          "flex min-h-dvh min-w-0 flex-1 flex-col transition-all duration-300 ease-in-out",
          collapsed ? "md:ml-[88px]" : "md:ml-[276px]"
        )}
      >
        <TopBar
          onToggleSidebar={() => {
            setCollapsed((v) => {
              const next = !v;
              try {
                localStorage.setItem("genfu.ui.sidebarCollapsed", next ? "1" : "0");
              } catch {
                void 0;
              }
              return next;
            });
          }}
        />
        <main className="min-w-0 flex-1 p-4 md:p-6">
          <div className="mx-auto w-full max-w-[1100px]">{children}</div>
        </main>
      </div>
      <ToastViewport />
    </div>
  );
}
