package decision

import (
	"context"
	"encoding/json"
	"strings"

	"genFu/internal/investment"
	"genFu/internal/news"
)

type BriefNewsProvider struct {
	repo      *news.Repository
	holdings  *investment.Repository
	accountID int64
	limit     int
	keywords  []string
}

func NewBriefNewsProvider(repo *news.Repository, holdings *investment.Repository, accountID int64, limit int, keywords []string) *BriefNewsProvider {
	if limit <= 0 {
		limit = 20
	}
	if accountID == 0 {
		accountID = 1
	}
	return &BriefNewsProvider{
		repo:      repo,
		holdings:  holdings,
		accountID: accountID,
		limit:     limit,
		keywords:  keywords,
	}
}

func (p *BriefNewsProvider) GetMarketSummary(ctx context.Context) (string, error) {
	_ = ctx
	return "", nil
}

func (p *BriefNewsProvider) GetNewsSummary(ctx context.Context) (string, error) {
	if p == nil || p.repo == nil {
		return "", nil
	}
	keywords := p.buildKeywords(ctx)
	if len(keywords) == 0 {
		return "", nil
	}
	briefs, err := p.repo.ListBriefsByKeywords(ctx, keywords, p.limit)
	if err != nil || len(briefs) == 0 {
		return "", err
	}
	payload, _ := json.Marshal(briefs)
	return string(payload), nil
}

func (p *BriefNewsProvider) buildKeywords(ctx context.Context) []string {
	seen := map[string]struct{}{}
	output := []string{}
	_ = ctx
	for _, kw := range p.keywords {
		kw = strings.TrimSpace(kw)
		if kw == "" {
			continue
		}
		if _, ok := seen[kw]; ok {
			continue
		}
		seen[kw] = struct{}{}
		output = append(output, kw)
	}
	return output
}
