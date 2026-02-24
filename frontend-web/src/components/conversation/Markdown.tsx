import { Fragment, ReactNode, useMemo } from "react";

import { cn } from "@/lib/utils";
import CodeBlock from "@/components/conversation/CodeBlock";

type Block =
  | { kind: "code"; lang?: string; code: string }
  | { kind: "heading"; level: 1 | 2 | 3; text: string }
  | { kind: "ul"; items: string[] }
  | { kind: "ol"; items: string[] }
  | { kind: "quote"; lines: string[] }
  | { kind: "hr" }
  | { kind: "p"; text: string };

type InlineToken =
  | { kind: "text"; text: string }
  | { kind: "code"; text: string }
  | { kind: "bold"; text: string }
  | { kind: "italic"; text: string }
  | { kind: "link"; text: string; href: string };

function safeHref(href: string): string | null {
  const v = href.trim();
  if (!v) return null;
  if (v.startsWith("http://") || v.startsWith("https://") || v.startsWith("mailto:")) return v;
  return null;
}

function parseInline(text: string): InlineToken[] {
  const out: InlineToken[] = [];
  let i = 0;
  while (i < text.length) {
    const rest = text.slice(i);

    if (rest.startsWith("`")) {
      const end = rest.indexOf("`", 1);
      if (end > 0) {
        out.push({ kind: "code", text: rest.slice(1, end) });
        i += end + 1;
        continue;
      }
    }

    if (rest.startsWith("**")) {
      const end = rest.indexOf("**", 2);
      if (end > 1) {
        out.push({ kind: "bold", text: rest.slice(2, end) });
        i += end + 2;
        continue;
      }
    }

    if (rest.startsWith("*")) {
      const end = rest.indexOf("*", 1);
      if (end > 0) {
        out.push({ kind: "italic", text: rest.slice(1, end) });
        i += end + 1;
        continue;
      }
    }

    if (rest.startsWith("[")) {
      const mid = rest.indexOf("](");
      if (mid > 0) {
        const end = rest.indexOf(")", mid + 2);
        if (end > mid) {
          const label = rest.slice(1, mid);
          const href = rest.slice(mid + 2, end);
          out.push({ kind: "link", text: label, href });
          i += end + 1;
          continue;
        }
      }
    }

    const nextSpecial = (() => {
      const candidates = ["`", "*", "["].map((ch) => {
        const idx = rest.indexOf(ch, 1);
        return idx < 0 ? Number.POSITIVE_INFINITY : idx;
      });
      const min = Math.min(...candidates);
      return min === Number.POSITIVE_INFINITY ? -1 : min;
    })();

    if (nextSpecial === -1) {
      out.push({ kind: "text", text: rest });
      break;
    }
    out.push({ kind: "text", text: rest.slice(0, nextSpecial) });
    i += nextSpecial;
  }
  return out;
}

function splitFencedBlocks(input: string): Array<{ kind: "text"; text: string } | { kind: "code"; lang?: string; code: string }> {
  const parts: Array<{ kind: "text"; text: string } | { kind: "code"; lang?: string; code: string }> = [];
  const re = /```([a-zA-Z0-9_-]+)?\n([\s\S]*?)```/g;
  let last = 0;
  for (;;) {
    const m = re.exec(input);
    if (!m) break;
    if (m.index > last) parts.push({ kind: "text", text: input.slice(last, m.index) });
    parts.push({ kind: "code", lang: (m[1] ?? "").trim() || undefined, code: (m[2] ?? "").replace(/\n$/, "") });
    last = m.index + m[0].length;
  }
  if (last < input.length) parts.push({ kind: "text", text: input.slice(last) });
  return parts;
}

