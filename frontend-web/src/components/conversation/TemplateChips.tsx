import { cn } from "@/lib/utils";

export type PromptTemplate = { label: string; value: string };

export default function TemplateChips({
  templates,
  onPick,
  className,
}: {
  templates: PromptTemplate[];
  onPick: (value: string) => void;
  className?: string;
}) {
  if (!templates.length) return null;
  return (
    <div className={cn("flex flex-wrap gap-2", className)}>
      {templates.map((t) => (
        <button
          key={t.label}
          type="button"
          className="rounded-full border border-border bg-background px-3 py-1.5 text-xs font-medium text-foreground transition-colors hover:bg-accent hover:text-accent-foreground"
          onClick={() => onPick(t.value)}
          title={t.value}
        >
          {t.label}
        </button>
      ))}
    </div>
  );
}
