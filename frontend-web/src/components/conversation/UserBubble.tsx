import { cn } from "@/lib/utils";

export default function UserBubble({ content, className }: { content: string; className?: string }) {
  return (
    <div className={cn("flex justify-end", className)}>
      <div className="w-[min(640px,90%)] rounded-2xl bg-accent px-5 py-3.5 text-sm leading-6 text-accent-foreground shadow-sm">
        {content}
      </div>
    </div>
  );
}
