package router

import (
	"context"
	"testing"

	"genFu/internal/agent"
	"genFu/internal/generate"
	"genFu/internal/message"
	"genFu/internal/testutil"
)

type testAgent struct {
	name string
}

func (t testAgent) Name() string { return t.name }
func (t testAgent) Capabilities() []string { return nil }
func (t testAgent) Handle(ctx context.Context, req generate.GenerateRequest) (generate.GenerateResponse, error) {
	_ = ctx
	_ = req
	return generate.GenerateResponse{}, nil
}

func TestPickByKeyword(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	if cfg.Server.Port == 0 {
		t.Fatalf("missing config")
	}
	a1 := testAgent{name: "a1"}
	a2 := testAgent{name: "a2"}
	r := NewRouter(a1)
	r.AddRoute([]string{"foo"}, a2)
	got := r.Pick(generate.GenerateRequest{
		Messages: []message.Message{{Role: message.RoleUser, Content: "hello foo"}},
	})
	if got.(agent.Agent).Name() != "a2" {
		t.Fatalf("unexpected agent")
	}
}

func TestPickDefault(t *testing.T) {
	cfg := testutil.LoadConfig(t)
	if cfg.News.AccountID == 0 {
		t.Fatalf("missing config")
	}
	a1 := testAgent{name: "a1"}
	r := NewRouter(a1)
	got := r.Pick(generate.GenerateRequest{
		Messages: []message.Message{{Role: message.RoleUser, Content: "hello"}},
	})
	if got.(agent.Agent).Name() != "a1" {
		t.Fatalf("unexpected agent")
	}
}
