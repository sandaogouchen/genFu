package tool

import (
	"context"
	"errors"
	"strings"
	"time"

	"genFu/internal/rsshub"
)

type RSSHubTool struct {
	client *rsshub.Client
	routes []string
}

func NewRSSHubTool(baseURL string, routes []string, timeout time.Duration) RSSHubTool {
	return RSSHubTool{
		client: rsshub.NewClient(baseURL, timeout),
		routes: routes,
	}
}

func (t RSSHubTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "rsshub",
		Description: "fetch rss feeds from rsshub",
		Params: map[string]string{
			"action": "string",
			"route":  "string",
			"limit":  "number",
		},
		Required: []string{"action"},
	}
}

func (t RSSHubTool) Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	_ = ctx
	if t.client == nil {
		return ToolResult{Name: "rsshub", Error: "client_not_initialized"}, errors.New("client_not_initialized")
	}
	action, err := requireString(args, "action")
	if err != nil {
		return ToolResult{Name: "rsshub", Error: err.Error()}, err
	}
	switch strings.ToLower(strings.TrimSpace(action)) {
	case "fetch_feed":
		route, err := requireString(args, "route")
		if err != nil {
			return ToolResult{Name: "rsshub", Error: err.Error()}, err
		}
		limit, _ := optionalInt(args, "limit")
		items, err := t.client.Fetch(route, limit)
		return ToolResult{Name: "rsshub", Output: items, Error: errorString(err)}, err
	case "list_routes":
		return ToolResult{Name: "rsshub", Output: t.routes}, nil
	default:
		return ToolResult{Name: "rsshub", Error: "unknown_action"}, errors.New("unknown_action")
	}
}
