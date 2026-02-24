package router

import (
	"strings"

	"genFu/internal/agent"
	"genFu/internal/generate"
	"genFu/internal/message"
)

type Route struct {
	Keywords []string
	Agent    agent.Agent
}

type Router struct {
	routes       []Route
	defaultAgent agent.Agent
}

func NewRouter(defaultAgent agent.Agent) *Router {
	return &Router{defaultAgent: defaultAgent}
}

func (r *Router) AddRoute(keywords []string, a agent.Agent) {
	r.routes = append(r.routes, Route{Keywords: keywords, Agent: a})
}

func (r *Router) Pick(req generate.GenerateRequest) agent.Agent {
	text := strings.ToLower(lastUserMessage(req.Messages))
	for _, route := range r.routes {
		for _, kw := range route.Keywords {
			if kw != "" && strings.Contains(text, strings.ToLower(kw)) {
				return route.Agent
			}
		}
	}
	if r.defaultAgent != nil {
		return r.defaultAgent
	}
	if len(r.routes) > 0 {
		return r.routes[0].Agent
	}
	return nil
}

func lastUserMessage(messages []message.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == message.RoleUser {
			return messages[i].Content
		}
	}
	return ""
}
