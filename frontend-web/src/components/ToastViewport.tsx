import { X } from "lucide-react";

import { useToastStore } from "@/hooks/useToast";

export default function ToastViewport() {
  const toasts = useToastStore((s) => s.toasts);
  const dismiss = useToastStore((s) => s.dismiss);

  return (
    <div
      className="pointer-events-none fixed inset-x-0 bottom-4 z-50 mx-auto flex max-w-[560px] flex-col gap-2 px-4"
      role="region"
      aria-label="Notifications"
    >
      {toasts.map((t) => (
        <div
          key={t.id}
          className="pointer-events-auto flex items-start justify-between gap-3 rounded-xl border border-zinc-800 bg-zinc-950 px-4 py-3 text-zinc-50 shadow-lg"
          role="status"
          aria-live="polite"
        >
          <div className="min-w-0">
            <div className="truncate text-sm font-medium">{t.title}</div>
            {t.description ? <div className="mt-0.5 line-clamp-2 text-xs text-zinc-300">{t.description}</div> : null}
          </div>
          <button
            type="button"
            className="inline-flex h-7 w-7 items-center justify-center rounded-lg text-zinc-300 hover:bg-zinc-900 hover:text-zinc-50"
            onClick={() => dismiss(t.id)}
            aria-label="Close"
          >
            <X className="h-4 w-4" />
          </button>
        </div>
      ))}
    </div>
  );
}

