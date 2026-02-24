import { ReactNode } from "react";
import { Copy, Share2 } from "lucide-react";

import { cn } from "@/lib/utils";

export default function AssistantCard({
  badge,
  title,
  rightActions,
  children,
  className,
}: {
  badge?: string;
  title?: string;
  rightActions?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <div
      className={cn(
        "rounded-2xl border border-border/50 bg-card px-5 py-4 shadow-sm",
        className
      )}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="flex-1 min-w-0">
          {badge ? (
            <div className="inline-flex items-center rounded-lg bg-accent/10 px-2.5 py-1 text-xs font-medium text-accent">
              {badge}
            </div>
          ) : null}
          {title ? (
            <div className="mt-2 text-lg font-semibold tracking-tight text-foreground">{title}</div>
          ) : null}
        </div>
        <div className="flex items-center gap-1">
          {rightActions}
          <button
            type="button"
            className="inline-flex h-9 w-9 items-center justify-center rounded-xl text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            aria-label="copy"
            title="复制"
            onClick={() => void 0}
          >
            <Copy className="h-4 w-4" />
          </button>
          <button
            type="button"
            className="inline-flex h-9 w-9 items-center justify-center rounded-xl text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            aria-label="share"
            title="分享"
            onClick={() => void 0}
          >
            <Share2 className="h-4 w-4" />
          </button>
        </div>
      </div>
      <div className={cn(title ? "mt-4" : "", "space-y-4 text-foreground")}>{children}</div>
    </div>
  );
}
