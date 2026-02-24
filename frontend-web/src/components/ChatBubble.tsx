import type { ChatMessage } from "@/utils/genfuApi";

export default function ChatBubble({ role, content, dim }: { role: ChatMessage["role"]; content: string; dim?: boolean }) {
  const isUser = role === "user";
  return (
    <div className={isUser ? "flex justify-end" : "flex justify-start"}>
      <div
        className={
          isUser
            ? "w-[min(520px,85%)] rounded-2xl bg-zinc-900 px-3 py-2 text-sm text-zinc-50"
            : `w-[min(520px,85%)] rounded-2xl border border-zinc-200 bg-white px-3 py-2 text-sm text-zinc-900 ${
                dim ? "opacity-80" : ""
              }`
        }
      >
        <div className="whitespace-pre-wrap break-words">{content}</div>
      </div>
    </div>
  );
}

