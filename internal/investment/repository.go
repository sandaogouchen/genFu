package investment

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"

	"genFu/internal/db"
)

type Repository struct {
	db *db.DB
}

var ErrUserNotFound = errors.New("user_not_found")
var ErrAccountNotFound = errors.New("account_not_found")

func NewRepository(db *db.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) GetFirstUser(ctx context.Context) (User, error) {
	if r == nil || r.db == nil || r.db.DB == nil {
		return User{}, errors.New("db_not_initialized")
	}
	row := r.db.QueryRowContext(ctx, `select id, name, created_at from users order by id asc limit 1`)
	var u User
	var createdRaw sql.NullString
	if err := row.Scan(&u.ID, &u.Name, &createdRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return User{}, ErrUserNotFound
		}
		return User{}, err
	}
	if parsed, ok := db.ParseTime(createdRaw); ok {
		u.CreatedAt = parsed
	}
	return u, nil
}

func (r *Repository) GetFirstAccountByUserID(ctx context.Context, userID int64) (Account, error) {
	if r == nil || r.db == nil || r.db.DB == nil {
		return Account{}, errors.New("db_not_initialized")
	}
	row := r.db.QueryRowContext(ctx, `select id, user_id, name, base_currency, created_at from accounts where user_id = ? order by id asc limit 1`, userID)
	var a Account
	var createdRaw sql.NullString
	if err := row.Scan(&a.ID, &a.UserID, &a.Name, &a.BaseCurrency, &createdRaw); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Account{}, ErrAccountNotFound
		}
		return Account{}, err
	}
	if parsed, ok := db.ParseTime(createdRaw); ok {
		a.CreatedAt = parsed
	}
	return a, nil
}

func (r *Repository) EnsureDefaultAccount(ctx context.Context) (User, Account, error) {
	user, err := r.GetFirstUser(ctx)
	if err != nil {
		if !errors.Is(err, ErrUserNotFound) {
			return User{}, Account{}, err
		}
		user, err = r.CreateUser(ctx, "默认用户")
		if err != nil {
			return User{}, Account{}, err
		}
	}
	account, err := r.GetFirstAccountByUserID(ctx, user.ID)
	if err != nil {
		if !errors.Is(err, ErrAccountNotFound) {
			return User{}, Account{}, err
		}
		account, err = r.CreateAccount(ctx, user.ID, "默认账户", "CNY")
		if err != nil {
			return User{}, Account{}, err
		}
	}
	return user, account, nil
}

func (r *Repository) DefaultAccountID(ctx context.Context) (int64, error) {
	_, account, err := r.EnsureDefaultAccount(ctx)
	if err != nil {
		return 0, err
	}
	return account.ID, nil
}

func (r *Repository) CreateUser(ctx context.Context, name string) (User, error) {
	if strings.TrimSpace(name) == "" {
		return User{}, errors.New("empty_name")
	}
	row := r.db.QueryRowContext(ctx, `insert into users(name) values (?) returning id, name, created_at`, name)
	var u User
	var createdRaw sql.NullString
	if err := row.Scan(&u.ID, &u.Name, &createdRaw); err != nil {
		return User{}, err
	}
	if parsed, ok := db.ParseTime(createdRaw); ok {
		u.CreatedAt = parsed
	}
	return u, nil
}

func (r *Repository) CreateAccount(ctx context.Context, userID int64, name string, baseCurrency string) (Account, error) {
	if strings.TrimSpace(name) == "" {
		return Account{}, errors.New("empty_name")
	}
	if strings.TrimSpace(baseCurrency) == "" {
		baseCurrency = "CNY"
	}
	row := r.db.QueryRowContext(ctx, `insert into accounts(user_id, name, base_currency) values (?, ?, ?) returning id, user_id, name, base_currency, created_at`, userID, name, baseCurrency)
	var a Account
	var createdRaw sql.NullString
	if err := row.Scan(&a.ID, &a.UserID, &a.Name, &a.BaseCurrency, &createdRaw); err != nil {
		return Account{}, err
	}
	if parsed, ok := db.ParseTime(createdRaw); ok {
		a.CreatedAt = parsed
	}
	return a, nil
}

