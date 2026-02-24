import { useEffect, useMemo, useRef, useState } from "react";

import AssistantCard from "@/components/conversation/AssistantCard";
import Composer from "@/components/conversation/Composer";
import MessageContent from "@/components/conversation/MessageContent";
import type { PromptTemplate } from "@/components/conversation/TemplateChips";
import TipCard from "@/components/conversation/TipCard";
import UserBubble from "@/components/conversation/UserBubble";
import Skeleton from "@/components/ui/Skeleton";
import { toast } from "@/hooks/useToast";
import type { ChatMessage, GenerateEvent, GenerateRequest } from "@/utils/genfuApi";
import { getJson, postSSE } from "@/utils/genfuApi";
import { useChatStore } from "@/stores/chatStore";

export default function Chat() {
  const sseAbortRef = useRef<AbortController | null>(null);
  const sessionId = useChatStore((s) => s.activeSessionId);
  const setActiveSessionId = useChatStore((s) => s.setActiveSessionId);
  const touchSession = useChatStore((s) => s.touchSession);
  const setSessionTitle = useChatStore((s) => s.setSessionTitle);
  const [input, setInput] = useState<string>("");
  const [sending, setSending] = useState(false);

  const [messages, setMessages] = useState<ChatMessage[]>([]);
  const [streaming, setStreaming] = useState<string>("");
  const [lastError, setLastError] = useState<string>("");

  const templates = useMemo<PromptTemplate[]>(
    () => [
      { label: "给我一个命令", value: "设定 pm2 5 号进程每半小时重启一次的命令" },
      { label: "排查 400", value: "请帮我排查模型 400 报错的常见原因，并给出优先级最高的检查项" },
      { label: "写一段脚本", value: "写一个 bash 脚本：把当前目录所有 .log 压缩并按日期归档" },
    ],
    []
  );

  const canSend = useMemo(() => input.trim().length > 0 && !sending, [input, sending]);

  useEffect(() => {
    return () => {
      sseAbortRef.current?.abort();
    };
  }, []);

  useEffect(() => {
    if (!sessionId.trim()) return;
    let cancelled = false;
    const run = async () => {
      setLastError("");
      try {
        const qs = new URLSearchParams({ session_id: sessionId.trim(), limit: "50" });
        const data = await getJson<ChatMessage[]>(`/api/chat/history?${qs.toString()}`);
        if (cancelled) return;
        setMessages(data);
        setStreaming("");
      } catch (e) {
        if (cancelled) return;
        const msg = e instanceof Error ? e.message : "unknown_error";
        setLastError(msg);
      }
    };
    void run();
    return () => {
      cancelled = true;
    };
  }, [sessionId]);

  const handleEvent = (evt: GenerateEvent) => {
    if (evt.type === "session" && evt.delta) {
      setActiveSessionId(evt.delta);
      return;
    }
    if (evt.type === "delta") {
      setStreaming((s) => s + (evt.delta ?? ""));
      return;
    }
    if (evt.type === "message" && evt.message) {
      setMessages((m) => [...m, evt.message as ChatMessage]);
      setStreaming("");
      return;
    }
    if (evt.type === "error") {
      const msg = evt.delta || "error";
      setLastError(msg);
      toast({ title: "生成失败", description: msg, durationMs: 5200 });
      return;
    }
    if (evt.type === "done") {
      return;
    }
  };

  const cancel = () => {
    sseAbortRef.current?.abort();
    setSending(false);
    toast({ title: "已取消", description: "已中止本次生成" });
  };

  const send = async () => {
    if (!canSend) return;

    const content = input.trim();
    setInput("");
    setLastError("");

    const nextMessages: ChatMessage[] = [...messages, { role: "user", content }];
    setMessages(nextMessages);
    setStreaming("");
    setSending(true);

    touchSession(sessionId);
    const existingTitle = useChatStore.getState().sessions.find((s) => s.id === sessionId)?.title ?? "";
    if (!existingTitle.trim()) {
      setSessionTitle(sessionId, content.slice(0, 22));
    }

    const req: GenerateRequest = {
      session_id: sessionId,
      messages: nextMessages,
    };

    try {
      sseAbortRef.current?.abort();
      const ac = new AbortController();
      sseAbortRef.current = ac;

      let gotAny = false;
      for await (const msg of postSSE("/api/chat", req, { signal: ac.signal })) {
        gotAny = true;
        const evt = JSON.parse(msg.data) as GenerateEvent;
        handleEvent(evt);
      }
      if (!gotAny) {
        toast({ title: "无响应", description: "未收到任何 SSE 事件" });
      }
    } catch (e) {
      const msg = e instanceof Error ? e.message : "unknown_error";
      setLastError(msg);
      toast({ title: "发送失败", description: msg, durationMs: 5200 });
      setSending(false);
    }
  };

  return (
    <div className="space-y-5">
      <TipCard title="提示：像普通 AI 聊天一样输入问题即可" dismissKey="chat">
        默认 SSE 流式输出；<span className="font-mono text-accent">Enter</span> 发送，<span className="font-mono text-accent">Shift + Enter</span> 换行。
      </TipCard>

      <div className="rounded-2xl border border-border/50 bg-card p-5">
        <div className="max-h-[560px] overflow-auto space-y-5">
          {messages.map((m, idx) => {
            if (m.role === "user") return <UserBubble key={idx} content={m.content} />;
            if (m.role === "assistant") {
              return (
                <AssistantCard key={idx} badge="Cloud-O-4.6">
                  <MessageContent content={m.content} />
                </AssistantCard>
              );
            }
            return (
              <AssistantCard key={idx} badge="Cloud-O-4.6">
                <MessageContent content={m.content} />
              </AssistantCard>
            );
          })}

          {sending && streaming.length === 0 ? (
            <AssistantCard badge="Cloud-O-4.6">
              <div className="space-y-3">
                <Skeleton className="h-4 w-40" />
                <Skeleton className="h-4 w-72" />
                <Skeleton className="h-4 w-56" />
              </div>
            </AssistantCard>
          ) : null}

          {streaming ? (
            <AssistantCard badge="Cloud-O-4.6">
              <MessageContent content={streaming} />
            </AssistantCard>
          ) : null}
        </div>
      </div>

      {lastError ? <div className="text-sm text-destructive">{lastError}</div> : null}
      <Composer value={input} onChange={setInput} disabled={sending} onSubmit={() => void send()} templates={templates} />
    </div>
  );
}
