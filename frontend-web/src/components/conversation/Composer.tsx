import { useMemo, useState } from "react";
import { ArrowUp, Paperclip, Plus, X } from "lucide-react";

import { cn } from "@/lib/utils";
import TemplateChips, { PromptTemplate } from "@/components/conversation/TemplateChips";

export default function Composer({
  value,
  onChange,
  onSubmit,
  disabled,
  placeholder,
  templates,
  className,
}: {
  value: string;
  onChange: (v: string) => void;
  onSubmit: () => void;
  disabled?: boolean;
  placeholder?: string;
  templates?: PromptTemplate[];
  className?: string;
}) {
  const canSend = useMemo(() => value.trim().length > 0 && !disabled, [value, disabled]);
  const [openTemplates, setOpenTemplates] = useState(false);

  return (
    <div className={cn("rounded-2xl border border-border/50 bg-card p-4 shadow-sm", className)}>
      <textarea
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={placeholder ?? "输入问题... (Shift + Enter 换行)"}
        className={cn(
          "w-full resize-none rounded-xl bg-muted/30 px-4 py-3 text-sm leading-6 text-foreground outline-none",
          "placeholder:text-muted-foreground",
          "focus-visible:ring-2 focus-visible:ring-accent/50"
        )}
        rows={3}
        onKeyDown={(e) => {
          if (e.key === "Enter" && !e.shiftKey) {
            e.preventDefault();
            if (canSend) onSubmit();
          }
        }}
      />
      {templates?.length && openTemplates ? (
        <div className="mt-3 rounded-xl border border-border/50 bg-muted/20 p-4">
          <div className="mb-3 flex items-center justify-between gap-2">
            <div className="text-xs font-medium text-foreground">模板提示词</div>
            <button
              type="button"
              className="inline-flex h-8 w-8 items-center justify-center rounded-lg text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
              aria-label="close-templates"
              onClick={() => setOpenTemplates(false)}
            >
              <X className="h-4 w-4" />
            </button>
          </div>
          <TemplateChips
            templates={templates}
            onPick={(v) => {
              onChange(v);
              setOpenTemplates(false);
            }}
          />
        </div>
      ) : null}

      <div className="mt-3 flex items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <button
            type="button"
            className="inline-flex h-9 w-9 items-center justify-center rounded-xl text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            aria-label="actions"
            onClick={() => {
              if (templates?.length) {
                setOpenTemplates((v) => !v);
              }
            }}
          >
            <Plus className="h-5 w-5" />
          </button>
          <button
            type="button"
            className="inline-flex h-9 w-9 items-center justify-center rounded-xl text-muted-foreground transition-colors hover:bg-muted hover:text-foreground"
            aria-label="attach"
            onClick={() => void 0}
          >
            <Paperclip className="h-5 w-5" />
          </button>
        </div>
        <button
          type="button"
          className={cn(
            "inline-flex h-10 w-10 items-center justify-center rounded-xl transition-all duration-200",
            canSend
              ? "bg-accent text-accent-foreground shadow-sm hover:bg-accent/90 hover:shadow-md"
              : "bg-muted text-muted-foreground"
          )}
          aria-label="send"
          onClick={() => {
            if (canSend) onSubmit();
          }}
          disabled={!canSend}
        >
          <ArrowUp className="h-5 w-5" />
        </button>
      </div>
    </div>
  );
}
