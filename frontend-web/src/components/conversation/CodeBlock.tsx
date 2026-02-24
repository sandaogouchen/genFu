import { useMemo, useState } from "react";
import { Check, Copy } from "lucide-react";

import { cn } from "@/lib/utils";

export default function CodeBlock({ language, code, className }: { language?: string; code: string; className?: string }) {
  const [copied, setCopied] = useState(false);

  const label = useMemo(() => {
    const v = (language ?? "").trim();
    if (!v) return "code";
    return v;
  }, [language]);

  return (
    <div className={cn("overflow-hidden rounded-xl border border-border bg-muted/50", className)}>
      <div className="flex items-center justify-between gap-3 border-b border-border bg-card px-3 py-2">
        <div className="text-[11px] font-medium text-muted-foreground">{label}</div>
        <button
          type="button"
          className="inline-flex h-7 w-7 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
          onClick={async () => {
            try {
              await navigator.clipboard.writeText(code);
              setCopied(true);
              window.setTimeout(() => setCopied(false), 1200);
            } catch (e) {
              void e;
            }
          }}
          aria-label="copy"
          title={copied ? "已复制" : "复制"}
        >
          {copied ? <Check className="h-4 w-4" /> : <Copy className="h-4 w-4" />}
        </button>
      </div>
      <pre className="overflow-auto px-4 py-3 text-xs leading-5 text-foreground">
        <code>{code}</code>
      </pre>
    </div>
  );
}
