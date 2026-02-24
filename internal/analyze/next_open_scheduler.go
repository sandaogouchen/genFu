package analyze

import (
	"context"
	"time"
)

type NextOpenGuideScheduler struct {
	service  *NextOpenGuideService
	hour     int
	minute   int
	location *time.Location
}

func NewNextOpenGuideScheduler(service *NextOpenGuideService, hour int, minute int, location *time.Location) *NextOpenGuideScheduler {
	if location == nil {
		location = time.Local
	}
	return &NextOpenGuideScheduler{
		service:  service,
		hour:     hour,
		minute:   minute,
		location: location,
	}
}

func (s *NextOpenGuideScheduler) Start(ctx context.Context) {
	if s == nil || s.service == nil {
		return
	}
	go s.loop(ctx)
}

func (s *NextOpenGuideScheduler) loop(ctx context.Context) {
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

func (s *NextOpenGuideScheduler) nextRun(now time.Time) time.Time {
	current := now.In(s.location)
	target := time.Date(current.Year(), current.Month(), current.Day(), s.hour, s.minute, 0, 0, s.location)
	if !target.After(current) {
		target = target.Add(24 * time.Hour)
	}
	return target
}
