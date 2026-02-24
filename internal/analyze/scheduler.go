package analyze

import (
	"context"
	"time"
)

type DailyReviewScheduler struct {
	service  *DailyReviewService
	hour     int
	minute   int
	location *time.Location
}

func NewDailyReviewScheduler(service *DailyReviewService, hour int, minute int, location *time.Location) *DailyReviewScheduler {
	if location == nil {
		location = time.Local
	}
	return &DailyReviewScheduler{
		service:  service,
		hour:     hour,
		minute:   minute,
		location: location,
	}
}

func (s *DailyReviewScheduler) Start(ctx context.Context) {
	if s == nil || s.service == nil {
		return
	}
	go s.loop(ctx)
}

func (s *DailyReviewScheduler) loop(ctx context.Context) {
	for {
		next := s.nextRun(time.Now())
		timer := time.NewTimer(time.Until(next))
		select {
		case <-ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
			_, _ = s.service.Run(ctx)
		}
	}
}

func (s *DailyReviewScheduler) nextRun(now time.Time) time.Time {
	current := now.In(s.location)
	target := time.Date(current.Year(), current.Month(), current.Day(), s.hour, s.minute, 0, 0, s.location)
	if !target.After(current) {
		target = target.Add(24 * time.Hour)
	}
	return target
}
