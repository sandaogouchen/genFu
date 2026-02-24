package news

import (
	"context"
	"time"
)

type Scheduler struct {
	service  *Service
	interval time.Duration
}

func NewScheduler(service *Service, interval time.Duration) *Scheduler {
	if interval <= 0 {
		interval = 10 * time.Minute
	}
	return &Scheduler{service: service, interval: interval}
}

func (s *Scheduler) Start(ctx context.Context) {
	if s == nil || s.service == nil {
		return
	}
	go s.loop(ctx)
}

func (s *Scheduler) loop(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()
	_, _, _ = s.service.Poll(ctx)
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			_, _, _ = s.service.Poll(ctx)
		}
	}
}
