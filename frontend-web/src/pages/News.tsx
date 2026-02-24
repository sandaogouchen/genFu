import { useEffect, useState } from "react";
import { Card, CardBody, CardHeader, CardTitle } from "@/components/ui/Card";
import Button from "@/components/ui/Button";
import Input from "@/components/ui/Input";
import {
  listNewsEvents,
  getNewsBriefing,
  triggerNewsAnalysis,
  NewsEvent,
  Briefing,
  EventDomain,
  NewsEventQuery,
  PipelineRunResult,
} from "@/utils/genfuApi";

// Event domain display names
const DOMAIN_NAMES: Record<EventDomain, string> = {
  macro: "宏观经济",
  geopolitical: "地缘政治",
  industry: "行业动态",
  corporate: "公司事件",
  regulatory: "监管政策",
  market: "市场行为",
  technology: "技术突破",
};

// Sentiment colors
const getSentimentColor = (sentiment: number): string => {
  if (sentiment >= 0.6) return "text-emerald-600 dark:text-emerald-400";
  if (sentiment >= 0.2) return "text-emerald-500 dark:text-emerald-400";
  if (sentiment >= -0.2) return "text-muted-foreground";
  if (sentiment >= -0.6) return "text-destructive";
  return "text-destructive";
};

const getSentimentLabel = (sentiment: number): string => {
  if (sentiment >= 0.6) return "非常利好";
  if (sentiment >= 0.2) return "利好";
  if (sentiment >= -0.2) return "中性";
  if (sentiment >= -0.6) return "利空";
  return "非常利空";
};

// Priority badge
const getPriorityBadge = (priority?: number): React.ReactNode => {
  if (!priority) return null;
  const colors: Record<number, string> = {
    1: "bg-muted text-muted-foreground",
    2: "bg-accent/10 text-accent border border-accent/20",
    3: "bg-warning/10 text-warning border border-warning/20",
    4: "bg-orange-500/10 text-orange-500 border border-orange-500/20",
    5: "bg-destructive/10 text-destructive border border-destructive/20",
  };
  return (
    <span className={`px-2 py-0.5 rounded text-xs font-medium ${colors[priority] || colors[1]}`}>
      P{priority}
    </span>
  );
};

// Domain badge
const DomainBadge = ({ domain }: { domain: EventDomain }) => (
  <span className="px-2 py-0.5 rounded text-xs bg-accent/10 text-accent border border-accent/20">
    {DOMAIN_NAMES[domain] || domain}
  </span>
);

