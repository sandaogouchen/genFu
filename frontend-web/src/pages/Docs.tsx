import { useEffect, useMemo, useState } from "react";

import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import Select from "@/components/ui/Select";
import { Card, CardBody, CardDescription, CardHeader, CardTitle } from "@/components/ui/Card";
import Skeleton from "@/components/ui/Skeleton";
import { toast } from "@/hooks/useToast";
import { getApiKeyConfig, setApiKeyConfig, type ApiKeyMode } from "@/utils/settings";

export default function Docs() {
  const [mode, setMode] = useState<ApiKeyMode>("authorization_bearer");
  const [key, setKey] = useState<string>("");
  const [checking, setChecking] = useState(false);
  const [health, setHealth] = useState<string>("");

  useEffect(() => {
    const cfg = getApiKeyConfig();
    setMode(cfg.mode);
    setKey(cfg.key);
  }, []);

  const healthBadge = useMemo(() => {
    if (checking) return <Skeleton className="h-6 w-28" />;
    if (!health) return <div className="text-xs text-muted-foreground">未检测</div>;
    return <div className="text-xs text-foreground">{health}</div>;
  }, [checking, health]);

  return (
    <div className="space-y-4">
      <Card>
        <CardHeader>
          <CardTitle>文档 / 调试</CardTitle>
          <CardDescription>后端提供 Swagger 与 OpenAPI JSON</CardDescription>
        </CardHeader>
        <CardBody>
          <div className="flex flex-wrap gap-2">
            <Button variant="secondary" size="sm" onClick={() => window.open("/docs", "_blank")}>Swagger</Button>
            <Button variant="secondary" size="sm" onClick={() => window.open("/openapi.json", "_blank")}>OpenAPI</Button>
          </div>
        </CardBody>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>鉴权</CardTitle>
          <CardDescription>如后端启用了 API Key，可在此配置并写入本地存储</CardDescription>
        </CardHeader>
        <CardBody>
          <div className="grid gap-3 md:grid-cols-[220px_1fr]">
            <div>
              <div className="text-xs text-muted-foreground">Key 方式</div>
              <Select value={mode} onChange={(e) => setMode(e.target.value as ApiKeyMode)} className="mt-1">
                <option value="authorization_bearer">Authorization: Bearer</option>
                <option value="x-api-key">X-Api-Key</option>
                <option value="x-goog-api-key">X-Goog-Api-Key</option>
              </Select>
            </div>
            <div>
              <div className="text-xs text-muted-foreground">API Key</div>
              <Input value={key} onChange={(e) => setKey(e.target.value)} placeholder="可留空" className="mt-1" />
            </div>
          </div>
          <div className="mt-3 flex flex-wrap items-center gap-2">
            <Button
              size="sm"
              variant="secondary"
              onClick={() => {
                setApiKeyConfig({ key: key.trim(), mode });
                toast({ title: "已保存", description: "API Key 配置已写入本地存储" });
              }}
            >
              保存
            </Button>
            <Button
              size="sm"
              variant="secondary"
              onClick={async () => {
                setChecking(true);
                try {
                  const resp = await fetch("/healthz", { headers: { ...(() => {
                    const cfg = getApiKeyConfig();
                    if (!cfg.key) return {};
                    if (cfg.mode === "x-api-key") return { "X-Api-Key": cfg.key };
                    if (cfg.mode === "x-goog-api-key") return { "X-Goog-Api-Key": cfg.key };
                    return { Authorization: `Bearer ${cfg.key}` };
                  })() } });
                  const text = await resp.text();
                  setHealth(resp.ok ? (text || "ok") : `HTTP ${resp.status}`);
                  toast({ title: resp.ok ? "联通正常" : "联通失败", description: resp.ok ? "已成功访问 /healthz" : `状态码：${resp.status}` });
                } catch (e) {
                  setHealth(e instanceof Error ? e.message : "network_error");
                  toast({ title: "联通失败", description: e instanceof Error ? e.message : "network_error" });
                } finally {
                  setChecking(false);
                }
              }}
            >
              测试 /healthz
            </Button>
            <div className="ml-auto">{healthBadge}</div>
          </div>
        </CardBody>
      </Card>
    </div>
  );
}