function parseTextBlocks(input: string): Block[] {
  const lines = input.replace(/\r\n/g, "\n").split("\n");
  const blocks: Block[] = [];
  let i = 0;

  const flushParagraph = (buf: string[]) => {
    const text = buf.join("\n").trim();
    if (text) blocks.push({ kind: "p", text });
    buf.length = 0;
  };

  const paragraphBuf: string[] = [];
  while (i < lines.length) {
    const line = lines[i] ?? "";
    const raw = line.trimEnd();

    if (!raw.trim()) {
      flushParagraph(paragraphBuf);
      i += 1;
      continue;
    }

    if (/^---+$/.test(raw.trim())) {
      flushParagraph(paragraphBuf);
      blocks.push({ kind: "hr" });
      i += 1;
      continue;
    }

    const h = raw.match(/^(#{1,3})\s+(.*)$/);
    if (h) {
      flushParagraph(paragraphBuf);
      const level = Math.min(3, h[1].length) as 1 | 2 | 3;
      blocks.push({ kind: "heading", level, text: (h[2] ?? "").trim() });
      i += 1;
      continue;
    }

    if (raw.trimStart().startsWith(">")) {
      flushParagraph(paragraphBuf);
      const quoteLines: string[] = [];
      while (i < lines.length) {
        const l = (lines[i] ?? "").trimEnd();
        if (!l.trimStart().startsWith(">")) break;
        quoteLines.push(l.replace(/^\s*>\s?/, ""));
        i += 1;
      }
      blocks.push({ kind: "quote", lines: quoteLines });
      continue;
    }

    const ul = raw.match(/^\s*[-*+]\s+(.*)$/);
    if (ul) {
      flushParagraph(paragraphBuf);
      const items: string[] = [];
      while (i < lines.length) {
        const m = (lines[i] ?? "").match(/^\s*[-*+]\s+(.*)$/);
        if (!m) break;
        items.push((m[1] ?? "").trim());
        i += 1;
      }
      blocks.push({ kind: "ul", items });
      continue;
    }

    const ol = raw.match(/^\s*\d+\.\s+(.*)$/);
    if (ol) {
      flushParagraph(paragraphBuf);
      const items: string[] = [];
      while (i < lines.length) {
        const m = (lines[i] ?? "").match(/^\s*\d+\.\s+(.*)$/);
        if (!m) break;
        items.push((m[1] ?? "").trim());
        i += 1;
      }
      blocks.push({ kind: "ol", items });
      continue;
    }

    paragraphBuf.push(raw);
    i += 1;
  }
  flushParagraph(paragraphBuf);
  return blocks;
}

function renderInline(tokens: InlineToken[]): ReactNode {
  return tokens.map((t, idx) => {
    if (t.kind === "text") return <Fragment key={idx}>{t.text}</Fragment>;
    if (t.kind === "code") {
      return (
        <code key={idx} className="rounded-md border border-border bg-muted px-1.5 py-0.5 font-mono text-[0.9em] text-foreground">
          {t.text}
        </code>
      );
    }
    if (t.kind === "bold") return <strong key={idx} className="font-semibold text-foreground">{t.text}</strong>;
    if (t.kind === "italic") return <em key={idx} className="italic">{t.text}</em>;
    if (t.kind === "link") {
      const href = safeHref(t.href);
      if (!href) return <Fragment key={idx}>{t.text}</Fragment>;
      return (
        <a key={idx} href={href} target="_blank" rel="noreferrer" className="text-accent underline underline-offset-4 hover:text-accent/80 transition-colors">
          {t.text}
        </a>
      );
    }
    return null;
  });
}

export default function Markdown({ source, className }: { source: string; className?: string }) {
  const blocks = useMemo(() => {
    const parts = splitFencedBlocks(source ?? "");
    const out: Block[] = [];
    for (const p of parts) {
      if (p.kind === "code") {
        out.push({ kind: "code", lang: p.lang, code: p.code });
      } else {
        out.push(...parseTextBlocks(p.text));
      }
    }
    return out;
  }, [source]);

  return (
    <div className={cn("space-y-3", className)}>
      {blocks.map((b, idx) => {
        if (b.kind === "code") return <CodeBlock key={idx} language={b.lang} code={b.code} />;
        if (b.kind === "hr") return <div key={idx} className="h-px w-full bg-border" />;
        if (b.kind === "heading") {
          const cls = b.level === 1 ? "text-xl" : b.level === 2 ? "text-lg" : "text-base";
          return (
            <div key={idx} className={cn("font-semibold tracking-tight text-card-foreground", cls)}>
              {renderInline(parseInline(b.text))}
            </div>
          );
        }
        if (b.kind === "quote") {
          return (
            <div key={idx} className="rounded-xl border border-border bg-muted/50 px-4 py-3 text-sm leading-6 text-foreground">
              {b.lines.map((l, j) => (
                <div key={j}>{renderInline(parseInline(l))}</div>
              ))}
            </div>
          );
        }
        if (b.kind === "ul") {
          return (
            <ul key={idx} className="list-disc space-y-1 pl-5 text-sm leading-6 text-foreground">
              {b.items.map((it, j) => (
                <li key={j}>{renderInline(parseInline(it))}</li>
              ))}
            </ul>
          );
        }
        if (b.kind === "ol") {
          return (
            <ol key={idx} className="list-decimal space-y-1 pl-5 text-sm leading-6 text-foreground">
              {b.items.map((it, j) => (
                <li key={j}>{renderInline(parseInline(it))}</li>
              ))}
            </ol>
          );
        }
        return (
          <div key={idx} className="whitespace-pre-wrap text-sm leading-6 text-foreground">
            {renderInline(parseInline(b.text))}
          </div>
        );
      })}
    </div>
  );
}