export default function News() {
  const [events, setEvents] = useState<NewsEvent[]>([]);
  const [briefing, setBriefing] = useState<Briefing | null>(null);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(1);
  const [pageSize] = useState(20);
  const [loading, setLoading] = useState(false);
  const [analyzing, setAnalyzing] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<"events" | "briefing">("events");

  // Filters
  const [selectedDomain, setSelectedDomain] = useState<string>("");
  const [selectedSentiment, setSelectedSentiment] = useState<string>("");
  const [keywords, setKeywords] = useState("");
  const [dateFrom, setDateFrom] = useState("");
  const [dateTo, setDateTo] = useState("");

  // Fetch events
  const fetchEvents = async () => {
    setLoading(true);
    setError(null);
    try {
      const query: NewsEventQuery = {
        page,
        page_size: pageSize,
      };
      if (selectedDomain) query.domain = selectedDomain;
      if (selectedSentiment) query.sentiment = selectedSentiment;
      if (keywords) query.keywords = keywords;
      if (dateFrom) query.date_from = dateFrom;
      if (dateTo) query.date_to = dateTo;

      const response = await listNewsEvents(query);
      setEvents(response.items || []);
      setTotal(response.total || 0);
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载失败");
    } finally {
      setLoading(false);
    }
  };

  // Fetch briefing
  const fetchBriefing = async () => {
    setLoading(true);
    setError(null);
    try {
      const response = await getNewsBriefing();
      setBriefing(response);
    } catch (err) {
      setError(err instanceof Error ? err.message : "加载简报失败");
    } finally {
      setLoading(false);
    }
  };

  // Trigger analysis
  const handleAnalyze = async () => {
    setAnalyzing(true);
    setError(null);
    try {
      const result: PipelineRunResult = await triggerNewsAnalysis();
      if (result.error) {
        setError(result.error);
      } else {
        // Refresh events after analysis
        await fetchEvents();
        if (result.briefing) {
          setBriefing(result.briefing);
        }
      }
    } catch (err) {
      setError(err instanceof Error ? err.message : "分析失败");
    } finally {
      setAnalyzing(false);
    }
  };

  // Initial load
  useEffect(() => {
    if (activeTab === "events") {
      fetchEvents();
    } else {
      fetchBriefing();
    }
  }, [page, activeTab]);

  // Apply filters
  const applyFilters = () => {
    setPage(1);
    fetchEvents();
  };

  // Reset filters
  const resetFilters = () => {
    setSelectedDomain("");
    setSelectedSentiment("");
    setKeywords("");
    setDateFrom("");
    setDateTo("");
    setPage(1);
  };

  // Calculate total pages
  const totalPages = Math.ceil(total / pageSize);

  return (
    <div className="space-y-4">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-2xl font-semibold text-foreground">新闻分析</h1>
          <p className="text-sm text-muted-foreground mt-1">
            三层漏斗筛选 + 六模块投资简报
          </p>
        </div>
        <Button onClick={handleAnalyze} disabled={analyzing}>
          {analyzing ? "分析中..." : "触发分析"}
        </Button>
      </div>

      {/* Error Alert */}
      {error && (
        <div className="bg-destructive/10 border border-destructive/20 text-destructive px-4 py-3 rounded-lg">
          {error}
        </div>
      )}

      {/* Tabs */}
      <div className="border-b border-border">
        <nav className="-mb-px flex space-x-8">
          <button
            onClick={() => setActiveTab("events")}
            className={`py-2 px-1 border-b-2 font-medium text-sm transition-colors ${
              activeTab === "events"
                ? "border-accent text-accent"
                : "border-transparent text-muted-foreground hover:text-foreground"
            }`}
          >
            新闻事件 ({total})
          </button>
          <button
            onClick={() => setActiveTab("briefing")}
            className={`py-2 px-1 border-b-2 font-medium text-sm transition-colors ${
              activeTab === "briefing"
                ? "border-accent text-accent"
                : "border-transparent text-muted-foreground hover:text-foreground"
            }`}
          >
            投资简报
          </button>
        </nav>
      </div>

      {/* Events Tab */}
      {activeTab === "events" && (
        <>
          {/* Filters */}
          <Card>
            <CardBody>
              <div className="grid grid-cols-1 md:grid-cols-5 gap-4">
                <div>
                  <label className="block text-sm font-medium text-foreground mb-1">
                    事件域
                  </label>
                  <select
                    value={selectedDomain}
                    onChange={(e) => setSelectedDomain(e.target.value)}
                    className="w-full rounded-lg border border-input bg-background px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  >
                    <option value="">全部</option>
                    {Object.entries(DOMAIN_NAMES).map(([key, name]) => (
                      <option key={key} value={key}>
                        {name}
                      </option>
                    ))}
                  </select>
                </div>

                <div>
                  <label className="block text-sm font-medium text-foreground mb-1">
                    情感
                  </label>
                  <select
                    value={selectedSentiment}
                    onChange={(e) => setSelectedSentiment(e.target.value)}
                    className="w-full rounded-lg border border-input bg-background px-3 py-2 text-sm focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
                  >
                    <option value="">全部</option>
                    <option value="positive">利好</option>
                    <option value="negative">利空</option>
                    <option value="neutral">中性</option>
                  </select>
                </div>

                <div>
                  <label className="block text-sm font-medium text-foreground mb-1">
                    关键词
                  </label>
                  <Input
                    type="text"
                    value={keywords}
                    onChange={(e) => setKeywords(e.target.value)}
                    placeholder="多个关键词用逗号分隔"
                  />
                </div>

                <div>
                  <label className="block text-sm font-medium text-foreground mb-1">
                    开始日期
                  </label>
                  <Input
                    type="date"
                    value={dateFrom}
                    onChange={(e) => setDateFrom(e.target.value)}
                  />
                </div>

                <div>
                  <label className="block text-sm font-medium text-foreground mb-1">
                    结束日期
                  </label>
                  <Input
                    type="date"
                    value={dateTo}
                    onChange={(e) => setDateTo(e.target.value)}
                  />
                </div>
              </div>

              <div className="flex justify-end mt-4 space-x-2">
                <Button variant="secondary" onClick={resetFilters}>
                  重置
                </Button>
                <Button onClick={applyFilters}>应用筛选</Button>
              </div>
            </CardBody>
          </Card>

          {/* Events List */}
          {loading ? (
            <div className="text-center py-12">
              <div className="text-muted-foreground">加载中...</div>
            </div>
          ) : events.length === 0 ? (
            <div className="text-center py-12">
              <div className="text-muted-foreground">暂无新闻事件</div>
            </div>
          ) : (
            <div className="space-y-4">
              {events.map((event) => (
                <Card key={event.id} className="transition-colors hover:border-accent/50">
                  <CardBody>
                    <div className="flex items-start justify-between">
                      <div className="flex-1">
                        {/* Title and URL */}
                        <div className="flex items-start gap-2">
                          <a
                            href={event.url}
                            target="_blank"
                            rel="noopener noreferrer"
                            className="text-lg font-medium text-foreground hover:text-accent transition-colors"
                          >
                            {event.title}
                          </a>
                          {event.funnel_result?.l2_priority && (
                            getPriorityBadge(event.funnel_result.l2_priority)
                          )}
                        </div>

                        {/* Domains and Event Types */}
                        <div className="flex flex-wrap gap-2 mt-2">
                          {event.domains?.map((domain) => (
                            <DomainBadge key={domain} domain={domain} />
                          ))}
                          {event.event_types?.map((type) => (
                            <span
                              key={type}
                              className="px-2 py-0.5 rounded text-xs bg-muted text-muted-foreground"
                            >
                              {type}
                            </span>
                          ))}
                        </div>

                        {/* Summary */}
                        <p className="text-sm text-muted-foreground mt-2 line-clamp-2">
                          {event.summary}
                        </p>

                        {/* Metadata */}
                        <div className="flex items-center gap-4 mt-2 text-xs text-muted-foreground">
                          <span>{event.source}</span>
                          <span>
                            {new Date(event.published_at).toLocaleString("zh-CN")}
                          </span>
                          {event.labels && (
                            <>
                              <span className={getSentimentColor(event.labels.sentiment)}>
                                {getSentimentLabel(event.labels.sentiment)} ({event.labels.sentiment.toFixed(2)})
                              </span>
                            </>
                          )}
                          <span>分类: {event.classify_method}</span>
                          <span>置信度: {(event.classify_confidence * 100).toFixed(0)}%</span>
                        </div>

                        {/* L2 Analysis */}
                        {event.funnel_result && (
                          <div className="mt-3 p-3 bg-muted/30 rounded-lg text-sm">
                            <div className="flex items-center gap-2 mb-2">
                              <span className="font-medium text-foreground">L2 分析:</span>
                              <span
                                className={`px-2 py-0.5 rounded ${
                                  event.funnel_result.l2_relevance === "high"
                                    ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
                                    : event.funnel_result.l2_relevance === "medium"
                                    ? "bg-warning/10 text-warning"
                                    : "bg-muted text-muted-foreground"
                                }`}
                              >
                                {event.funnel_result.l2_relevance}
                              </span>
                            </div>
                            {event.funnel_result.l2_causal_sketch && (
                              <p className="text-muted-foreground">
                                {event.funnel_result.l2_causal_sketch}
                              </p>
                            )}
                            {event.funnel_result.l2_affected_assets && event.funnel_result.l2_affected_assets.length > 0 && (
                              <div className="mt-2 flex flex-wrap gap-2">
                                <span className="font-medium text-foreground">影响资产:</span>
                                {event.funnel_result.l2_affected_assets.map((asset, idx) => (
                                  <span
                                    key={idx}
                                    className={`px-2 py-0.5 rounded ${
                                      asset.direction === "bullish"
                                        ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
                                        : asset.direction === "bearish"
                                        ? "bg-destructive/10 text-destructive"
                                        : "bg-muted text-muted-foreground"
                                    }`}
                                  >
                                    {asset.asset_name} ({asset.direction})
                                    {asset.is_holding && " ✓"}
                                  </span>
                                ))}
                              </div>
                            )}
                          </div>
                        )}
                      </div>
                    </div>
                  </CardBody>
                </Card>
              ))}
            </div>
          )}

          {/* Pagination */}
          {totalPages > 1 && (
            <div className="flex items-center justify-between">
              <div className="text-sm text-muted-foreground">
                共 {total} 条，第 {page} / {totalPages} 页
              </div>
              <div className="flex gap-2">
                <Button
                  variant="secondary"
                  onClick={() => setPage(Math.max(1, page - 1))}
                  disabled={page === 1}
                >
                  上一页
                </Button>
                <Button
                  variant="secondary"
                  onClick={() => setPage(Math.min(totalPages, page + 1))}
                  disabled={page === totalPages}
                >
                  下一页
                </Button>
              </div>
            </div>
          )}
        </>
      )}

      {/* Briefing Tab */}
      {activeTab === "briefing" && (
        <>
          {loading ? (
            <div className="text-center py-12">
              <div className="text-muted-foreground">加载中...</div>
            </div>
          ) : briefing ? (
            <div className="space-y-4">
              {/* Header */}
              <Card>
                <CardHeader>
                  <CardTitle>投资简报</CardTitle>
                  <div className="text-sm text-muted-foreground mt-1">
                    触发类型: {briefing.trigger_type} | 时间:{" "}
                    {new Date(briefing.generated_at).toLocaleString("zh-CN")}
                  </div>
                  <div className="flex gap-4 mt-2 text-sm text-muted-foreground">
                    <span>总新闻: {briefing.total_news_processed}</span>
                    <span>L1通过: {briefing.l1_passed}</span>
                    <span>L2通过: {briefing.l2_passed}</span>
                    <span>L3分析: {briefing.l3_analyzed}</span>
                  </div>
                </CardHeader>
              </Card>

              {/* Module 1: Macro Overview */}
              {briefing.macro_overview && (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-lg">模块一：宏观概览</CardTitle>
                  </CardHeader>
                  <CardBody>
                    <p className="text-sm mb-3 text-foreground">{briefing.macro_overview.summary}</p>
                    <div className="grid grid-cols-2 gap-2 text-sm text-muted-foreground">
                      <div>
                        市场情绪:{" "}
                        <span className="font-medium text-foreground">
                          {briefing.macro_overview.market_sentiment}
                        </span>
                      </div>
                      <div>
                        风险水平:{" "}
                        <span className="font-medium text-foreground">
                          {briefing.macro_overview.risk_level}
                        </span>
                      </div>
                    </div>
                    {briefing.macro_overview?.key_factors && briefing.macro_overview.key_factors.length > 0 && (
                    <div className="mt-3">
                      <div className="font-medium text-sm mb-2 text-foreground">关键因素:</div>
                      <ul className="list-disc list-inside text-sm space-y-1 text-muted-foreground">
                        {briefing.macro_overview.key_factors.map((factor, idx) => (
                          <li key={idx}>
                            {factor.factor} ({factor.direction})
                          </li>
                        ))}
                      </ul>
                    </div>
                  )}
                </CardBody>
              </Card>
              )}

              {/* Module 2: Portfolio Impact */}
              <Card>
                <CardHeader>
                  <CardTitle className="text-lg">模块二：持仓影响矩阵</CardTitle>
                </CardHeader>
                <CardBody>
                  {!briefing.portfolio_impact || briefing.portfolio_impact.length === 0 ? (
                    <p className="text-sm text-muted-foreground">暂无持仓影响</p>
                  ) : (
                    <div className="overflow-x-auto">
                      <table className="w-full text-sm">
                        <thead>
                          <tr className="border-b border-border">
                            <th className="text-left py-2 text-muted-foreground">资产</th>
                            <th className="text-left py-2 text-muted-foreground">方向</th>
                            <th className="text-left py-2 text-muted-foreground">置信度</th>
                            <th className="text-left py-2 text-muted-foreground">紧急度</th>
                            <th className="text-left py-2 text-muted-foreground">建议动作</th>
                          </tr>
                        </thead>
                        <tbody>
                          {briefing.portfolio_impact.map((row, idx) => (
                            <tr key={idx} className="border-b border-border">
                              <td className="py-2 text-foreground">{row.asset}</td>
                              <td className="py-2">
                                <span
                                  className={`px-2 py-0.5 rounded ${
                                    row.net_direction === "bullish"
                                      ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
                                      : row.net_direction === "bearish"
                                      ? "bg-destructive/10 text-destructive"
                                      : "bg-muted text-muted-foreground"
                                  }`}
                                >
                                  {row.net_direction}
                                </span>
                              </td>
                              <td className="py-2 text-muted-foreground">
                                {(row.confidence * 100).toFixed(0)}%
                              </td>
                              <td className="py-2 text-muted-foreground">{row.urgency}</td>
                              <td className="py-2 text-muted-foreground">{row.action}</td>
                            </tr>
                          ))}
                        </tbody>
                      </table>
                    </div>
                  )}
                </CardBody>
              </Card>

              {/* Module 3: Opportunities */}
              {briefing.opportunities && briefing.opportunities.length > 0 && (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-lg">模块三：机会发现</CardTitle>
                  </CardHeader>
                  <CardBody>
                    <div className="space-y-3">
                      {briefing.opportunities.map((opp, idx) => (
                        <div key={idx} className="p-3 bg-emerald-500/5 rounded-lg border border-emerald-500/20">
                          <div className="flex items-center gap-2 mb-2">
                            <span className="font-medium text-foreground">{opp.asset}</span>
                            <span
                              className={`px-2 py-0.5 rounded text-xs ${
                                opp.direction === "bullish"
                                  ? "bg-emerald-500/10 text-emerald-600 dark:text-emerald-400"
                                  : "bg-muted text-muted-foreground"
                              }`}
                            >
                              {opp.direction}
                            </span>
                            <span className="text-xs text-muted-foreground">
                              来源: {opp.source}
                            </span>
                          </div>
                          <p className="text-sm text-muted-foreground">{opp.mechanism}</p>
                        </div>
                      ))}
                    </div>
                  </CardBody>
                </Card>
              )}

              {/* Module 4: Risk Alerts */}
              {briefing.risk_alerts && briefing.risk_alerts.length > 0 && (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-lg">模块四：风险预警</CardTitle>
                  </CardHeader>
                  <CardBody>
                    <div className="space-y-3">
                      {briefing.risk_alerts.map((alert, idx) => (
                        <div
                          key={idx}
                          className={`p-3 rounded-lg ${
                            alert.level === "critical"
                              ? "bg-destructive/5 border border-destructive/20"
                              : "bg-warning/5 border border-warning/20"
                          }`}
                        >
                          <div className="flex items-center gap-2 mb-2">
                            <span
                              className={`px-2 py-0.5 rounded text-xs ${
                                alert.level === "critical"
                                  ? "bg-destructive/10 text-destructive"
                                  : "bg-warning/10 text-warning"
                              }`}
                            >
                              {alert.level}
                            </span>
                          </div>
                          <p className="text-sm text-foreground">{alert.description}</p>
                          <p className="text-xs text-muted-foreground mt-1">
                            建议动作: {alert.action}
                          </p>
                        </div>
                      ))}
                    </div>
                  </CardBody>
                </Card>
              )}

              {/* Module 5: Conflict Signals */}
              {briefing.conflict_signals && briefing.conflict_signals.length > 0 && (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-lg">模块五：冲突信号</CardTitle>
                  </CardHeader>
                  <CardBody>
                    <div className="space-y-3">
                      {briefing.conflict_signals.map((signal, idx) => (
                        <div key={idx} className="p-3 bg-warning/5 rounded-lg border border-warning/20">
                          <div className="font-medium mb-2 text-foreground">{signal.asset}</div>
                          <div className="grid grid-cols-2 gap-2 text-sm">
                            <div className="text-emerald-600 dark:text-emerald-400">
                              利好: {signal.bullish_event}
                            </div>
                            <div className="text-destructive">
                              利空: {signal.bearish_event}
                            </div>
                          </div>
                          <p className="text-sm text-muted-foreground mt-2">
                            {signal.analysis}
                          </p>
                        </div>
                      ))}
                    </div>
                  </CardBody>
                </Card>
              )}

              {/* Module 6: Monitoring Items */}
              {briefing.monitoring_items && briefing.monitoring_items.length > 0 && (
                <Card>
                  <CardHeader>
                    <CardTitle className="text-lg">模块六：持续监控</CardTitle>
                  </CardHeader>
                  <CardBody>
                    <div className="space-y-2">
                      {briefing.monitoring_items.map((item, idx) => (
                        <div key={idx} className="p-2 bg-muted/30 rounded-lg text-sm">
                          <div className="font-medium text-foreground">{item.signal}</div>
                          <div className="text-muted-foreground text-xs mt-1">
                            阈值: {item.threshold} | 原因: {item.reason}
                          </div>
                        </div>
                      ))}
                    </div>
                  </CardBody>
                </Card>
              )}
            </div>
          ) : (
            <div className="text-center py-12">
              <div className="text-muted-foreground">暂无简报</div>
              <Button onClick={handleAnalyze} disabled={analyzing} className="mt-4">
                触发分析生成简报
              </Button>
            </div>
          )}
        </>
      )}
    </div>
  );
}
