package decision

import "context"

type MarketNewsProvider interface {
	GetMarketSummary(ctx context.Context) (string, error)
	GetNewsSummary(ctx context.Context) (string, error)
}

type EmptyMarketNewsProvider struct{}

func (p EmptyMarketNewsProvider) GetMarketSummary(ctx context.Context) (string, error) {
	return "", nil
}

func (p EmptyMarketNewsProvider) GetNewsSummary(ctx context.Context) (string, error) {
	return "", nil
}
