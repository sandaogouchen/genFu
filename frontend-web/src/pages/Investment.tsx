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
  type?: string;
  asset_type?: string;
  price?: number;
}

interface SnapshotPosition {
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
  current_price: number;
  price_source: "realtime" | "stored" | "avg_cost" | string;
  cost: number;
  market_value: number;
  pnl: number;
  pnl_pct: number;
}

interface SnapshotSummary {
  account_id: number;
  position_count: number;
  trade_count: number;
  total_cost: number;
  total_value: number;
  total_pnl: number;
  pnl_pct: number;
}

interface SnapshotPriceFailure {
  symbol: string;
  name: string;
  asset_type: string;
  reason: string;
}

interface PortfolioSnapshot {
  positions: SnapshotPosition[];
  summary: SnapshotSummary;
  refreshed_at: string;
  has_stale_prices: boolean;
  price_failures?: SnapshotPriceFailure[];
}

type Step = "search" | "input";

export default function Investment() {
  const [positions, setPositions] = useState<SnapshotPosition[]>([]);
  const [summary, setSummary] = useState<SnapshotSummary | null>(null);
  const [refreshedAt, setRefreshedAt] = useState("");
  const [hasStalePrices, setHasStalePrices] = useState(false);
  const [priceFailures, setPriceFailures] = useState<SnapshotPriceFailure[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string>("");

  // Modal state
  const [open, setOpen] = useState(false);
  const [step, setStep] = useState<Step>("search");
  const [selectedItem, setSelectedItem] = useState<SearchItem | null>(null);
  const [quantity, setQuantity] = useState("");
  const [avgCost, setAvgCost] = useState("");
  const [submitting, setSubmitting] = useState(false);

  // Search state
  const [searchQuery, setSearchQuery] = useState("");
  const [searchResults, setSearchResults] = useState<SearchItem[]>([]);
  const [searching, setSearching] = useState(false);
  const visibleSearchResults = useMemo(() => searchResults.slice(0, 6), [searchResults]);

  // OCR state
  const [ocrOpen, setOcrOpen] = useState(false);
  const [imageFile, setImageFile] = useState<File | null>(null);

  const normalizeAssetType = useCallback((item: SearchItem | null | undefined): string => {
    return item?.asset_type ?? item?.type ?? "unknown";
  }, []);

  const assetTypeLabel = useCallback((assetType: string): string => {
    if (assetType === "fund") return "基金";
    if (assetType === "stock") return "股票";
    return assetType;
  }, []);

  const loadData = useCallback(async () => {
    setLoading(true);
    setError("");
    try {
      const snapshotResp = await postJson<{ output?: PortfolioSnapshot; error?: string }>("/api/investment", {
        action: "get_portfolio_snapshot",
      });
      if (snapshotResp.error) {
        setError(snapshotResp.error);
      } else {
        const snapshot = snapshotResp.output;
        setPositions(snapshot?.positions ?? []);
        setSummary(snapshot?.summary ?? null);
        setRefreshedAt(snapshot?.refreshed_at ?? "");
        setHasStalePrices(Boolean(snapshot?.has_stale_prices));
        setPriceFailures(snapshot?.price_failures ?? []);
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
    setQuantity("");
    setAvgCost("");
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
        void loadData();
      }
    } catch (e) {
      const msg = e instanceof Error ? e.message : "unknown_error";
      toast({ title: "删除失败", description: msg });
    }
  };

  // Submit new position
  const handleSubmit = async () => {
    if (!selectedItem) return;
    if (!quantity || !avgCost) {
      toast({ title: "请填写完整信息" });
      return;
    }
    const quantityValue = parseFloat(quantity);
    const avgCostValue = parseFloat(avgCost);
    if (!Number.isFinite(quantityValue) || !Number.isFinite(avgCostValue) || quantityValue <= 0 || avgCostValue <= 0) {
      toast({ title: "请输入有效的数量和成本单价" });
      return;
    }

    setSubmitting(true);
    try {
      const payload: Record<string, unknown> = {
        action: "add_position_by_quantity",
        symbol: selectedItem.symbol,
        name: selectedItem.name,
        asset_type: normalizeAssetType(selectedItem),
        quantity: quantityValue,
        avg_cost: avgCostValue,
      };
      if (selectedItem.price && selectedItem.price > 0) {
        payload.market_price = selectedItem.price;
      }
      const resp = await postJson<{ error?: string }>("/api/investment", {
        ...payload,
      });
      if (resp.error) {
        toast({ title: "添加失败", description: resp.error });
      } else {
        toast({ title: "添加成功" });
        setOpen(false);
        void loadData();
        // Reset state
        setStep("search");
        setSelectedItem(null);
        setQuantity("");
        setAvgCost("");
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
      void loadData();
    } catch (e) {
      const msg = e instanceof Error ? e.message : "unknown_error";
      toast({ title: "识别失败", description: msg });
    }
  };

  const totalValue = useMemo(() => Number(summary?.total_value ?? 0), [summary?.total_value]);
  const totalCost = useMemo(() => Number(summary?.total_cost ?? 0), [summary?.total_cost]);
  const totalPnL = useMemo(() => Number(summary?.total_pnl ?? 0), [summary?.total_pnl]);
  const pnlRate = useMemo(() => Number(summary?.pnl_pct ?? (totalCost > 0 ? totalPnL / totalCost : 0)), [summary?.pnl_pct, totalCost, totalPnL]);

  const currency = "CNY";
  const money = (value: number) => value.toLocaleString(undefined, { maximumFractionDigits: 2 });

  // Calculate estimated snapshot for add-position modal.
  const calculatedInfo = useMemo(() => {
    if (!quantity || !avgCost) return null;
    const quantityValue = parseFloat(quantity);
    const avgCostValue = parseFloat(avgCost);
    if (!Number.isFinite(quantityValue) || !Number.isFinite(avgCostValue)) return null;
    if (quantityValue <= 0 || avgCostValue <= 0) return null;
    const costValue = quantityValue * avgCostValue;
    const marketPrice = selectedItem?.price;
    if (!marketPrice || marketPrice <= 0) {
      return {
        quantity: quantityValue,
        avgCost: avgCostValue,
        cost: costValue,
        marketPrice: null,
        marketValue: null,
        pnl: null,
      };
    }
    const marketValue = quantityValue * marketPrice;
    return {
      quantity: quantityValue,
      avgCost: avgCostValue,
      cost: costValue,
      marketPrice,
      marketValue,
      pnl: marketValue - costValue,
    };
  }, [avgCost, quantity, selectedItem?.price]);

  return (
    <div className="space-y-4">
      <div className="flex items-center justify-between">
        <div className="text-lg font-semibold text-foreground">投资管理</div>
        <Button variant="secondary" onClick={() => void loadData()} disabled={loading}>
          {loading ? "刷新中..." : "手动刷新"}
        </Button>
      </div>

      <div className="rounded-2xl border border-border bg-card p-4">
        <div className="mb-3 flex items-center justify-between text-xs text-muted-foreground">
          <div>
            {refreshedAt ? `最近刷新：${new Date(refreshedAt).toLocaleString()}` : "最近刷新：--"}
          </div>
          {hasStalePrices ? <div>部分标的使用了回退价格</div> : null}
        </div>
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
                          const currentPrice = Number(p.current_price);
                          const costVal = Number(p.cost);
                          const valueVal = Number(p.market_value);
                          const pnl = Number(p.pnl);
                          const pnlPct = Number(p.pnl_pct) * 100;
                          return (
                            <tr key={p.id} className="border-t border-border">
                              <td className="py-2 pr-3">
                                <div className="font-medium text-foreground">{p.instrument.name}</div>
                                <div className="text-xs text-muted-foreground">
                                  {assetTypeLabel(p.instrument.asset_type)}
                                </div>
                              </td>
                              <td className="py-2 pr-3 font-mono text-xs text-muted-foreground">{p.instrument.symbol}</td>
                              <td className="py-2 pr-3 text-right text-foreground">{p.quantity.toFixed(2)}</td>
                              <td className="py-2 pr-3 text-right text-foreground">{money(p.avg_cost)}</td>
                              <td className="py-2 pr-3 text-right text-foreground">
                                <div>{money(currentPrice)}</div>
                                <div className="text-xs text-muted-foreground">{p.price_source}</div>
                              </td>
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
        {hasStalePrices ? (
          <div className="mt-3 rounded-lg bg-warning/10 px-3 py-2 text-xs text-warning">
            部分标的未取到实时价，已回退缓存/成本价{priceFailures.length > 0 ? `（${priceFailures.length} 个）` : ""}。
          </div>
        ) : null}
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
            setQuantity("");
            setAvgCost("");
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
                  placeholder="输入基金/股票代码或名称搜索..."
                  value={searchQuery}
                  onChange={(e) => setSearchQuery(e.target.value)}
                  autoFocus
                />
                {searching && (
                  <div className="text-xs text-muted-foreground">搜索中...</div>
                )}
                {visibleSearchResults.length > 0 && (
                  <div className="space-y-2">
                    <div className="grid max-h-72 grid-cols-1 gap-2 overflow-auto">
                      {visibleSearchResults.map((item) => (
                        <button
                          key={`${item.symbol}-${item.name}`}
                          type="button"
                          className="w-full rounded-lg border border-border bg-card px-3 py-2 text-left transition-colors hover:border-accent hover:bg-accent/30"
                          onClick={() => selectItem(item)}
                        >
                          <div className="flex items-center justify-between gap-2">
                            <div className="font-medium text-foreground">{item.name}</div>
                            <div className="rounded-full bg-muted px-2 py-0.5 text-[11px] text-muted-foreground">
                              {assetTypeLabel(normalizeAssetType(item))}
                            </div>
                          </div>
                          <div className="mt-1 flex items-center gap-2 text-xs text-muted-foreground">
                            <span className="font-mono">{item.symbol}</span>
                            {item.price && item.price > 0 ? <span>￥{item.price.toFixed(2)}</span> : null}
                          </div>
                        </button>
                      ))}
                    </div>
                    {searchResults.length > visibleSearchResults.length ? (
                      <div className="text-xs text-muted-foreground">
                        结果较多，仅展示前 {visibleSearchResults.length} 条
                      </div>
                    ) : null}
                  </div>
                )}
                {searchQuery && !searching && searchResults.length === 0 && (
                  <div className="text-sm text-muted-foreground">未找到匹配标的</div>
                )}
              </div>
            ) : (
              <div className="mt-3 space-y-3">
                <div className="rounded-lg bg-muted/50 px-3 py-2">
                  <div className="text-xs text-muted-foreground">已选择</div>
                  <div className="font-medium text-foreground">{selectedItem?.name}</div>
                  <div className="text-xs text-muted-foreground">
                    {selectedItem?.symbol} · {assetTypeLabel(normalizeAssetType(selectedItem))}
                    {selectedItem?.price && ` · 当前价: ￥${selectedItem.price.toFixed(2)}`}
                  </div>
                </div>
                <Input
                  placeholder="持有数量（份）"
                  type="number"
                  value={quantity}
                  onChange={(e) => setQuantity(e.target.value)}
                  autoFocus
                />
                <Input
                  placeholder="成本单价（元/份）"
                  type="number"
                  value={avgCost}
                  onChange={(e) => setAvgCost(e.target.value)}
                />

                {/* Show calculated info */}
                {calculatedInfo && (
                  <div className="rounded-lg bg-accent/10 px-3 py-2 text-sm">
                    <div className="mb-1 text-xs text-accent">持仓信息预估</div>
                    <div className="grid grid-cols-2 gap-2 text-muted-foreground">
                      <div>持有数量: <span className="font-medium text-foreground">{calculatedInfo.quantity.toFixed(2)}份</span></div>
                      <div>成本单价: <span className="font-medium text-foreground">￥{calculatedInfo.avgCost.toFixed(4)}</span></div>
                      <div>总成本: <span className="font-medium text-foreground">￥{money(calculatedInfo.cost)}</span></div>
                      <div>
                        当前市值:
                        <span className="font-medium text-foreground">
                          {calculatedInfo.marketValue !== null ? ` ￥${money(calculatedInfo.marketValue)}` : " --"}
                        </span>
                      </div>
                      <div>
                        预估盈亏:
                        {calculatedInfo.pnl === null ? (
                          <span className="font-medium text-foreground"> --</span>
                        ) : (
                          <span className={calculatedInfo.pnl >= 0 ? "text-emerald-500" : "text-destructive"}>
                            {" "}{calculatedInfo.pnl >= 0 ? "+" : ""}{money(calculatedInfo.pnl)}元
                          </span>
                        )}
                      </div>
                    </div>
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
                <Button onClick={handleSubmit} disabled={submitting || !quantity || !avgCost}>
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
