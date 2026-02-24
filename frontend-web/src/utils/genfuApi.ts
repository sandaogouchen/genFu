import { getApiKeyConfig } from "@/utils/settings";
import { parseSSEStream, SSEMessage } from "@/utils/sse";

export type ToolResult = {
  name?: string;
  output?: unknown;
  error?: string;
};

export type AnalyzeRequest = {
  type: "fund" | "stock";
  symbol: string;
  name?: string;
  kline?: string;
  manager?: string;
  meta?: Record<string, string>;
};

export type AnalyzeStep = {
  name?: string;
  input?: string;
  output?: string;
  tool_results?: ToolResult[];
};

export type AnalyzeResponse = {
  type?: string;
  symbol?: string;
  name?: string;
  steps?: AnalyzeStep[];
  summary?: string;
  report_id?: number;
};

// Report types
export type ReportListItem = {
  id: number;
  report_type: string;
  symbol: string;
  name: string;
  title: string;
  created_at: string;
};

export type ReportListResponse = {
  items: ReportListItem[];
  total: number;
  page: number;
  page_size: number;
};

export type ReportDetail = {
  id: number;
  report_type: string;
  symbol: string;
  name: string;
  title: string;
  request: AnalyzeRequest;
  steps: AnalyzeStep[];
  summary: string;
  created_at: string;
};

export type DecisionRequest = {
  report_ids?: number[];
  meta?: Record<string, string>;
};

export type DecisionItem = {
  account_id?: number;
  symbol?: string;
  name?: string;
  asset_type?: string;
  action?: string;
  quantity?: number;
  price?: number;
  confidence?: number;
  valid_until?: string;
  reason?: string;
};

export type DecisionOutput = {
  decision_id?: string;
  market_view?: string;
  risk_notes?: string;
  decisions?: DecisionItem[];
};

export type TradeSignal = DecisionItem & { decision_id?: string };

export type ExecutionResult = {
  signal?: TradeSignal;
  trade?: {
    id: number;
    price: number;
    quantity: number;
  };
  position?: {
    quantity: number;
    average_cost: number;
  };
  error?: string;
  status?: string;
};

export type DecisionResponse = {
  decision?: DecisionOutput;
  raw?: string;
  signals?: TradeSignal[];
  executions?: ExecutionResult[];
  tool_results?: ToolResult[];
};

export type StockWorkflowInput = {
  symbol: string;
  name?: string;
  stock_news_routes?: string[];
  industry_news_routes?: string[];
  news_limit?: number;
};

export type HoldingPosition = {
  symbol?: string;
  name?: string;
  quantity?: number;
  avg_cost?: number;
  market_price?: number;
  value?: number;
  ratio?: number;
};

export type HoldingsOutput = {
  positions?: HoldingPosition[];
  total_value?: number;
};

export type MarketMove = {
  symbol?: string;
  name?: string;
  price?: number;
  change?: number;
  change_rate?: number;
  amount?: number;
  error?: string;
};

export type NewsSummaryOutput = {
  summary?: string;
  sentiment?: string;
};

export type StockWorkflowOutput = {
  holdings?: HoldingsOutput;
  holdings_market?: MarketMove[];
  target_market?: MarketMove;
  news?: NewsSummaryOutput;
  bull_analysis?: string;
  bear_analysis?: string;
  debate_analysis?: string;
  summary?: string;
};

export type InvestmentRequest = {
  action: string;
  [k: string]: unknown;
};

export type MarketDataRequest = {
  action: string;
  [k: string]: unknown;
};

export type ChatMessage = {
  role: "system" | "user" | "assistant" | "tool";
  content: string;
  name?: string;
  tool_call_id?: string;
};

export type GenerateRequest = {
  session_id?: string;
  messages: ChatMessage[];
  meta?: Record<string, string>;
};

export type GenerateEvent = {
  type: string;
  delta?: string;
  message?: ChatMessage;
  tool_call?: unknown;
  tool_result?: ToolResult;
  done?: boolean;
};

function authHeaders() {
  const { key, mode } = getApiKeyConfig();
  if (!key) return {} as Record<string, string>;
  if (mode === "x-api-key") return { "X-Api-Key": key };
  if (mode === "x-goog-api-key") return { "X-Goog-Api-Key": key };
  return { Authorization: `Bearer ${key}` };
}

export async function postJson<T>(path: string, body: unknown, init?: { signal?: AbortSignal }): Promise<T> {
  const resp = await fetch(path, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...authHeaders(),
    },
    body: JSON.stringify(body),
    signal: init?.signal,
  });
  if (!resp.ok) {
    const msg = await resp.text().catch(() => "");
    throw new Error(msg || `HTTP ${resp.status}`);
  }
  return (await resp.json()) as T;
}

