import { useCallback, useEffect, useMemo, useState } from "react";

import Button from "@/components/ui/Button";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/Card";
import Input from "@/components/ui/Input";
import Skeleton from "@/components/ui/Skeleton";
import { toast } from "@/hooks/useToast";
import { postJson } from "@/utils/genfuApi";

interface SearchItem {
  symbol: string;
  name: string;
  type: string;
  price?: number;
}

interface Position {
  id: number;
  account_id: number;
  instrument: {
    id: number;
    symbol: string;
    name: string;
    asset_type: string;
  };
  quantity: number;
  avg_cost: number;
  market_price: number | null;
}

type Step = "search" | "input";

export default function Investment() {
  const [positions, setPositions] = useState<Position[]>([]);
  const [summary, setSummary] = useState<Record<string, unknown> | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>("");

  // Modal state
  const [open, setOpen] = useState(false);
  const [step, setStep] = useState<Step>("search");
  const [selectedItem, setSelectedItem] = useState<SearchItem | null>(null);
  const [cost, setCost] = useState("");
  const [currentValue, setCurrentValue] = useState("");
  const [submitting, setSubmitting] = useState(false);

  // Search state
  const [searchQuery, setSearchQuery] = useState("");
  const [searchResults, setSearchResults] = useState<SearchItem[]>([]);
  const [searching, setSearching] = useState(false);

  // OCR state
  const [ocrOpen, setOcrOpen] = useState(false);
  const [imageFile, setImageFile] = useState<File | null>(null);

  const loadData = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const [listResp, summaryResp] = await Promise.all([
        postJson<{ output?: unknown; error?: string }>("/api/investment", {
          action: "list_positions",
          limit: 200,
          offset: 0,
        }),
        postJson<{ output?: unknown; error?: string }>("/api/investment", {
          action: "get_portfolio_summary",
        }),
      ]);
      if (listResp.error) {
        setError(listResp.error);
      } else {
        const output = Array.isArray(listResp.output) ? listResp.output : [];
        setPositions(output as Position[]);
      }
      if (summaryResp.error) {
        setError(summaryResp.error);
      } else {
        setSummary((summaryResp.output as Record<string, unknown>) ?? null);
      }
    } catch (e) {
      const msg = e instanceof Error ? e.message : "unknown_error";
      setError(msg);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadData();
  }, [loadData]);

  // Search instruments
  const searchInstruments = useCallback(async (query: string) => {
    if (!query.trim()) {
      setSearchResults([]);
      return;
    }
    setSearching(true);
    try {
      const resp = await postJson<{ output?: SearchItem[]; error?: string }>("/api/investment", {
        action: "search_instruments",
        query: query,
        limit: 10,
      });
      if (resp.output) {
        setSearchResults(resp.output);
      }
    } catch {
      setSearchResults([]);
    } finally {
      setSearching(false);
    }
  }, []);

  // Debounced search
  useEffect(() => {
    const timer = setTimeout(() => {
      if (searchQuery && step === "search") {
        searchInstruments(searchQuery);
      }
    }, 300);
    return () => clearTimeout(timer);
  }, [searchQuery, searchInstruments, step]);

  // Select search result
  const selectItem = (item: SearchItem) => {
    setSelectedItem(item);
    setStep("input");
    setSearchQuery("");
    setSearchResults([]);
  };

  // Back to search
  const backToSearch = () => {
    setStep("search");
    setSelectedItem(null);
    setCost("");
    setCurrentValue("");
  };

  // Delete position
  const deletePosition = async (instrumentId: number, name: string) => {
    if (!confirm(`确定要删除 ${name} 的持仓吗？`)) return;
    try {
      const resp = await postJson<{ error?: string }>("/api/investment", {
        action: "delete_position",
        instrument_id: instrumentId,
      });
      if (resp.error) {
        toast({ title: "删除失败", description: resp.error });
      } else {
        toast({ title: "删除成功" });
        loadData();
      }
    } catch (e) {
      const msg = e instanceof Error ? e.message : "unknown_error";
      toast({ title: "删除失败", description: msg });
    }
  };

  // Submit new position
  const handleSubmit = async () => {
    if (!selectedItem) return;
    if (!cost || !currentValue) {
      toast({ title: "请填写完整信息" });
      return;
    }

    setSubmitting(true);
    try {
      const resp = await postJson<{ error?: string }>("/api/investment", {
        action: "add_position_simple",
        symbol: selectedItem.symbol,
        name: selectedItem.name,
        asset_type: selectedItem.type,
        cost: parseFloat(cost),
        current_value: parseFloat(currentValue),
        market_price: selectedItem.price || 0,
      });
      if (resp.error) {
        toast({ title: "添加失败", description: resp.error });
      } else {
        toast({ title: "添加成功" });
        setOpen(false);
        loadData();
        // Reset state
        setStep("search");
        setSelectedItem(null);
        setCost("");
        setCurrentValue("");
      }
    } catch (e) {
      const msg = e instanceof Error ? e.message : "unknown_error";
      toast({ title: "添加失败", description: msg });
    } finally {
      setSubmitting(false);
    }
  };

  // OCR submit
  const handleOcrSubmit = async () => {
    if (!imageFile) {
      toast({ title: "请先选择截图" });
      return;
    }
    const formData = new FormData();
    formData.append("image", imageFile);
    try {
      const resp = await fetch("/api/investment/ocr_holdings", {
        method: "POST",
        body: formData,
      });
      if (!resp.ok) {
        const msg = await resp.text().catch(() => "");
        throw new Error(msg || `HTTP ${resp.status}`);
      }
      toast({ title: "识别完成", description: "持仓已更新" });
      setOcrOpen(false);
      setImageFile(null);
      loadData();
    } catch (e) {
      const msg = e instanceof Error ? e.message : "unknown_error";
      toast({ title: "识别失败", description: msg });
    }
  };

  const totalValue = useMemo(() => Number(summary?.total_value ?? 0), [summary]);
  const totalCost = useMemo(() => Number(summary?.total_cost ?? 0), [summary]);
  const totalPnL = useMemo(() => Number(summary?.total_pnl ?? 0), [summary]);
  const pnlRate = useMemo(() => (totalCost > 0 ? totalPnL / totalCost : 0), [totalCost, totalPnL]);

  const currency = (summary?.base_currency as string) || "CNY";
  const money = (value: number) => value.toLocaleString(undefined, { maximumFractionDigits: 2 });

  // Calculate estimated quantity and cost price
  const calculatedInfo = useMemo(() => {
    if (!cost || !currentValue || !selectedItem?.price) return null;
    const marketPrice = selectedItem.price;
    const quantity = parseFloat(currentValue) / marketPrice;
    const avgCost = quantity > 0 ? parseFloat(cost) / quantity : 0;
    return { quantity, avgCost, marketPrice };
  }, [cost, currentValue, selectedItem?.price]);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="text-lg font-semibold text-foreground">投资管理</div>
      </div>

      <div className="rounded-2xl border border-border bg-card p-4">
        {loading ? (
          <div className="space-y-3">
            <Skeleton className="h-6 w-40" />
            <Skeleton className="h-24 w-full" />
            <Skeleton className="h-64 w-full" />
          </div>
        ) : (
          <div className="space-y-4">
            <div className="grid gap-3 md:grid-cols-4">
              <Card>
                <CardHeader>
                  <CardTitle>总市值</CardTitle>
                </CardHeader>
                <CardBody>
                  <div className="text-2xl font-semibold text-foreground">{money(totalValue)} {currency}</div>
                </CardBody>
              </Card>
              <Card>
                <CardHeader>
                  <CardTitle>总成本</CardTitle>
                </CardHeader>
                <CardBody>
                  <div className="text-2xl font-semibold text-foreground">{money(totalCost)} {currency}</div>
                </CardBody>
              </Card>
              <Card>
                <CardHeader>
                  <CardTitle>总盈亏</CardTitle>
                </CardHeader>
                <CardBody>
                  <div className={totalPnL >= 0 ? "text-2xl font-semibold text-emerald-500" : "text-2xl font-semibold text-destructive"}>
                    {totalPnL >= 0 ? "+" : ""}{money(totalPnL)} {currency}
                  </div>
                </CardBody>
              </Card>
              <Card>
                <CardHeader>
                  <CardTitle>收益率</CardTitle>
                </CardHeader>
                <CardBody>
                  <div className={pnlRate >= 0 ? "text-2xl font-semibold text-emerald-500" : "text-2xl font-semibold text-destructive"}>
                    {(pnlRate * 100).toFixed(2)}%
                  </div>
                </CardBody>
              </Card>
            </div>

            <Card>
              <CardHeader>
                <CardTitle>持仓明细</CardTitle>
              </CardHeader>
              <CardBody>
                {positions.length === 0 ? (
                  <div className="text-sm text-muted-foreground">暂无持仓，点击下方按钮添加</div>
                ) : (
                  <div className="overflow-auto">
                    <table className="w-full text-left text-sm">
                      <thead className="text-xs text-muted-foreground">
                        <tr>
                          <th className="py-2 pr-3">标的</th>
                          <th className="py-2 pr-3">代码</th>
                          <th className="py-2 pr-3 text-right">数量</th>
                          <th className="py-2 pr-3 text-right">成本价</th>
                          <th className="py-2 pr-3 text-right">当前价</th>
                          <th className="py-2 pr-3 text-right">成本</th>
                          <th className="py-2 pr-3 text-right">市值</th>
                          <th className="py-2 pr-3 text-right">盈亏</th>
                          <th className="py-2 pr-3 text-right">操作</th>
                        </tr>
                      </thead>
                      <tbody>
                        {positions.map((p) => {
                          const currentPrice = p.market_price ?? p.avg_cost;
                          const costVal = p.quantity * p.avg_cost;
                          const valueVal = p.quantity * currentPrice;
                          const pnl = valueVal - costVal;
                          const pnlPct = costVal > 0 ? (pnl / costVal) * 100 : 0;
                          return (
                            <tr key={p.id} className="border-t border-border">
                              <td className="py-2 pr-3">
                                <div className="font-medium text-foreground">{p.instrument.name}</div>
                                <div className="text-xs text-muted-foreground">
                                  {p.instrument.asset_type === "fund" ? "基金" : p.instrument.asset_type === "stock" ? "股票" : p.instrument.asset_type}
                                </div>
                              </td>
                              <td className="py-2 pr-3 font-mono text-xs text-muted-foreground">{p.instrument.symbol}</td>
                              <td className="py-2 pr-3 text-right text-foreground">{p.quantity.toFixed(2)}</td>
                              <td className="py-2 pr-3 text-right text-foreground">{money(p.avg_cost)}</td>
                              <td className="py-2 pr-3 text-right text-foreground">{money(currentPrice)}</td>
                              <td className="py-2 pr-3 text-right text-foreground">{money(costVal)}</td>
                              <td className="py-2 pr-3 text-right font-medium text-foreground">{money(valueVal)}</td>
                              <td className={`py-2 pr-3 text-right ${pnl >= 0 ? "text-emerald-500" : "text-destructive"}`}>
                                <div>{pnl >= 0 ? "+" : ""}{money(pnl)}</div>
                                <div className="text-xs">{pnlPct >= 0 ? "+" : ""}{pnlPct.toFixed(2)}%</div>
                              </td>
                              <td className="py-2 pr-3 text-right">
                                <button
                                  className="rounded px-2 py-1 text-xs text-destructive hover:bg-destructive/10 transition-colors"
                                  onClick={() => deletePosition(p.instrument.id, p.instrument.name)}
                                >
                                  删除
                                </button>
                              </td>
                            </tr>
                          );
                        })}
                      </tbody>
                    </table>
                  </div>
                )}
              </CardBody>
            </Card>
          </div>
        )}
        {error ? <div className="mt-3 text-sm text-destructive">{error}</div> : null}
      </div>

      <div className="flex items-center gap-2">
        <Button variant="secondary" onClick={() => setOcrOpen(true)}>
          截图识别
        </Button>
        <Button
          onClick={() => {
            setStep("search");
            setSearchQuery("");
            setSearchResults([]);
            setSelectedItem(null);
            setCost("");
            setCurrentValue("");
            setOpen(true);
          }}
        >
          新增持仓
        </Button>
      </div>

      {/* Add position modal */}
      {open ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 backdrop-blur-sm p-4">
          <div className="w-full max-w-md rounded-xl border border-border bg-card p-4 shadow-xl">
            <div className="text-sm font-semibold text-foreground">
              {step === "search" ? "搜索基金/股票" : "添加持仓"}
            </div>

            {step === "search" ? (
              <div className="mt-3 space-y-3">
                <Input
                  placeholder="输入代码或名称搜索..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  autoFocus
                />
                {searching && (
                  <div className="text-xs text-muted-foreground">搜索中...</div>
                )}
                {searchResults.length > 0 && (
                  <div className="max-h-64 space-y-1 overflow-auto rounded-lg border border-border">
                    {searchResults.map((item) => (
                      <button
                        key={item.symbol}
                        type="button"
                        className="w-full px-3 py-2 text-left text-sm hover:bg-accent hover:text-accent-foreground transition-colors"
                        onClick={() => selectItem(item)}
                      >
                        <span className="font-mono text-xs text-muted-foreground">{item.symbol}</span>
                        <span className="ml-2 font-medium text-foreground">{item.name}</span>
                        <span className="ml-2 text-xs text-muted-foreground">
                          {item.type === "fund" ? "基金" : item.type === "stock" ? "股票" : item.type}
                        </span>
                        {item.price && (
                          <span className="ml-2 text-xs text-muted-foreground">￥{item.price.toFixed(2)}</span>
                        )}
                      </button>
                    ))}
                  </div>
                )}
                {searchQuery && !searching && searchResults.length === 0 && (
                  <div className="text-sm text-muted-foreground">未找到结果</div>
                )}
              </div>
            ) : (
              <div className="mt-3 space-y-3">
                <div className="rounded-lg bg-muted/50 px-3 py-2">
                  <div className="text-xs text-muted-foreground">已选择</div>
                  <div className="font-medium text-foreground">{selectedItem?.name}</div>
                  <div className="text-xs text-muted-foreground">
                    {selectedItem?.symbol} · {selectedItem?.type === "fund" ? "基金" : selectedItem?.type === "stock" ? "股票" : selectedItem?.type}
                    {selectedItem?.price && ` · 当前价: ￥${selectedItem.price.toFixed(2)}`}
                  </div>
                </div>
                <Input
                  placeholder="购入总成本（元）"
                  type="number"
                  value={cost}
                  onChange={(e) => setCost(e.target.value)}
                  autoFocus
                />
                <Input
                  placeholder="当前总金额/市值（元）"
                  type="number"
                  value={currentValue}
                  onChange={(e) => setCurrentValue(e.target.value)}
                />

                {/* Show calculated info */}
                {calculatedInfo && (
                  <div className="rounded-lg bg-accent/10 px-3 py-2 text-sm">
                    <div className="text-xs text-accent mb-1">持仓信息计算</div>
                    <div className="grid grid-cols-2 gap-2 text-muted-foreground">
                      <div>持有数量: <span className="font-medium text-foreground">{calculatedInfo.quantity.toFixed(2)}份</span></div>
                      <div>成本价: <span className="font-medium text-foreground">￥{calculatedInfo.avgCost.toFixed(4)}</span></div>
                      <div>当前价: <span className="font-medium text-foreground">￥{calculatedInfo.marketPrice.toFixed(2)}</span></div>
                      <div>盈亏: <span className={parseFloat(currentValue) >= parseFloat(cost) ? "text-emerald-500" : "text-destructive"}>
                        {money(parseFloat(currentValue) - parseFloat(cost))}元
                      </span></div>
                    </div>
                  </div>
                )}

                {!selectedItem?.price && cost && currentValue && (
                  <div className="rounded-lg bg-warning/10 px-3 py-2 text-xs text-warning">
                    未获取到实时价格，数量将设为1，成本价=购入成本，当前价=当前金额
                  </div>
                )}
              </div>
            )}

            <div className="mt-4 flex items-center justify-between gap-2">
              {step === "input" && (
                <Button variant="ghost" onClick={backToSearch}>
                  返回
                </Button>
              )}
              <div className="flex-1" />
              <Button variant="ghost" onClick={() => setOpen(false)}>
                取消
              </Button>
              {step === "input" && (
                <Button onClick={handleSubmit} disabled={submitting || !cost || !currentValue}>
                  {submitting ? "添加中..." : "确认添加"}
                </Button>
              )}
            </div>
          </div>
        </div>
      ) : null}

      {/* OCR modal */}
      {ocrOpen ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 backdrop-blur-sm p-4">
          <div className="w-full max-w-md rounded-xl border border-border bg-card p-4 shadow-xl">
            <div className="text-sm font-semibold text-foreground">截图识别持仓</div>
            <div className="mt-3 space-y-2">
              <input
                type="file"
                accept="image/*"
                className="text-sm text-foreground"
                onChange={(e) => {
                  const file = e.target.files?.[0] ?? null;
                  setImageFile(file);
                }}
              />
              <div className="text-xs text-muted-foreground">上传后将覆盖现有持仓</div>
            </div>
            <div className="mt-4 flex items-center justify-end gap-2">
              <Button variant="ghost" onClick={() => setOcrOpen(false)}>
                取消
              </Button>
              <Button onClick={handleOcrSubmit} disabled={!imageFile}>
                识别
              </Button>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}
