package news

import (
	"context"
	"errors"
	"genFu/internal/rsshub"
)

type Service struct {
	repo      *Repository
	rsshub    *rsshub.Client
	generator BriefGenerator
	routes    []string
	maxItems  int
}

type BriefGenerator interface {
	Generate(ctx context.Context, item NewsItem) (string, string, []string, error)
}

func NewService(repo *Repository, rsshub *rsshub.Client, generator BriefGenerator, routes []string, maxItems int) *Service {
	return &Service{
		repo:      repo,
		rsshub:    rsshub,
		generator: generator,
		routes:    routes,
		maxItems:  maxItems,
	}
}

func (s *Service) Poll(ctx context.Context) (int, int, error) {
	if s == nil || s.repo == nil || s.rsshub == nil {
		return 0, 0, errors.New("news_service_not_initialized")
	}
	totalItems := 0
	totalBriefs := 0
	for _, route := range s.routes {
		items, err := s.rsshub.Fetch(route, s.maxItems)
		if err != nil {
			continue
		}
		for _, item := range items {
			if item.Title == "" || item.Link == "" {
				continue
			}
			totalItems++
			newsItem := NewsItem{
				Source:      route,
				Title:       item.Title,
				Link:        item.Link,
				GUID:        item.GUID,
				PublishedAt: item.PublishedAt,
				Content:     item.Description,
			}
			created, _, err := s.repo.CreateItem(ctx, newsItem)
			if err != nil {
				continue
			}
			hasBrief, err := s.repo.HasBrief(ctx, created.ID)
			if err != nil || hasBrief {
				continue
			}
			if s.generator == nil {
				continue
			}
			sentiment, brief, keywords, err := s.generator.Generate(ctx, created)
			if err != nil || sentiment == "" || brief == "" {
				continue
			}
			_, err = s.repo.CreateBrief(ctx, created.ID, sentiment, brief, keywords)
			if err == nil {
				totalBriefs++
			}
		}
	}
	return totalItems, totalBriefs, nil
}
