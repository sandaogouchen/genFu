with u as (
	insert into users(name) values ('示例用户') returning id
),
a as (
	insert into accounts(user_id, name, base_currency)
	select id, '示例账户', 'CNY' from u
	returning id
),
i as (
	insert into instruments(symbol, name, asset_type)
	values ('600519', '贵州茅台', 'stock')
	on conflict(symbol) do update set name = excluded.name, asset_type = excluded.asset_type
	returning id
),
p as (
	insert into positions(account_id, instrument_id, quantity, avg_cost, market_price)
	select a.id, i.id, 100, 1200, 1500 from a, i
	returning account_id
)
insert into trades(account_id, instrument_id, side, quantity, price, fee, trade_at, note)
select a.id, i.id, 'buy', 100, 1200, 5, CURRENT_TIMESTAMP, '示例买入'
from a, i;

insert into cash_flows(account_id, amount, currency, flow_type, flow_at, note)
select a.id, 120000, 'CNY', 'deposit', CURRENT_TIMESTAMP, '示例入金'
from accounts a
order by a.id desc
limit 1;
