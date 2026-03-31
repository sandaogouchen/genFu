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
  session_id?: string;
  session_title?: string;
  prompt?: string;
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

export type DecisionRequest = {
  account_id?: number;
  report_ids?: number[];
  guide_selections?: GuideSelection[];
  meta?: Record<string, string>;
  session_id?: string;
  session_title?: string;
  prompt?: string;
};

export type GuideSelection = {
  symbol: string;
  guide_id: number;
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

export type RiskBudget = {
  max_single_order_ratio?: number;
  max_symbol_exposure_ratio?: number;
  max_daily_trade_ratio?: number;
  min_confidence?: number;
};

export type PlannedOrder = {
  order_id?: string;
  account_id?: number;
  symbol?: string;
  name?: string;
  asset_type?: string;
  action?: string;
  quantity?: number;
  price?: number;
  notional?: number;
  confidence?: number;
  decision_id?: string;
  planning_reason?: string;
};

export type GuardedOrder = PlannedOrder & {
  guard_status?: string;
  guard_reason?: string;
  execution_status?: string;
  execution_error?: string;
  trade_id?: number;
};

export type ReviewAttribution = {
  order_id?: string;
  title?: string;
  detail?: string;
};

export type PostTradeReview = {
  summary?: string;
  attributions?: ReviewAttribution[];
  learning_points?: string[];
};

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
  run_id?: string;
  risk_budget?: RiskBudget;
  planned_orders?: PlannedOrder[];
  guarded_orders?: GuardedOrder[];
  review?: PostTradeReview;
  warnings?: string[];
};

export type StockWorkflowInput = {
  symbol: string;
  account_id?: number;
  name?: string;
  stock_news_routes?: string[];
  industry_news_routes?: string[];
  news_limit?: number;
  session_id?: string;
  session_title?: string;
  prompt?: string;
};