func (r *Repository) UpsertInstrument(ctx context.Context, symbol string, name string, assetType string) (Instrument, error) {
	if strings.TrimSpace(symbol) == "" {
		return Instrument{}, errors.New("empty_symbol")
	}
	if strings.TrimSpace(name) == "" {
		name = symbol
	}
	if strings.TrimSpace(assetType) == "" {
		assetType = "unknown"
	}
	row := r.db.QueryRowContext(ctx, `
		insert into instruments(symbol, name, asset_type)
		values (?, ?, ?)
		on conflict(symbol) do update set name = excluded.name, asset_type = excluded.asset_type
		returning id, symbol, name, asset_type, created_at
	`, symbol, name, assetType)
	var i Instrument
	var createdRaw sql.NullString
	if err := row.Scan(&i.ID, &i.Symbol, &i.Name, &i.AssetType, &createdRaw); err != nil {
		return Instrument{}, err
	}
	if parsed, ok := db.ParseTime(createdRaw); ok {
		i.CreatedAt = parsed
	}
	return i, nil
}

func (r *Repository) SetPosition(ctx context.Context, accountID int64, instrumentID int64, quantity float64, avgCost float64, marketPrice *float64) (Position, error) {
	row := r.db.QueryRowContext(ctx, `
		insert into positions(account_id, instrument_id, quantity, avg_cost, market_price, updated_at)
		values (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		on conflict(account_id, instrument_id) do update
		set quantity = excluded.quantity,
			avg_cost = excluded.avg_cost,
			market_price = excluded.market_price,
			updated_at = CURRENT_TIMESTAMP
		returning id, account_id, instrument_id, quantity, avg_cost, market_price, operation_guide_id, created_at, updated_at
	`, accountID, instrumentID, quantity, avgCost, marketPrice)
	var p Position
	var instrument Instrument
	var operationGuideID sql.NullInt64
	var createdRaw sql.NullString
	var updatedRaw sql.NullString
	if err := row.Scan(&p.ID, &p.AccountID, &instrument.ID, &p.Quantity, &p.AvgCost, &p.MarketPrice, &operationGuideID, &createdRaw, &updatedRaw); err != nil {
		return Position{}, err
	}
	if operationGuideID.Valid {
		v := operationGuideID.Int64
		p.OperationGuideID = &v
	}
	if parsed, ok := db.ParseTime(createdRaw); ok {
		p.CreatedAt = parsed
	}
	if parsed, ok := db.ParseTime(updatedRaw); ok {
		p.UpdatedAt = parsed
	}
	instrument, err := r.getInstrumentByID(ctx, instrument.ID)
	if err != nil {
		return Position{}, err
	}
	p.Instrument = instrument
	return p, nil
}

func (r *Repository) RecordTrade(ctx context.Context, accountID int64, instrumentID int64, side string, quantity float64, price float64, fee float64, tradeAt time.Time, note string) (Trade, error) {
	if strings.TrimSpace(side) == "" {
		return Trade{}, errors.New("empty_side")
	}
	if tradeAt.IsZero() {
		tradeAt = time.Now()
	}
	row := r.db.QueryRowContext(ctx, `
		insert into trades(account_id, instrument_id, side, quantity, price, fee, trade_at, note)
		values (?, ?, ?, ?, ?, ?, ?, ?)
		returning id, account_id, instrument_id, side, quantity, price, fee, trade_at, note
	`, accountID, instrumentID, side, quantity, price, fee, tradeAt, note)
	var t Trade
	var instrument Instrument
	var tradeAtRaw sql.NullString
	if err := row.Scan(&t.ID, &t.AccountID, &instrument.ID, &t.Side, &t.Quantity, &t.Price, &t.Fee, &tradeAtRaw, &t.Note); err != nil {
		return Trade{}, err
	}
	if parsed, ok := db.ParseTime(tradeAtRaw); ok {
		t.TradeAt = parsed
	}
	instrument, err := r.getInstrumentByID(ctx, instrument.ID)
	if err != nil {
		return Trade{}, err
	}
	t.Instrument = instrument
	return t, nil
}

