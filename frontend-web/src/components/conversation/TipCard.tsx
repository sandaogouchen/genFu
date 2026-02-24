import { ReactNode, useMemo, useState } from "react";
import { X } from "lucide-react";

import { cn } from "@/lib/utils";

export default function TipCard({
  title,
  children,
  className,
  dismissKey,
}: {
  title: string;
  children: ReactNode;
  className?: string;
  dismissKey?: string;
}) {
  const dismissed = useMemo(() => {
    if (!dismissKey) return false;
    try {
      return localStorage.getItem(`genfu.tip.dismissed.${dismissKey}`) === "1";
    } catch {
      return false;
    }
  }, [dismissKey]);
  const [open, setOpen] = useState(!dismissed);

  if (!open) return null;

  return (
    <div
      className={cn(
        "relative rounded-2xl border border-accent/20 bg-accent/5 px-5 py-4 text-sm leading-6",
        className
      )}
    >
      <div className="flex items-start justify-between gap-3">
        <div className="font-semibold text-foreground">{title}</div>
        {dismissKey ? (
          <button
            type="button"
            className="-mr-1 -mt-1 inline-flex h-8 w-8 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            aria-label="close"
            onClick={() => {
              try {
                localStorage.setItem(`genfu.tip.dismissed.${dismissKey}`, "1");
              } catch {
                void 0;
              }
              setOpen(false);
            }}
          >
            <X className="h-4 w-4" />
          </button>
        ) : null}
      </div>
      <div className="mt-2 text-muted-foreground">{children}</div>
    </div>
  );
}
