create table if not exists users (
	id integer primary key autoincrement,
	name text not null,
	created_at text not null default CURRENT_TIMESTAMP
);

create table if not exists accounts (
	id integer primary key autoincrement,
	user_id bigint not null references users(id),
	name text not null,
	base_currency text not null default 'CNY',
	created_at text not null default CURRENT_TIMESTAMP
);

create table if not exists instruments (
	id integer primary key autoincrement,
	symbol text not null unique,
	name text not null default '',
	asset_type text not null default 'unknown',
	created_at text not null default CURRENT_TIMESTAMP
);

create table if not exists positions (
	id integer primary key autoincrement,
	account_id bigint not null references accounts(id),
	instrument_id bigint not null references instruments(id),
	quantity numeric(20,8) not null default 0,
	avg_cost numeric(20,8) not null default 0,
	market_price numeric(20,8),
	created_at text not null default CURRENT_TIMESTAMP,
	updated_at text not null default CURRENT_TIMESTAMP,
	unique(account_id, instrument_id)
);

create table if not exists trades (
	id integer primary key autoincrement,
	account_id bigint not null references accounts(id),
	instrument_id bigint not null references instruments(id),
	side text not null,
	quantity numeric(20,8) not null,
	price numeric(20,8) not null,
	fee numeric(20,8) not null default 0,
	trade_at text not null default CURRENT_TIMESTAMP,
	note text not null default ''
);

create table if not exists cash_flows (
	id integer primary key autoincrement,
	account_id bigint not null references accounts(id),
	amount numeric(20,8) not null,
	currency text not null default 'CNY',
	flow_type text not null,
	flow_at text not null default CURRENT_TIMESTAMP,
	note text not null default ''
);

create table if not exists valuations (
	id integer primary key autoincrement,
	account_id bigint not null references accounts(id),
	total_value numeric(20,8) not null,
	total_cost numeric(20,8) not null,
	total_pnl numeric(20,8) not null,
	valuation_at text not null default CURRENT_TIMESTAMP
);

create index if not exists idx_accounts_user_id on accounts(user_id);
create index if not exists idx_positions_account_id on positions(account_id);
create index if not exists idx_trades_account_id on trades(account_id);
create index if not exists idx_trades_trade_at on trades(trade_at);
create index if not exists idx_cash_flows_account_id on cash_flows(account_id);
create index if not exists idx_valuations_account_id on valuations(account_id);
