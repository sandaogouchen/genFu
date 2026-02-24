import { useMemo } from "react";

import CollapsibleSection from "@/components/conversation/CollapsibleSection";
import Markdown from "@/components/conversation/Markdown";

type Section = { title: string; body: string };

function normalizeTitle(t: string) {
  return t.trim().toLowerCase();
}

function isSummaryTitle(title: string) {
  const t = normalizeTitle(title);
  return t.startsWith("summary") || t.startsWith("总结") || t === "总览";
}

function splitByHeadings(md: string): Section[] {
  const lines = md.replace(/\r\n/g, "\n").split("\n");
  const sections: Section[] = [];
  let current: Section | null = null;
  for (const line of lines) {
    const m = line.match(/^(#{1,3})\s+(.*)$/);
    if (m) {
      if (current) sections.push({ ...current, body: current.body.trim() });
      current = { title: (m[2] ?? "").trim(), body: "" };
      continue;
    }
    if (!current) current = { title: "summary", body: "" };
    current.body += line + "\n";
  }
  if (current) sections.push({ ...current, body: current.body.trim() });
  return sections.filter((s) => s.body.trim().length > 0);
}

function splitByMarkers(md: string): Section[] {
  const lines = md.replace(/\r\n/g, "\n").split("\n");
  const sections: Section[] = [];
  let current: Section | null = null;
  const markerRe = /^(summary|bull|bear|debate|manager|kline)\b\s*[:：-]?\s*(.*)$/i;
  for (const line of lines) {
    const m = line.trim().match(markerRe);
    if (m) {
      if (current) sections.push({ ...current, body: current.body.trim() });
      current = { title: m[1] ?? "section", body: (m[2] ?? "").trim() };
      if (current.body) current.body += "\n";
      continue;
    }
    if (!current) current = { title: "summary", body: "" };
    current.body += line + "\n";
  }
  if (current) sections.push({ ...current, body: current.body.trim() });
  return sections.filter((s) => s.body.trim().length > 0);
}

function buildSections(md: string): Section[] {
  const hasHeading = /^(#{1,3})\s+/m.test(md);
  if (hasHeading) return splitByHeadings(md);
  const hasMarker = /^(summary|bull|bear|debate|manager|kline)\b/m.test(md.trim().toLowerCase());
  if (hasMarker) return splitByMarkers(md);
  return [{ title: "summary", body: md }];
}

export default function SectionedMarkdown({ source }: { source: string }) {
  const sections = useMemo(() => buildSections(source ?? ""), [source]);
  if (!sections.length) return null;

  if (sections.length === 1) {
    return <Markdown source={sections[0]!.body} />;
  }

  return (
    <div className="space-y-3">
      {sections.map((s, idx) => (
        <CollapsibleSection key={idx} title={s.title} defaultOpen={isSummaryTitle(s.title)}>
          <Markdown source={s.body} />
        </CollapsibleSection>
      ))}
    </div>
  );
}

