import { create } from "zustand";

type ChatSession = {
  id: string;
  title: string;
  updatedAt: number;
};

type State = {
  activeSessionId: string;
  sessions: ChatSession[];
  setActiveSessionId: (id: string) => void;
  newSession: () => string;
  touchSession: (id: string) => void;
  setSessionTitle: (id: string, title: string) => void;
};

function newSessionId() {
  return `sess_${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`;
}

const STORAGE_KEY = "genfu.chat.sessions.v1";

function loadSessions(): ChatSession[] {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return [];
    const parsed = JSON.parse(raw) as unknown;
    if (!Array.isArray(parsed)) return [];
    return parsed
      .map((x) => {
        const obj = x as Partial<ChatSession>;
        const id = String(obj.id ?? "").trim();
        if (!id) return null;
        return {
          id,
          title: String(obj.title ?? "").trim(),
          updatedAt: Number(obj.updatedAt ?? Date.now()),
        } satisfies ChatSession;
      })
      .filter(Boolean) as ChatSession[];
  } catch {
    return [];
  }
}

function persistSessions(sessions: ChatSession[]) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(sessions.slice(0, 50)));
  } catch {
    void 0;
  }
}

const initialSessions = typeof window !== "undefined" ? loadSessions() : [];
const initialActive = initialSessions[0]?.id ?? newSessionId();

export const useChatStore = create<State>((set, get) => ({
  activeSessionId: initialActive,
  sessions: initialSessions.length ? initialSessions : [{ id: initialActive, title: "", updatedAt: Date.now() }],
  setActiveSessionId: (id) => {
    const next = id.trim();
    if (!next) return;
    const exists = get().sessions.some((s) => s.id === next);
    const sessions = exists ? get().sessions : [{ id: next, title: "", updatedAt: Date.now() }, ...get().sessions];
    const normalized = sessions
      .map((s) => (s.id === next ? { ...s, updatedAt: Date.now() } : s))
      .sort((a, b) => b.updatedAt - a.updatedAt);
    persistSessions(normalized);
    set({ activeSessionId: next, sessions: normalized });
  },
  newSession: () => {
    const id = newSessionId();
    const sessions = [{ id, title: "", updatedAt: Date.now() }, ...get().sessions].sort((a, b) => b.updatedAt - a.updatedAt);
    persistSessions(sessions);
    set({ activeSessionId: id, sessions });
    return id;
  },
  touchSession: (id) => {
    const next = id.trim();
    if (!next) return;
    const sessions = get().sessions
      .map((s) => (s.id === next ? { ...s, updatedAt: Date.now() } : s))
      .sort((a, b) => b.updatedAt - a.updatedAt);
    persistSessions(sessions);
    set({ sessions });
  },
  setSessionTitle: (id, title) => {
    const next = id.trim();
    const t = title.trim();
    if (!next) return;
    const sessions = get().sessions
      .map((s) => (s.id === next ? { ...s, title: t || s.title, updatedAt: Date.now() } : s))
      .sort((a, b) => b.updatedAt - a.updatedAt);
    persistSessions(sessions);
    set({ sessions });
  },
}));

