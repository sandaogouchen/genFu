import { useMemo, useRef, useState } from "react";

import Button from "@/components/ui/Button";
import Textarea from "@/components/ui/Textarea";
import { Card, CardBody, CardDescription, CardHeader, CardTitle } from "@/components/ui/Card";
import Skeleton from "@/components/ui/Skeleton";
import { toast } from "@/hooks/useToast";
import { postJson } from "@/utils/genfuApi";

export default function JsonPlayground({
  title,
  description,
  endpoint,
  defaultBody,
}: {
  title: string;
  description: string;
  endpoint: string;
  defaultBody: unknown;
}) {
  const abortRef = useRef<AbortController | null>(null);
  const [bodyText, setBodyText] = useState<string>(JSON.stringify(defaultBody, null, 2));
  const [loading, setLoading] = useState(false);
  const [resp, setResp] = useState<unknown>(null);
  const [error, setError] = useState<string>("");

  const canSubmit = useMemo(() => !loading, [loading]);

  return (
    <div className="grid gap-4 md:grid-cols-2">
      <Card>
        <CardHeader>
          <CardTitle>{title}</CardTitle>
          <CardDescription>{description}</CardDescription>
        </CardHeader>
        <CardBody>
          <div className="text-xs text-zinc-600">POST {endpoint}</div>
          <Textarea value={bodyText} onChange={(e) => setBodyText(e.target.value)} className="mt-2 font-mono text-xs" />
          <div className="mt-3 flex flex-wrap gap-2">
            <Button
              disabled={!canSubmit}
              onClick={async () => {
                abortRef.current?.abort();
                const ac = new AbortController();
                abortRef.current = ac;

                setError("");
                setResp(null);
                setLoading(true);
                try {
                  const parsed = JSON.parse(bodyText);
                  const data = await postJson<unknown>(endpoint, parsed, { signal: ac.signal });
                  setResp(data);
                  toast({ title: "请求完成", description: `${endpoint} 返回成功` });
                } catch (e) {
                  const msg = e instanceof Error ? e.message : "unknown_error";
                  setError(msg);
                  toast({ title: "请求失败", description: msg, durationMs: 5200 });
                } finally {
                  setLoading(false);
                }
              }}
            >
              {loading ? "请求中…" : "发送"}
            </Button>
            <Button
              variant="ghost"
              disabled={!loading}
              onClick={() => {
                abortRef.current?.abort();
                setLoading(false);
                toast({ title: "已取消", description: "已中止本次请求" });
              }}
            >
              取消
            </Button>
            <Button
              variant="secondary"
              onClick={() => {
                setBodyText(JSON.stringify(defaultBody, null, 2));
                toast({ title: "已重置", description: "请求体已恢复默认模板" });
              }}
            >
              重置模板
            </Button>
          </div>
          {error ? <div className="mt-2 text-xs text-rose-700">{error}</div> : null}
        </CardBody>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>响应</CardTitle>
          <CardDescription>展示后端返回的 JSON</CardDescription>
        </CardHeader>
        <CardBody>
          {loading && !resp ? (
            <div className="space-y-3">
              <Skeleton className="h-6 w-40" />
              <Skeleton className="h-40 w-full" />
            </div>
          ) : resp ? (
            <pre className="max-h-[520px] overflow-auto whitespace-pre-wrap rounded-lg border border-zinc-200 bg-zinc-50 p-3 font-mono text-xs text-zinc-800">
              {JSON.stringify(resp, null, 2)}
            </pre>
          ) : (
            <div className="text-sm text-zinc-500">暂无响应</div>
          )}
        </CardBody>
      </Card>
    </div>
  );
}

