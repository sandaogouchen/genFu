package agent

import (
	"context"

	"genFu/internal/generate"
)

type Agent interface {
	Name() string
	Capabilities() []string
	Handle(ctx context.Context, req generate.GenerateRequest) (generate.GenerateResponse, error)
}
