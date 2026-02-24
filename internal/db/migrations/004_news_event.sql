-- News events table
create table if not exists news_events (
  id integer primary key autoincrement,
  raw_item_id bigint,
  title text not null,
  summary text not null default '',
  content text not null default '',
  source text not null,
  source_type text not null default 'financial_media',
  url text,
  published_at text,
  processed_at text not null default CURRENT_TIMESTAMP,
  domains text not null default '[]',
  event_types text not null default '[]',
  labels text not null default '{}',
  classify_confidence real not null default 0.0,
  classify_method text not null default '',
  dedup_cluster_id text,
  related_sources text default '[]',
  funnel_result text,
  created_at text not null default CURRENT_TIMESTAMP
);

create index if not exists idx_news_events_published_at on news_events (published_at desc);
create index if not exists idx_news_events_processed_at on news_events (processed_at desc);
create index if not exists idx_news_events_source_type on news_events (source_type);
create index if not exists idx_news_events_domains on news_events (domains);
create index if not exists idx_news_events_dedup_cluster_id on news_events (dedup_cluster_id);

-- News anchors pool table
create table if not exists news_anchors (
  id integer primary key autoincrement,
  type text not null,
  text text not null,
  embedding blob,
  weight real not null default 0.5,
  related_asset text,
  updated_at text not null default CURRENT_TIMESTAMP
);

create index if not exists idx_news_anchors_type on news_anchors (type);
create index if not exists idx_news_anchors_related_asset on news_anchors (related_asset);

-- News analysis briefings table
create table if not exists news_analysis_briefings (
  id integer primary key autoincrement,
  trigger_type text not null,
  period text,
  generated_at text not null default CURRENT_TIMESTAMP,
  macro_overview text,
  portfolio_impact text,
  opportunities text,
  risk_alerts text,
  conflict_signals text,
  monitoring_items text,
  stats text,
  created_at text not null default CURRENT_TIMESTAMP
);

create index if not exists idx_news_analysis_briefings_generated_at on news_analysis_briefings (generated_at desc);
create index if not exists idx_news_analysis_briefings_trigger_type on news_analysis_briefings (trigger_type);
