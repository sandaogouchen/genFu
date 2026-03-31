import { useCallback, useEffect, useMemo, useRef, useState } from "react";
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
import { toast } from "@/hooks/useToast";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Modal from "@/components/ui/Modal";
import {
  createConversationSession,
  deleteConversationSession,
  listConversationSessions,
  renameConversationSession,
  type ConversationScene,
  type ConversationSession,
} from "@/utils/genfuApi";
import { useChatStore } from "@/stores/chatStore";
import { usePageSessionStore } from "@/stores/pageSessionStore";

const NAV = [
  { to: "/", label: "概览", icon: Activity },
  { to: "/analyze", label: "分析", icon: BarChart3 },
  { to: "/decision", label: "决策", icon: Target },
  { to: "/stockpicker", label: "智能选股", icon: TrendingUp },
  { to: "/chat", label: "聊天", icon: MessageSquare },
  { to: "/investment", label: "投资管理", icon: BriefcaseBusiness },
  { to: "/market", label: "行情", icon: LineChart },
  { to: "/news", label: "新闻/资讯", icon: Newspaper },
  { to: "/workflow", label: "工作流", icon: Route },
  { to: "/docs", label: "文档/调试", icon: FileText },
];

const SCENE_BY_PATH: Record<string, ConversationScene> = {
  "/analyze": "analyze",
  "/decision": "decision",
  "/stockpicker": "stockpicker",
  "/workflow": "workflow",
};

const SCENE_LABEL: Record<ConversationScene, string> = {
  analyze: "分析日志",
  decision: "决策日志",
  stockpicker: "选股日志",
  workflow: "工作流日志",
};

function inferTitleFromScene(scene: ConversationScene, count: number) {
  return `${SCENE_LABEL[scene]} ${count + 1}`;
}

const SESSION_TITLE_MAX = 64;

type SessionDialogState =
  | {
      kind: "rename";
      sessionId: string;
      value: string;
      fallbackLabel: string;
      error: string;
    }
  | {
      kind: "delete";
      sessionId: string;
      title: string;
      error: string;
    }
  | null;

function validateSessionTitle(value: string) {
  const next = value.trim();
  if (!next) return "会话名称不能为空";
  if (next.length > SESSION_TITLE_MAX) return `会话名称不能超过 ${SESSION_TITLE_MAX} 个字符`;
  return "";
}

