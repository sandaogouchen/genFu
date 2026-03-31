import { ReactNode, useEffect, useMemo, useState } from "react";
import { X } from "lucide-react";
import { useLocation } from "react-router-dom";

import SidebarNav from "@/components/SidebarNav";
import TopBar from "@/components/TopBar";
import ToastViewport from "@/components/ToastViewport";
import { initTheme } from "@/components/ui/ThemeToggle";
import { cn } from "@/lib/utils";

export default function AppShell({ children }: { children: ReactNode }) {
  const { pathname } = useLocation();
  const initial = useMemo(() => {
    try {
      return localStorage.getItem("genfu.ui.sidebarCollapsed") === "1";
    } catch {
      return false;
    }
  }, []);
  const [collapsed, setCollapsed] = useState<boolean>(initial);
  const [mobileNavOpen, setMobileNavOpen] = useState(false);

  useEffect(() => {
    initTheme();
  }, []);

  useEffect(() => {
    setMobileNavOpen(false);
  }, [pathname]);

  useEffect(() => {
    if (!mobileNavOpen) return;
    const onKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setMobileNavOpen(false);
      }
    };
    const previousOverflow = document.body.style.overflow;
    document.body.style.overflow = "hidden";
    window.addEventListener("keydown", onKeyDown);
    return () => {
      document.body.style.overflow = previousOverflow;
      window.removeEventListener("keydown", onKeyDown);
    };
  }, [mobileNavOpen]);

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

      {/* Mobile Drawer */}
      {mobileNavOpen ? (
        <div className="fixed inset-0 z-40 md:hidden">
          <button
            type="button"
            className="absolute inset-0 bg-background/70 backdrop-blur-sm"
            aria-label="close-mobile-nav"
            onClick={() => setMobileNavOpen(false)}
          />
          <aside className="relative flex h-full w-[280px] max-w-[85vw] flex-col border-r border-border/70 bg-card shadow-xl">
            <div className="flex justify-end p-3">
              <button
                type="button"
                className="inline-flex h-9 w-9 items-center justify-center rounded-xl text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
                aria-label="close-mobile-nav-button"
                onClick={() => setMobileNavOpen(false)}
              >
                <X className="h-5 w-5" />
              </button>
            </div>
            <div className="min-h-0 flex-1">
              <SidebarNav />
            </div>
          </aside>
        </div>
      ) : null}

      {/* Main content area */}
      <div
        className={cn(
          "flex min-h-dvh min-w-0 flex-1 flex-col transition-all duration-300 ease-in-out",
          collapsed ? "md:ml-[88px]" : "md:ml-[276px]"
        )}
      >
        <TopBar
          onOpenMobileNav={() => {
            setMobileNavOpen(true);
          }}
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