export async function getJson<T>(path: string, init?: { signal?: AbortSignal }): Promise<T> {
  const resp = await fetch(path, {
    method: "GET",
    headers: {
      ...authHeaders(),
    },
    signal: init?.signal,
  });
  if (!resp.ok) {
    const msg = await resp.text().catch(() => "");
    throw new Error(msg || `HTTP ${resp.status}`);
  }
  return (await resp.json()) as T;
}

export async function* postSSE(path: string, body: unknown, init?: { signal?: AbortSignal }): AsyncGenerator<SSEMessage> {
  const resp = await fetch(path, {
    method: "POST",
    headers: {
      "Content-Type": "application/json",
      ...authHeaders(),
    },
    body: JSON.stringify(body),
    signal: init?.signal,
  });
  if (!resp.ok) {
    const msg = await resp.text().catch(() => "");
    throw new Error(msg || `HTTP ${resp.status}`);
  }
  yield* parseSSEStream(resp, init?.signal);
}

// ───────────────���──────────────────────────────
// News Event Types
// ──────────────────────────────────────────────

export type EventDomain =
  | "macro"
  | "geopolitical"
  | "industry"
  | "corporate"
  | "regulatory"
  | "market"
  | "technology";

export type Novelty = "breaking" | "follow_up" | "recurring" | "old_news";
export type Predictability = "scheduled" | "unscheduled" | "semi_known";
export type Timeframe = "immediate" | "short" | "medium" | "long";
export type Direction = "bullish" | "bearish" | "mixed" | "uncertain";
export type Relevance = "high" | "medium" | "low" | "none";
export type TriggerType = "pre_market" | "intraday" | "breaking" | "manual";

export type EntityLabel = {
  name: string;
  code?: string;
  role: "primary" | "secondary" | "mentioned";
  relevance: number;
};

export type LabelSet = {
  sentiment: number;
  novelty: Novelty;
  predictability: Predictability;
  timeframe: Timeframe;
  entities: EntityLabel[];
};

export type AnchorMatch = {
  anchor_id: string;
  anchor_text: string;
  anchor_type: string;
  similarity: number;
  weighted_score: number;
  related_asset: string;
};

export type AssetImpact = {
  asset_name: string;
  asset_code?: string;
  direction: Direction;
  confidence: number;
  is_holding: boolean;
  causal_sketch?: string;
};

export type CausalHop = {
  from: string;
  to: string;
  mechanism: string;
  confidence: number;
  evidence: string;
  time_lag: string;
};

export type CausalChain = {
  hops: CausalHop[];
  final_impact?: {
    asset: string;
    direction: Direction;
    magnitude: string;
    timeframe: string;
  };
};

export type CausalAnalysis = {
  event_summary: string;
  causal_chains: CausalChain[];
  counter_chains: CausalChain[];
  key_uncertainties: string[];
  monitoring_signals: string[];
  cross_asset_impacts: {
    asset: string;
    asset_code?: string;
    direction: Direction;
    mechanism: string;
    confidence: number;
    is_holding: boolean;
    source_event: string;
  }[];
};

export type FunnelResult = {
  l1_score: number;
  l1_matched_anchors?: AnchorMatch[];
  l1_pass: boolean;
  l2_relevance: Relevance;
  l2_affected_assets?: AssetImpact[];  // 可能为 null
  l2_causal_sketch?: string;
  l2_priority: number;
  l2_needs_deep: boolean;
  l2_pass: boolean;
  l3_analysis?: CausalAnalysis;
};

export type NewsEvent = {
  id: string;  // 后端返回字符串类型的ID
  title: string;
  summary: string;
  content: string;
  source: string;
  source_type: string;
  url: string;
  published_at: string;
  processed_at: string;
  domains?: EventDomain[];  // 可能为 null
  event_types?: string[];   // 可能为 null
  labels?: LabelSet;        // 可能为 null
  classify_confidence: number;
  classify_method: string;
  dedup_cluster_id?: string;
  related_sources?: string[];
  funnel_result?: FunnelResult;
};

export type NewsEventQuery = {
  page?: number;
  page_size?: number;
  domain?: string;
  event_type?: string;
  sentiment?: string;
  date_from?: string;
  date_to?: string;
  keywords?: string;
  source_type?: string;
  min_priority?: number;
  sort_by?: string;
};

export type NewsEventListResponse = {
  items: NewsEvent[];
  total: number;
  page: number;
  page_size: number;
};

// ──────────────────────────────────────────────
// Briefing Types
// ──────────────────────────────────────────────

export type MacroFactor = {
  factor: string;
  direction: Direction;
  impact: string;
};

