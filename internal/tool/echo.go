package tool

import "context"

type EchoTool struct{}

func (t EchoTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        "echo",
		Description: "echo input text",
		Params: map[string]string{
			"text": "string",
		},
		Required: []string{"text"},
	}
}

func (t EchoTool) Execute(ctx context.Context, args map[string]interface{}) (ToolResult, error) {
	_ = ctx
	return ToolResult{
		Name:   "echo",
		Output: args["text"],
	}, nil
}
