create table if not exists news_items (
  id integer primary key autoincrement,
  source text not null,
  title text not null,
  link text not null,
  guid text not null,
  published_at text,
  content text not null default '',
  fetched_at text not null default CURRENT_TIMESTAMP
);

create unique index if not exists idx_news_items_source_guid on news_items (source, guid);
create index if not exists idx_news_items_published_at on news_items (published_at desc);

create table if not exists news_briefs (
  id integer primary key autoincrement,
  item_id bigint not null references news_items(id) on delete cascade,
  sentiment text not null,
  brief text not null,
  keywords text not null,
  created_at text not null default CURRENT_TIMESTAMP
);

create index if not exists idx_news_briefs_item_id on news_briefs (item_id);
create index if not exists idx_news_briefs_created_at on news_briefs (created_at desc);
