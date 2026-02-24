create table if not exists analyze_reports (
  id integer primary key autoincrement,
  report_type text not null,
  symbol text not null,
  name text not null default '',
  request text not null,
  steps text not null,
  summary text not null,
  created_at text not null default CURRENT_TIMESTAMP
);

create index if not exists idx_analyze_reports_symbol_created_at on analyze_reports (symbol, created_at desc);