func (r *Repository) RecordTradeAndUpdatePosition(ctx context.Context, accountID int64, instrumentID int64, side string, quantity float64, price float64, fee float64, tradeAt time.Time, note string) (Trade, Position, error) {
	if strings.TrimSpace(side) == "" {
		return Trade{}, Position{}, errors.New("empty_side")
	}
	if tradeAt.IsZero() {
		tradeAt = time.Now()
	}
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return Trade{}, Position{}, err
	}
	tradeRow := tx.QueryRowContext(ctx, `
		insert into trades(account_id, instrument_id, side, quantity, price, fee, trade_at, note)
		values (?, ?, ?, ?, ?, ?, ?, ?)
		returning id, account_id, instrument_id, side, quantity, price, fee, trade_at, note
	`, accountID, instrumentID, side, quantity, price, fee, tradeAt, note)
	var trade Trade
	var tradeAtRaw sql.NullString
	if err := tradeRow.Scan(&trade.ID, &trade.AccountID, &instrumentID, &trade.Side, &trade.Quantity, &trade.Price, &trade.Fee, &tradeAtRaw, &trade.Note); err != nil {
		_ = tx.Rollback()
		return Trade{}, Position{}, err
	}
	if parsed, ok := db.ParseTime(tradeAtRaw); ok {
		trade.TradeAt = parsed
	}

	var currentQty float64
	var currentAvg float64
	var currentMarket sql.NullFloat64
	err = tx.QueryRowContext(ctx, `
		select quantity, avg_cost, market_price
		from positions
		where account_id = ? and instrument_id = ?
	`, accountID, instrumentID).Scan(&currentQty, &currentAvg, &currentMarket)
	if err != nil && err != sql.ErrNoRows {
		_ = tx.Rollback()
		return Trade{}, Position{}, err
	}

	nextQty := currentQty
	nextAvg := currentAvg
	marketPrice := price
	if strings.ToLower(strings.TrimSpace(side)) == "buy" {
		totalCost := currentQty*currentAvg + quantity*price + fee
		nextQty = currentQty + quantity
		if nextQty > 0 {
			nextAvg = totalCost / nextQty
		} else {
			nextAvg = 0
		}
	} else {
		if quantity > currentQty {
			_ = tx.Rollback()
			return Trade{}, Position{}, errors.New("insufficient_quantity")
		}
		nextQty = currentQty - quantity
		if nextQty == 0 {
			nextAvg = 0
		}
	}

	positionRow := tx.QueryRowContext(ctx, `
		insert into positions(account_id, instrument_id, quantity, avg_cost, market_price, updated_at)
		values (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)
		on conflict(account_id, instrument_id) do update
		set quantity = excluded.quantity,
			avg_cost = excluded.avg_cost,
			market_price = excluded.market_price,
			updated_at = CURRENT_TIMESTAMP
		returning id, account_id, instrument_id, quantity, avg_cost, market_price, operation_guide_id, created_at, updated_at
	`, accountID, instrumentID, nextQty, nextAvg, marketPrice)
	var position Position
	var market sql.NullFloat64
	var operationGuideID sql.NullInt64
	var createdRaw sql.NullString
	var updatedRaw sql.NullString
	if err := positionRow.Scan(&position.ID, &position.AccountID, &instrumentID, &position.Quantity, &position.AvgCost, &market, &operationGuideID, &createdRaw, &updatedRaw); err != nil {
		_ = tx.Rollback()
		return Trade{}, Position{}, err
	}
	if parsed, ok := db.ParseTime(createdRaw); ok {
		position.CreatedAt = parsed
	}
	if parsed, ok := db.ParseTime(updatedRaw); ok {
		position.UpdatedAt = parsed
	}
	if market.Valid {
		v := market.Float64
		position.MarketPrice = &v
	}
	if operationGuideID.Valid {
		v := operationGuideID.Int64
		position.OperationGuideID = &v
	}

	if err := tx.Commit(); err != nil {
		return Trade{}, Position{}, err
	}

	instrument, err := r.getInstrumentByID(ctx, instrumentID)
	if err != nil {
		return Trade{}, Position{}, err
	}
	trade.Instrument = instrument
	position.Instrument = instrument
	return trade, position, nil
}

func (r *Repository) RecordCashFlow(ctx context.Context, accountID int64, amount float64, currency string, flowType string, flowAt time.Time, note string) (CashFlow, error) {
	if strings.TrimSpace(flowType) == "" {
		return CashFlow{}, errors.New("empty_flow_type")
	}
	if strings.TrimSpace(currency) == "" {
		currency = "CNY"
	}
	if flowAt.IsZero() {
		flowAt = time.Now()
	}
	row := r.db.QueryRowContext(ctx, `
		insert into cash_flows(account_id, amount, currency, flow_type, flow_at, note)
		values (?, ?, ?, ?, ?, ?)
		returning id, account_id, amount, currency, flow_type, flow_at, note
	`, accountID, amount, currency, flowType, flowAt, note)
	var f CashFlow
	var flowAtRaw sql.NullString
	if err := row.Scan(&f.ID, &f.AccountID, &f.Amount, &f.Currency, &f.FlowType, &flowAtRaw, &f.Note); err != nil {
		return CashFlow{}, err
	}
	if parsed, ok := db.ParseTime(flowAtRaw); ok {
		f.FlowAt = parsed
	}
	return f, nil
}

