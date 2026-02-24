package news

import (
	"context"
	"strings"

	"genFu/internal/investment"
)

// BuildPortfolioContext builds portfolio context from investment repository
func BuildPortfolioContext(ctx context.Context, investRepo *investment.Repository) (*PortfolioContext, error) {
	if investRepo == nil {
		return &PortfolioContext{}, nil
	}

	// Get default account
	accountID, err := investRepo.DefaultAccountID(ctx)
	if err != nil {
		accountID = 1
	}

	// Get positions
	positions, err := investRepo.ListPositions(ctx, accountID)
	if err != nil {
		return &PortfolioContext{}, err
	}

	// Build holdings
	holdings := make([]Holding, 0, len(positions))
	for _, p := range positions {
		holding := Holding{
			Name:     p.Instrument.Name,
			Code:     p.Instrument.Symbol,
			Industry: p.Instrument.Industry,
			Weight:   0,
		}

		// Calculate weight based on position value
		if p.MarketPrice != nil && *p.MarketPrice > 0 {
			holding.Weight = p.Quantity * *p.MarketPrice
		} else {
			holding.Weight = p.Quantity * p.AvgCost
		}

		// Extract products/competitors/supply_chain from instrument metadata
		if p.Instrument.Products != nil {
			holding.Products = p.Instrument.Products
		}
		if p.Instrument.Competitors != nil {
			holding.Competitors = p.Instrument.Competitors
		}
		if p.Instrument.SupplyChain != nil {
			holding.SupplyChain = p.Instrument.SupplyChain
		}

		holdings = append(holdings, holding)
	}

	// Normalize weights to sum to 1
	totalWeight := 0.0
	for _, h := range holdings {
		totalWeight += h.Weight
	}
	if totalWeight > 0 {
		for i := range holdings {
			holdings[i].Weight = holdings[i].Weight / totalWeight
		}
	}

	// Build watchlist (could be from a separate table or config)
	watchlist := []WatchItem{} // TODO: Implement watchlist from config or DB

	// Build industry themes (could be from config)
	industryThemes := []string{} // TODO: Implement from config

	// Build macro factors (could be from config)
	macroFactors := []string{
		"美联储利率政策",
		"中国经济政策",
		"地缘政治风险",
	}

	return &PortfolioContext{
		Holdings:       holdings,
		Watchlist:      watchlist,
		IndustryThemes: industryThemes,
		MacroFactors:   macroFactors,
	}, nil
}

// BuildPortfolioContextFromConfig builds portfolio context from static configuration
func BuildPortfolioContextFromConfig(holdings []Holding, watchlist []WatchItem, themes []string, factors []string) *PortfolioContext {
	return &PortfolioContext{
		Holdings:       holdings,
		Watchlist:      watchlist,
		IndustryThemes: themes,
		MacroFactors:   factors,
	}
}

// isHolding checks if asset is a holding
func isHolding(portfolio *PortfolioContext, assetName string) bool {
	if portfolio == nil {
		return false
	}
	name := strings.ToLower(assetName)
	for _, h := range portfolio.Holdings {
		if strings.Contains(strings.ToLower(h.Name), name) || strings.Contains(name, strings.ToLower(h.Name)) {
			return true
		}
		if strings.Contains(strings.ToLower(h.Code), name) || strings.Contains(name, strings.ToLower(h.Code)) {
			return true
		}
	}
	return false
}
