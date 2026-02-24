import { create } from "zustand";

type ToastItem = {
  id: string;
  title: string;
  description?: string;
};

type ToastInput = {
  title: string;
  description?: string;
  durationMs?: number;
};

type ToastState = {
  toasts: ToastItem[];
  push: (t: ToastInput) => string;
  dismiss: (id: string) => void;
  clear: () => void;
};

const timers = new Map<string, number>();

function uid() {
  return `${Date.now().toString(36)}_${Math.random().toString(36).slice(2, 8)}`;
}

export const useToastStore = create<ToastState>((set, get) => ({
  toasts: [],
  push: (t) => {
    const id = uid();
    const item: ToastItem = { id, title: t.title, description: t.description };
    const duration = Math.max(800, Math.min(12000, t.durationMs ?? 3200));
    set((s) => ({ toasts: [item, ...s.toasts].slice(0, 4) }));
    const handle = window.setTimeout(() => {
      timers.delete(id);
      get().dismiss(id);
    }, duration);
    timers.set(id, handle);
    return id;
  },
  dismiss: (id) => {
    const handle = timers.get(id);
    if (handle != null) {
      window.clearTimeout(handle);
      timers.delete(id);
    }
    set((s) => ({ toasts: s.toasts.filter((t) => t.id !== id) }));
  },
  clear: () => {
    for (const handle of timers.values()) {
      window.clearTimeout(handle);
    }
    timers.clear();
    set({ toasts: [] });
  },
}));

export function toast(input: ToastInput) {
  return useToastStore.getState().push(input);
}

