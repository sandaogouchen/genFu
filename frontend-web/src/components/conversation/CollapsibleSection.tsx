import { ReactNode, useState } from "react";
import { ChevronDown } from "lucide-react";

import { cn } from "@/lib/utils";

export default function CollapsibleSection({
  title,
  defaultOpen,
  children,
  className,
}: {
  title: string;
  defaultOpen?: boolean;
  children: ReactNode;
  className?: string;
}) {
  const [open, setOpen] = useState(Boolean(defaultOpen));
  return (
    <div className={cn("rounded-xl border border-border/50 bg-muted/20", className)}>
      <button
        type="button"
        className="flex w-full items-center justify-between gap-3 px-4 py-3 text-left transition-colors hover:bg-muted/30 rounded-t-xl"
        onClick={() => setOpen((v) => !v)}
      >
        <div className="text-sm font-medium text-foreground">{title}</div>
        <ChevronDown
          className={cn("h-4 w-4 text-muted-foreground transition-transform duration-200", open ? "rotate-180" : "rotate-0")}
        />
      </button>
      {open ? <div className="border-t border-border/50 px-4 py-3">{children}</div> : null}
    </div>
  );
}
