package main

import (
	"context"

	"genFu/internal/llm"
	"genFu/internal/news"
)

// EmbeddingClassifierAdapter wraps llm.EmbeddingService to implement news.EmbeddingClassifier
type EmbeddingClassifierAdapter struct {
	svc *llm.EmbeddingService
}

func NewEmbeddingClassifierAdapter(svc *llm.EmbeddingService) *EmbeddingClassifierAdapter {
	return &EmbeddingClassifierAdapter{svc: svc}
}

func (a *EmbeddingClassifierAdapter) Classify(ctx context.Context, title, summary string) ([]news.EventDomain, []string, float64, error) {
	domains, types, conf, err := a.svc.Classify(ctx, title, summary)
	if err != nil {
		return nil, nil, 0, err
	}
	// Convert llm.EventDomain to news.EventDomain
	newsDomains := make([]news.EventDomain, len(domains))
	for i, d := range domains {
		newsDomains[i] = news.EventDomain(d)
	}
	return newsDomains, types, conf, nil
}

// LLMLabelServiceAdapter wraps llm.LLMServiceAdapter to implement news.LLMLabelService
type LLMLabelServiceAdapter struct {
	adapter *llm.LLMServiceAdapter
}

func NewLLMLabelServiceAdapter(adapter *llm.LLMServiceAdapter) *LLMLabelServiceAdapter {
	return &LLMLabelServiceAdapter{adapter: adapter}
}

func (a *LLMLabelServiceAdapter) Label(ctx context.Context, title, summary string, domains []news.EventDomain) (*news.LabelSet, error) {
	// Convert news.EventDomain to llm.EventDomain
	llmDomains := make([]llm.EventDomain, len(domains))
	for i, d := range domains {
		llmDomains[i] = llm.EventDomain(d)
	}

	labels, err := a.adapter.Label(ctx, title, summary, llmDomains)
	if err != nil {
		return nil, err
	}

	// Convert llm.LabelSet to news.LabelSet
	return &news.LabelSet{
		Sentiment:      labels.Sentiment,
		Novelty:        news.Novelty(labels.Novelty),
		Predictability: news.Predictability(labels.Predictability),
		Timeframe:      news.Timeframe(labels.Timeframe),
		Entities:       convertEntities(labels.Entities),
	}, nil
}

func convertEntities(entities []string) []news.EntityLabel {
	if entities == nil {
		return nil
	}
	result := make([]news.EntityLabel, len(entities))
	for i, e := range entities {
		result[i] = news.EntityLabel{Name: e}
	}
	return result
}