func (r *Repository) RecordValuation(ctx context.Context, accountID int64, totalValue float64, totalCost float64, valuationAt time.Time) (Valuation, error) {
	if valuationAt.IsZero() {
		valuationAt = time.Now()
	}
	totalPnL := totalValue - totalCost
	row := r.db.QueryRowContext(ctx, `
		insert into valuations(account_id, total_value, total_cost, total_pnl, valuation_at)
		values (?, ?, ?, ?, ?)
		returning id, account_id, total_value, total_cost, total_pnl, valuation_at
	`, accountID, totalValue, totalCost, totalPnL, valuationAt)
	var v Valuation
	var valuationRaw sql.NullString
	if err := row.Scan(&v.ID, &v.AccountID, &v.TotalValue, &v.TotalCost, &v.TotalPnL, &valuationRaw); err != nil {
		return Valuation{}, err
	}
	if parsed, ok := db.ParseTime(valuationRaw); ok {
		v.ValuationAt = parsed
	}
	return v, nil
}

func (r *Repository) ListPositions(ctx context.Context, accountID int64) ([]Position, error) {
	rows, err := r.db.QueryContext(ctx, `
		select p.id, p.account_id, p.instrument_id, p.quantity, p.avg_cost, p.market_price, p.operation_guide_id, p.created_at, p.updated_at
		from positions p
		where p.account_id = ?
		order by p.updated_at desc
	`, accountID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	positions := []Position{}
	for rows.Next() {
		var p Position
		var instrumentID int64
		var operationGuideID sql.NullInt64
		var createdRaw sql.NullString
		var updatedRaw sql.NullString
		if err := rows.Scan(&p.ID, &p.AccountID, &instrumentID, &p.Quantity, &p.AvgCost, &p.MarketPrice, &operationGuideID, &createdRaw, &updatedRaw); err != nil {
			return nil, err
		}
		if operationGuideID.Valid {
			v := operationGuideID.Int64
			p.OperationGuideID = &v
		}
		if parsed, ok := db.ParseTime(createdRaw); ok {
			p.CreatedAt = parsed
		}
		if parsed, ok := db.ParseTime(updatedRaw); ok {
			p.UpdatedAt = parsed
		}
		instrument, err := r.getInstrumentByID(ctx, instrumentID)
		if err != nil {
			return nil, err
		}
		p.Instrument = instrument
		positions = append(positions, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return positions, nil
}

func (r *Repository) GetPosition(ctx context.Context, accountID int64, instrumentID int64) (Position, error) {
	row := r.db.QueryRowContext(ctx, `
		select p.id, p.account_id, p.instrument_id, p.quantity, p.avg_cost, p.market_price, p.operation_guide_id, p.created_at, p.updated_at
		from positions p
		where p.account_id = ? and p.instrument_id = ?
	`, accountID, instrumentID)
	var p Position
	var operationGuideID sql.NullInt64
	var createdRaw sql.NullString
	var updatedRaw sql.NullString
	if err := row.Scan(&p.ID, &p.AccountID, &instrumentID, &p.Quantity, &p.AvgCost, &p.MarketPrice, &operationGuideID, &createdRaw, &updatedRaw); err != nil {
		return Position{}, err
	}
	if operationGuideID.Valid {
		v := operationGuideID.Int64
		p.OperationGuideID = &v
	}
	if parsed, ok := db.ParseTime(createdRaw); ok {
		p.CreatedAt = parsed
	}
	if parsed, ok := db.ParseTime(updatedRaw); ok {
		p.UpdatedAt = parsed
	}
	instrument, err := r.getInstrumentByID(ctx, instrumentID)
	if err != nil {
		return Position{}, err
	}
	p.Instrument = instrument
	return p, nil
}

func (r *Repository) DeletePosition(ctx context.Context, accountID int64, instrumentID int64) error {
	result, err := r.db.ExecContext(ctx, `
		delete from positions
		where account_id = ? and instrument_id = ?
	`, accountID, instrumentID)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return errors.New("position_not_found")
	}
	return nil
}

func (r *Repository) SetPositionOperationGuideBySymbol(ctx context.Context, accountID int64, symbol string, guideID int64) error {
	trimmedSymbol := strings.TrimSpace(symbol)
	if accountID == 0 {
		return errors.New("invalid_account_id")
	}
	if trimmedSymbol == "" {
		return errors.New("empty_symbol")
	}
	if guideID <= 0 {
		return errors.New("invalid_guide_id")
	}
	_, err := r.db.ExecContext(ctx, `
		update positions
		set operation_guide_id = ?, updated_at = CURRENT_TIMESTAMP
		where account_id = ?
		  and instrument_id in (
			  select id from instruments where symbol = ?
		  )
	`, guideID, accountID, trimmedSymbol)
	return err
}

func (r *Repository) EstimateAvailableCash(ctx context.Context, accountID int64) (float64, error) {
	if accountID == 0 {
		return 0, errors.New("invalid_account_id")
	}
	var flowSum sql.NullFloat64
	if err := r.db.QueryRowContext(ctx, `
		select
			coalesce(sum(
				case
					when lower(flow_type) in ('withdraw', 'withdrawal', 'out', 'transfer_out') then -amount
					else amount
				end
			), 0)
		from cash_flows
		where account_id = ?
	`, accountID).Scan(&flowSum); err != nil {
		return 0, err
	}

	var buyTotal sql.NullFloat64
	var sellTotal sql.NullFloat64
	if err := r.db.QueryRowContext(ctx, `
		select
			coalesce(sum(case when lower(side) = 'buy' then quantity * price + fee else 0 end), 0) as buy_total,
			coalesce(sum(case when lower(side) = 'sell' then quantity * price - fee else 0 end), 0) as sell_total
		from trades
		where account_id = ?
	`, accountID).Scan(&buyTotal, &sellTotal); err != nil {
		return 0, err
	}

	flow := 0.0
	if flowSum.Valid {
		flow = flowSum.Float64
	}
	buy := 0.0
	if buyTotal.Valid {
		buy = buyTotal.Float64
	}
	sell := 0.0
	if sellTotal.Valid {
		sell = sellTotal.Float64
	}
	return flow + sell - buy, nil
}

func (r *Repository) ListTrades(ctx context.Context, accountID int64, limit int, offset int) ([]Trade, error) {
	if limit <= 0 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}
	rows, err := r.db.QueryContext(ctx, `
		select t.id, t.account_id, t.instrument_id, t.side, t.quantity, t.price, t.fee, t.trade_at, t.note
		from trades t
		where t.account_id = ?
		order by t.trade_at desc
		limit ? offset ?
	`, accountID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	trades := []Trade{}
	for rows.Next() {
		var t Trade
		var instrumentID int64
		var tradeAtRaw sql.NullString
		if err := rows.Scan(&t.ID, &t.AccountID, &instrumentID, &t.Side, &t.Quantity, &t.Price, &t.Fee, &tradeAtRaw, &t.Note); err != nil {
			return nil, err
		}
		if parsed, ok := db.ParseTime(tradeAtRaw); ok {
			t.TradeAt = parsed
		}
		instrument, err := r.getInstrumentByID(ctx, instrumentID)
		if err != nil {
			return nil, err
		}
		t.Instrument = instrument
		trades = append(trades, t)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return trades, nil
}

func (r *Repository) GetPortfolioSummary(ctx context.Context, accountID int64) (PortfolioSummary, error) {
	var summary PortfolioSummary
	summary.AccountID = accountID
	err := r.db.QueryRowContext(ctx, `select count(*) from positions where account_id = ?`, accountID).Scan(&summary.PositionCount)
	if err != nil {
		return PortfolioSummary{}, err
	}
	err = r.db.QueryRowContext(ctx, `select count(*) from trades where account_id = ?`, accountID).Scan(&summary.TradeCount)
	if err != nil {
		return PortfolioSummary{}, err
	}
	var valuation Valuation
	var valuationRaw sql.NullString
	err = r.db.QueryRowContext(ctx, `
		select id, account_id, total_value, total_cost, total_pnl, valuation_at
		from valuations
		where account_id = ?
		order by valuation_at desc
		limit 1
	`, accountID).Scan(&valuation.ID, &valuation.AccountID, &valuation.TotalValue, &valuation.TotalCost, &valuation.TotalPnL, &valuationRaw)
	if err == nil {
		summary.TotalValue = valuation.TotalValue
		summary.TotalCost = valuation.TotalCost
		summary.TotalPnL = valuation.TotalPnL
		if parsed, ok := db.ParseTime(valuationRaw); ok {
			valuation.ValuationAt = parsed
			summary.ValuationAt = &valuation.ValuationAt
		}
		return summary, nil
	}
	if err != sql.ErrNoRows {
		return PortfolioSummary{}, err
	}
	var totalCost sql.NullFloat64
	var totalValue sql.NullFloat64
	err = r.db.QueryRowContext(ctx, `
		select
			sum(p.quantity * p.avg_cost) as total_cost,
			sum(p.quantity * coalesce(p.market_price, p.avg_cost)) as total_value
		from positions p
		where p.account_id = ?
	`, accountID).Scan(&totalCost, &totalValue)
	if err != nil {
		return PortfolioSummary{}, err
	}
	if totalCost.Valid {
		summary.TotalCost = totalCost.Float64
	}
	if totalValue.Valid {
		summary.TotalValue = totalValue.Float64
	} else {
		summary.TotalValue = summary.TotalCost
	}
	summary.TotalPnL = summary.TotalValue - summary.TotalCost
	return summary, nil
}

func (r *Repository) getInstrumentByID(ctx context.Context, id int64) (Instrument, error) {
	row := r.db.QueryRowContext(ctx, `select id, symbol, name, asset_type, created_at from instruments where id = ?`, id)
	var i Instrument
	var createdRaw sql.NullString
	if err := row.Scan(&i.ID, &i.Symbol, &i.Name, &i.AssetType, &createdRaw); err != nil {
		return Instrument{}, err
	}
	if parsed, ok := db.ParseTime(createdRaw); ok {
		i.CreatedAt = parsed
	}
	return i, nil
}

func (r *Repository) SearchInstruments(ctx context.Context, query string, limit int) ([]Instrument, error) {
	if limit <= 0 {
		limit = 20
	}
	searchPattern := "%" + strings.TrimSpace(query) + "%"
	rows, err := r.db.QueryContext(ctx, `
		select id, symbol, name, asset_type, created_at
		from instruments
		where symbol like ? or name like ?
		order by
			case when symbol like ? then 0 else 1 end,
			case when name like ? then 0 else 1 end,
			id asc
		limit ?
	`, searchPattern, searchPattern, searchPattern, searchPattern, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	instruments := []Instrument{}
	for rows.Next() {
		var i Instrument
		var createdRaw sql.NullString
		if err := rows.Scan(&i.ID, &i.Symbol, &i.Name, &i.AssetType, &createdRaw); err != nil {
			return nil, err
		}
		if parsed, ok := db.ParseTime(createdRaw); ok {
			i.CreatedAt = parsed
		}
		instruments = append(instruments, i)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return instruments, nil
}

func (r *Repository) GetInstrumentBySymbol(ctx context.Context, symbol string) (Instrument, error) {
	row := r.db.QueryRowContext(ctx, `select id, symbol, name, asset_type, created_at from instruments where symbol = ?`, symbol)
	var i Instrument
	var createdRaw sql.NullString
	if err := row.Scan(&i.ID, &i.Symbol, &i.Name, &i.AssetType, &createdRaw); err != nil {
		return Instrument{}, err
	}
	if parsed, ok := db.ParseTime(createdRaw); ok {
		i.CreatedAt = parsed
	}
	return i, nil
}

// ListValuations returns historical valuations for a given account,
// ordered by valuation_at ascending, limited to the last `days` days.
func (r *Repository) ListValuations(ctx context.Context, accountID int64, days int) ([]Valuation, error) {
	if days <= 0 {
		days = 30
	}
	cutoff := time.Now().AddDate(0, 0, -days)
	rows, err := r.db.QueryContext(ctx, `
		select id, account_id, total_value, total_cost, total_pnl, valuation_at
		from valuations
		where account_id = ? and valuation_at >= ?
		order by valuation_at asc
	`, accountID, cutoff)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var valuations []Valuation
	for rows.Next() {
		var v Valuation
		var valuationRaw sql.NullString
		if err := rows.Scan(&v.ID, &v.AccountID, &v.TotalValue, &v.TotalCost, &v.TotalPnL, &valuationRaw); err != nil {
			return nil, err
		}
		if parsed, ok := db.ParseTime(valuationRaw); ok {
			v.ValuationAt = parsed
		}
		valuations = append(valuations, v)
	}
	return valuations, rows.Err()
}