export type HoldingPosition = {
  symbol?: string;
  name?: string;
  asset_type?: string;
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

export type WorkflowNewsItem = {
  title?: string;
  link?: string;
  guid?: string;
  published_at?: string;
  description?: string;
};

export type NewsSummaryOutput = {
  items?: WorkflowNewsItem[];
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
  intent?: {
    intent: string;
    workflow: string;
    confidence: number;
    fallback?: boolean;
  };
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
export type ImpactLevel = "weak" | "moderate" | "strong";
export type VerificationVerdict = "passed" | "weak" | "invalid";
export type ExposureBucket = "holding" | "watchlist";
export type ExposureRelation = "direct" | "product" | "competitor" | "supply" | "theme" | "macro" | "unknown";
export type SignalOperator = "gt" | "gte" | "lt" | "lte" | "eq";

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

export type ImpactItem = {
  entity_name: string;
  entity_code?: string;
  direction: Direction;
  impact_score: number;
  impact_level: ImpactLevel;
  confidence: number;
  rationale?: string;
};

export type StructuredMonitoringSignal = {
  signal: string;
  metric?: string;
  operator?: SignalOperator;
  threshold?: string;
  window?: string;
  assets?: string[];
  reason?: string;
};

export type ImpactMapping = {
  event_summary: string;
  items?: ImpactItem[];
  monitoring_signals?: StructuredMonitoringSignal[];
};

export type PortfolioExposure = {
  asset_name: string;
  asset_code?: string;
  bucket: ExposureBucket;
  relation: ExposureRelation;
  direction: Direction;
  impact_score: number;
  exposure_score: number;
  position_weight?: number;
  confidence: number;
  rationale?: string;
};

export type CausalVerification = {
  verdict: VerificationVerdict;
  score: number;
  reason?: string;
  uncertains?: string[];
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
  event_entities?: EntityLabel[];
  impact_mapping?: ImpactMapping;
  exposure_mapping?: PortfolioExposure[];
  causal_verification?: CausalVerification;
  monitoring_signals_v2?: StructuredMonitoringSignal[];
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
  session_id?: string;
  session_title?: string;
  prompt?: string;
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
  operation_guide?: {
    stop_loss?: string;
    take_profit?: string;
  };
  trade_guide_text: string;
  trade_guide_json: string;
  trade_guide_version: string;
  risk_level: "low" | "medium" | "high";
  allocation: Allocation;
};

export type OperationGuideCondition = {
  type?: string;
  description?: string;
  value?: string;
};

export type OperationGuide = {
  id: number;
  symbol: string;
  pick_id?: string;
  buy_conditions?: OperationGuideCondition[];
  sell_conditions?: OperationGuideCondition[];
  stop_loss?: string;
  take_profit?: string;
  risk_monitors?: string[];
  trade_guide_text?: string;
  trade_guide_json?: string;
  trade_guide_version?: string;
  valid_until?: string;
  created_at?: string;
  updated_at?: string;
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

export type ConversationScene = "analyze" | "decision" | "stockpicker" | "workflow";

export type ConversationSession = {
  id: string;
  scene: ConversationScene;
  title: string;
  created_at: string;
  updated_at: string;
};

export type ConversationSessionsResponse = {
  items: ConversationSession[];
  limit: number;
  offset: number;
};

export type ConversationRun = {
  id: number;
  session_id: string;
  prompt: string;
  request: unknown;
  result?: unknown;
  error?: string;
  created_at: string;
};

export type ConversationRunsResponse = {
  items: ConversationRun[];
  limit: number;
};

export type ConversationSessionRetryOptions = {
  retries?: number;
  initialDelayMs?: number;
  maxDelayMs?: number;
  backoffFactor?: number;
  signal?: AbortSignal;
};

function isAbortError(err: unknown): boolean {
  return (
    (err instanceof DOMException && err.name === "AbortError") ||
    (err instanceof Error && err.name === "AbortError")
  );
}

async function waitForRetry(delayMs: number, signal?: AbortSignal): Promise<void> {
  if (delayMs <= 0) return;
  await new Promise<void>((resolve, reject) => {
    const timer = window.setTimeout(() => {
      signal?.removeEventListener("abort", onAbort);
      resolve();
    }, delayMs);
    const onAbort = () => {
      window.clearTimeout(timer);
      signal?.removeEventListener("abort", onAbort);
      reject(new DOMException("Aborted", "AbortError"));
    };
    signal?.addEventListener("abort", onAbort, { once: true });
  });
}

export async function pickStocks(request: StockPickRequest): Promise<StockPickResponse> {
  return postJson<StockPickResponse>("/api/stockpicker", request);
}

export async function listOperationGuides(symbol: string): Promise<OperationGuide[]> {
  const params = new URLSearchParams({ symbol });
  return getJson<OperationGuide[]>(`/api/operation-guides?${params.toString()}`);
}

export async function createConversationSession(payload: {
  scene: ConversationScene;
  title?: string;
}, options?: ConversationSessionRetryOptions): Promise<ConversationSession> {
  const retries = Math.max(0, Math.floor(options?.retries ?? 2));
  const initialDelayMs = Math.max(0, options?.initialDelayMs ?? 400);
  const maxDelayMs = Math.max(initialDelayMs, options?.maxDelayMs ?? 2600);
  const backoffFactor = Math.max(1, options?.backoffFactor ?? 2);
  let delayMs = initialDelayMs;

  for (let attempt = 0; attempt <= retries; attempt += 1) {
    try {
      return await postJson<ConversationSession>("/api/conversations/sessions", payload, { signal: options?.signal });
    } catch (err) {
      if (isAbortError(err) || attempt >= retries) {
        throw err;
      }
      await waitForRetry(delayMs, options?.signal);
      delayMs = Math.min(maxDelayMs, Math.max(delayMs + 1, Math.round(delayMs * backoffFactor)));
    }
  }

  throw new Error("create_session_retry_exhausted");
}

export async function listConversationSessions(
  scene: ConversationScene,
  limit = 50,
  offset = 0
): Promise<ConversationSessionsResponse> {
  const params = new URLSearchParams({
    scene,
    limit: String(limit),
    offset: String(offset),
  });
  return getJson<ConversationSessionsResponse>(`/api/conversations/sessions?${params.toString()}`);
}

export async function renameConversationSession(
  sessionId: string,
  title: string
): Promise<ConversationSession> {
  const resp = await fetch(`/api/conversations/sessions/${encodeURIComponent(sessionId)}`, {
    method: "PATCH",
    headers: {
      "Content-Type": "application/json",
      ...authHeaders(),
    },
    body: JSON.stringify({ title }),
  });
  if (!resp.ok) {
    const msg = await resp.text().catch(() => "");
    throw new Error(msg || `HTTP ${resp.status}`);
  }
  return (await resp.json()) as ConversationSession;
}

export async function deleteConversationSession(sessionId: string): Promise<void> {
  const resp = await fetch(`/api/conversations/sessions/${encodeURIComponent(sessionId)}`, {
    method: "DELETE",
    headers: {
      ...authHeaders(),
    },
  });
  if (!resp.ok) {
    const msg = await resp.text().catch(() => "");
    throw new Error(msg || `HTTP ${resp.status}`);
  }
}

export async function listConversationRuns(
  sessionId: string,
  limit = 100
): Promise<ConversationRunsResponse> {
  const params = new URLSearchParams({
    session_id: sessionId,
    limit: String(limit),
  });
  return getJson<ConversationRunsResponse>(`/api/conversations/runs?${params.toString()}`);
}
