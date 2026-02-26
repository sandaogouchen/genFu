import { create } from "zustand";

import type { ConversationScene } from "@/utils/genfuApi";

type ActiveByScene = Partial<Record<ConversationScene, string>>;

type State = {
  activeByScene: ActiveByScene;
  setActive: (scene: ConversationScene, sessionId: string) => void;
  clearActive: (scene: ConversationScene) => void;
};

const STORAGE_KEY = "genfu.page.sessions.v1";

function loadActiveByScene(): ActiveByScene {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return {};
    const parsed = JSON.parse(raw) as unknown;
    if (!parsed || typeof parsed !== "object") return {};
    const obj = parsed as Record<string, unknown>;
    const next: ActiveByScene = {};
    const scenes: ConversationScene[] = ["analyze", "decision", "stockpicker", "workflow"];
    for (const scene of scenes) {
      const v = obj[scene];
      if (typeof v === "string" && v.trim()) {
        next[scene] = v.trim();
      }
    }
    return next;
  } catch {
    return {};
  }
}

function persistActiveByScene(activeByScene: ActiveByScene) {
  try {
    localStorage.setItem(STORAGE_KEY, JSON.stringify(activeByScene));
  } catch {
    void 0;
  }
}

const initialActiveByScene = typeof window !== "undefined" ? loadActiveByScene() : {};

export const usePageSessionStore = create<State>((set, get) => ({
  activeByScene: initialActiveByScene,
  setActive: (scene, sessionId) => {
    const id = sessionId.trim();
    if (!id) return;
    const next = { ...get().activeByScene, [scene]: id };
    persistActiveByScene(next);
    set({ activeByScene: next });
  },
  clearActive: (scene) => {
    const next = { ...get().activeByScene };
    delete next[scene];
    persistActiveByScene(next);
    set({ activeByScene: next });
  },
}));