export type MacroOverview = {
  summary: string;
  market_sentiment: string;
  key_factors: MacroFactor[];
  risk_level: string;
};

export type PortfolioImpactRow = {
  asset: string;
  code: string;
  related_events: string[];
  net_direction: Direction;
  confidence: number;
  urgency: string;
  action: string;
  key_causal: string;
};

export type OpportunityAlert = {
  source: string;
  asset: string;
  asset_code?: string;
  direction: Direction;
  mechanism: string;
  confidence: number;
  source_events: string[];
};

export type RiskAlert = {
  level: string;
  description: string;
  assets: string[];
  events: string[];
  action: string;
};

export type ConflictSignal = {
  asset: string;
  bullish_event: string;
  bearish_event: string;
  analysis: string;
};

export type MonitoringItem = {
  signal: string;
  threshold: string;
  assets: string[];
  reason: string;
};

export type Briefing = {
  id: string;
  generated_at: string;
  trigger_type: TriggerType;
  period: string;
  macro_overview?: MacroOverview;        // 可能为 null
  portfolio_impact?: PortfolioImpactRow[];  // 可能为 null
  opportunities?: OpportunityAlert[];    // 可能为 null
  risk_alerts?: RiskAlert[];             // 可能为 null
  conflict_signals?: ConflictSignal[];   // 可能为 null
  monitoring_items?: MonitoringItem[];   // 可能为 null
  total_news_processed: number;
  l1_passed: number;
  l2_passed: number;
  l3_analyzed: number;
};

export type PipelineRunResult = {
  briefing?: Briefing;
  total_collected: number;
  total_tagged: number;
  l1_passed: number;
  l2_passed: number;
  l3_analyzed: number;
  duration: number;
  error?: string;
};

// ──────────────────────────────────────────────
// News API Functions
// ──────────────────────────────────────────────

export async function listNewsEvents(query: NewsEventQuery): Promise<NewsEventListResponse> {
  const params = new URLSearchParams();
  if (query.page) params.set("page", String(query.page));
  if (query.page_size) params.set("page_size", String(query.page_size));
  if (query.domain) params.set("domain", query.domain);
  if (query.event_type) params.set("event_type", query.event_type);
  if (query.sentiment) params.set("sentiment", query.sentiment);
  if (query.date_from) params.set("date_from", query.date_from);
  if (query.date_to) params.set("date_to", query.date_to);
  if (query.keywords) params.set("keywords", query.keywords);
  if (query.source_type) params.set("source_type", query.source_type);
  if (query.min_priority) params.set("min_priority", String(query.min_priority));
  if (query.sort_by) params.set("sort_by", query.sort_by);

  const queryString = params.toString();
  const path = queryString ? `/api/news/events?${queryString}` : "/api/news/events";
  return getJson<NewsEventListResponse>(path);
}

export async function getNewsEvent(id: number): Promise<NewsEvent> {
  return getJson<NewsEvent>(`/api/news/events/${id}`);
}

export async function getNewsBriefing(): Promise<Briefing> {
  return getJson<Briefing>("/api/news/briefing");
}

export async function triggerNewsAnalysis(): Promise<PipelineRunResult> {
  return postJson<PipelineRunResult>("/api/news/analyze", {});
}

// ──────────────────────────────────────────────
// Stock Picker Types
// ──────────────────────────────────────────────

export type StockPickRequest = {
  account_id?: number;
  top_n?: number;
  date_from?: string;
  date_to?: string;
};

export type TechnicalReason = {
  trend: string;
  volume_signal: string;
  technical_indicators: string[];
  key_levels: string[];
  risk_points: string[];
};

export type Allocation = {
  suggested_weight: number;
  industry_diversity: number;
  risk_exposure: number;
  liquidity_score: number;
  correlation_with_holding: number;
};

export type StockPick = {
  symbol: string;
  name: string;
  industry: string;
  current_price: number;
  recommendation: "buy" | "watch";
  confidence: number;
  technical_reasons: TechnicalReason;
  risk_level: "low" | "medium" | "high";
  allocation: Allocation;
};

export type IndexQuote = {
  code: string;
  name: string;
  price: number;
  change: number;
  change_rate: number;
  amount?: number;
};

export type MarketData = {
  index_quotes: IndexQuote[];
  market_sentiment: string;
  up_count: number;
  down_count: number;
  limit_up: number;
  limit_down: number;
};

export type StockPickResponse = {
  pick_id: string;
  generated_at: string;
  stocks: StockPick[];
  market_data: MarketData;
  news_summary: string;
  warnings?: string[];
};

export async function pickStocks(request: StockPickRequest): Promise<StockPickResponse> {
  return postJson<StockPickResponse>("/api/stockpicker", request);
}
