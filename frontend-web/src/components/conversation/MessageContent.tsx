import SectionedMarkdown from "@/components/conversation/SectionedMarkdown";

export default function MessageContent({ content }: { content: string }) {
  return <SectionedMarkdown source={content} />;
}
