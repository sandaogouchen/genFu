package tool

import (
	"context"
	"errors"
	"strings"

	"genFu/internal/news"
)

type BriefSearchTool struct {
	repo *news.Repository
}

func NewBriefSearchTool(repo *news.Repository) BriefSearchTool {
	return BriefSearchTool{repo: repo}
}

func (t BriefSearchTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "brief_search",
		Description: "search news briefs by keywords",
		Params: map[string]string{
			"keywords": "array",
			"limit":    "number",
		},
		Required: []string{"keywords"},
	}
}

func (t BriefSearchTool) Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	if t.repo == nil {
		return ToolResult{Name: "brief_search", Error: "repository_not_initialized"}, errors.New("repository_not_initialized")
	}
	keywords, err := requireStringSliceArg(args, "keywords")
	if err != nil {
		return ToolResult{Name: "brief_search", Error: err.Error()}, err
	}
	limit, _ := optionalIntArg(args, "limit")
	results, err := t.repo.ListBriefsByKeywords(ctx, normalizeKeywords(keywords), limit)
	return ToolResult{Name: "brief_search", Output: results, Error: errorString(err)}, err
}

func normalizeKeywords(input []string) []string {
	out := make([]string, 0, len(input))
	for _, kw := range input {
		kw = strings.TrimSpace(kw)
		if kw != "" {
			out = append(out, kw)
		}
	}
	return out
}