export default function SidebarNav({ collapsed }: { collapsed?: boolean }) {
  const { pathname } = useLocation();
  const navigate = useNavigate();
  const [pageSessions, setPageSessions] = useState<ConversationSession[]>([]);
  const [loadingPageSessions, setLoadingPageSessions] = useState(false);
  const [sessionDialog, setSessionDialog] = useState<SessionDialogState>(null);
  const [dialogSubmitting, setDialogSubmitting] = useState(false);
  const renameInputRef = useRef<HTMLInputElement>(null);

  const scene = SCENE_BY_PATH[pathname] ?? null;
  const activeByScene = usePageSessionStore((s) => s.activeByScene);
  const setActive = usePageSessionStore((s) => s.setActive);
  const clearActive = usePageSessionStore((s) => s.clearActive);

  const sessions = useChatStore((s) => s.sessions);
  const activeSessionId = useChatStore((s) => s.activeSessionId);
  const setActiveSessionId = useChatStore((s) => s.setActiveSessionId);
  const newSession = useChatStore((s) => s.newSession);

  const inChat = pathname === "/chat";
  const activePageSessionId = useMemo(
    () => (scene ? activeByScene[scene] ?? "" : ""),
    [activeByScene, scene]
  );

  const reloadPageSessions = useCallback(
    async (targetScene: ConversationScene) => {
      setLoadingPageSessions(true);
      try {
        const data = await listConversationSessions(targetScene, 100, 0);
        const items = data.items ?? [];
        setPageSessions(items);
        const currentActive = usePageSessionStore.getState().activeByScene[targetScene];
        if (items.length > 0 && !currentActive) {
          setActive(targetScene, items[0].id);
        }
      } catch (err) {
        const msg = err instanceof Error ? err.message : "加载失败";
        toast({ title: "会话加载失败", description: msg, durationMs: 4200 });
        setPageSessions([]);
      } finally {
        setLoadingPageSessions(false);
      }
    },
    [setActive]
  );

  useEffect(() => {
    if (!scene) return;
    void reloadPageSessions(scene);
    const handler = () => {
      void reloadPageSessions(scene);
    };
    window.addEventListener("genfu:conversation-updated", handler);
    return () => {
      window.removeEventListener("genfu:conversation-updated", handler);
    };
  }, [reloadPageSessions, scene]);

  useEffect(() => {
    setSessionDialog(null);
    setDialogSubmitting(false);
  }, [scene]);

  const closeSessionDialog = useCallback(() => {
    if (dialogSubmitting) return;
    setSessionDialog(null);
  }, [dialogSubmitting]);

  const submitRenameDialog = async () => {
    if (!scene || !sessionDialog || sessionDialog.kind !== "rename") return;
    const validationError = validateSessionTitle(sessionDialog.value);
    if (validationError) {
      setSessionDialog((prev) =>
        prev && prev.kind === "rename" ? { ...prev, error: validationError } : prev
      );
      return;
    }

    setDialogSubmitting(true);
    try {
      await renameConversationSession(sessionDialog.sessionId, sessionDialog.value.trim());
      setSessionDialog(null);
      await reloadPageSessions(scene);
      window.dispatchEvent(new Event("genfu:conversation-updated"));
    } catch (err) {
      const msg = err instanceof Error ? err.message : "重命名失败";
      setSessionDialog((prev) => (prev && prev.kind === "rename" ? { ...prev, error: msg } : prev));
    } finally {
      setDialogSubmitting(false);
    }
  };

  const submitDeleteDialog = async () => {
    if (!scene || !sessionDialog || sessionDialog.kind !== "delete") return;
    setDialogSubmitting(true);
    try {
      await deleteConversationSession(sessionDialog.sessionId);
      if (activePageSessionId === sessionDialog.sessionId) {
        clearActive(scene);
      }
      setSessionDialog(null);
      await reloadPageSessions(scene);
      window.dispatchEvent(new Event("genfu:conversation-updated"));
    } catch (err) {
      const msg = err instanceof Error ? err.message : "删除失败";
      setSessionDialog((prev) => (prev && prev.kind === "delete" ? { ...prev, error: msg } : prev));
    } finally {
      setDialogSubmitting(false);
    }
  };

  return (
    <div className="flex h-full flex-col">
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
        ) : scene ? (
          <div className="mt-6 rounded-xl border border-border/50 bg-muted/20">
            <div className="flex items-center justify-between gap-2 border-b border-border/50 px-3 py-2.5">
              <div className="text-xs font-semibold text-foreground">{SCENE_LABEL[scene]}</div>
              <button
                type="button"
                className="rounded-lg bg-accent/10 px-2.5 py-1 text-xs font-medium text-accent transition-colors hover:bg-accent/20"
                onClick={async () => {
                  try {
                    const created = await createConversationSession({
                      scene,
                      title: inferTitleFromScene(scene, pageSessions.length),
                    });
                    setActive(scene, created.id);
                    await reloadPageSessions(scene);
                    window.dispatchEvent(new Event("genfu:conversation-updated"));
                  } catch (err) {
                    const msg = err instanceof Error ? err.message : "创建失败";
                    toast({ title: "新建会话失败", description: msg, durationMs: 4200 });
                  }
                }}
              >
                新建
              </button>
            </div>
            <div className="max-h-[40vh] overflow-auto p-2">
              {loadingPageSessions ? (
                <div className="px-2 py-2 text-xs text-muted-foreground">加载中...</div>
              ) : (
                <div className="space-y-1">
                  {pageSessions.map((s) => {
                    const active = s.id === activePageSessionId;
                    const rawTitle = s.title ?? "";
                    const label = rawTitle.trim() ? rawTitle : "未命名会话";
                    return (
                      <div key={s.id} className="rounded-lg px-1 py-1 hover:bg-muted/40">
                        <button
                          type="button"
                          className={cn(
                            "w-full rounded-md px-2 py-1.5 text-left text-sm transition-colors duration-200 cursor-pointer",
                            active
                              ? "bg-accent/10 text-accent"
                              : "text-foreground/70 hover:bg-muted/50 hover:text-foreground"
                          )}
                          onClick={() => {
                            setActive(scene, s.id);
                            window.dispatchEvent(new Event("genfu:conversation-updated"));
                          }}
                          title={s.id}
                        >
                          <div className="truncate">{label}</div>
                        </button>
                        <div className="mt-1 flex items-center gap-2 px-2">
                          <button
                            type="button"
                            className="text-[11px] text-muted-foreground hover:text-foreground"
                            onClick={() => {
                              setSessionDialog({
                                kind: "rename",
                                sessionId: s.id,
                                value: rawTitle,
                                fallbackLabel: label,
                                error: "",
                              });
                            }}
                          >
                            重命名
                          </button>
                          <button
                            type="button"
                            className="text-[11px] text-destructive/80 hover:text-destructive"
                            onClick={() => {
                              setSessionDialog({
                                kind: "delete",
                                sessionId: s.id,
                                title: label,
                                error: "",
                              });
                            }}
                          >
                            删除
                          </button>
                        </div>
                      </div>
                    );
                  })}
                  {!loadingPageSessions && pageSessions.length === 0 ? (
                    <div className="px-2 py-2 text-xs text-muted-foreground">暂无会话</div>
                  ) : null}
                </div>
              )}
            </div>
          </div>
        ) : null}
      </nav>

      <Modal
        open={sessionDialog !== null}
        onClose={closeSessionDialog}
        initialFocusRef={sessionDialog?.kind === "rename" ? renameInputRef : undefined}
        closeOnBackdropClick={!dialogSubmitting}
      >
        {sessionDialog?.kind === "rename" ? (
          <form
            className="space-y-4"
            onSubmit={(event) => {
              event.preventDefault();
              void submitRenameDialog();
            }}
          >
            <div>
              <div className="text-sm font-semibold text-foreground">重命名会话</div>
              <div className="mt-1 text-xs text-muted-foreground">更新后将用于后续会话记录展示。</div>
            </div>
            <div className="space-y-2">
              <Input
                ref={renameInputRef}
                value={sessionDialog.value}
                placeholder={sessionDialog.fallbackLabel}
                maxLength={SESSION_TITLE_MAX}
                disabled={dialogSubmitting}
                aria-invalid={Boolean(sessionDialog.error)}
                aria-describedby={sessionDialog.error ? "session-rename-error" : undefined}
                onChange={(event) => {
                  const value = event.target.value;
                  setSessionDialog((prev) =>
                    prev && prev.kind === "rename" ? { ...prev, value, error: "" } : prev
                  );
                }}
              />
              <div className="text-right text-[11px] text-muted-foreground">
                {sessionDialog.value.trim().length}/{SESSION_TITLE_MAX}
              </div>
              {sessionDialog.error ? (
                <div id="session-rename-error" className="text-xs text-destructive">
                  {sessionDialog.error}
                </div>
              ) : null}
            </div>
            <div className="flex justify-end gap-2">
              <Button type="button" size="sm" variant="secondary" onClick={closeSessionDialog} disabled={dialogSubmitting}>
                取消
              </Button>
              <Button type="submit" size="sm" disabled={dialogSubmitting}>
                {dialogSubmitting ? "保存中..." : "保存"}
              </Button>
            </div>
          </form>
        ) : sessionDialog?.kind === "delete" ? (
          <form
            className="space-y-4"
            onSubmit={(event) => {
              event.preventDefault();
              void submitDeleteDialog();
            }}
          >
            <div>
              <div className="text-sm font-semibold text-foreground">删除会话</div>
              <div className="mt-1 text-xs text-muted-foreground">
                确认删除「{sessionDialog.title}」吗？该操作不可撤销。
              </div>
            </div>
            {sessionDialog.error ? <div className="text-xs text-destructive">{sessionDialog.error}</div> : null}
            <div className="flex justify-end gap-2">
              <Button type="button" size="sm" variant="secondary" onClick={closeSessionDialog} disabled={dialogSubmitting}>
                取消
              </Button>
              <Button type="submit" size="sm" variant="destructive" disabled={dialogSubmitting}>
                {dialogSubmitting ? "删除中..." : "确认删除"}
              </Button>
            </div>
          </form>
        ) : null}
      </Modal>

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
