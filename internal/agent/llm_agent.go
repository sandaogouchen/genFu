package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/compose"
	"github.com/cloudwego/eino/schema"

	"genFu/internal/generate"
	"genFu/internal/message"
	"genFu/internal/tool"
)

type LLMAgent struct {
	name         string
	capabilities []string
	prompt       string
	model        model.ToolCallingChatModel
	registry     *tool.Registry
}

func NewLLMAgentFromFile(name string, capabilities []string, promptPath string, model model.ToolCallingChatModel, registry *tool.Registry) (*LLMAgent, error) {
	data, err := os.ReadFile(promptPath)
	if err != nil {
		return nil, err
	}
	return &LLMAgent{
		name:         name,
		capabilities: capabilities,
		prompt:       strings.TrimSpace(string(data)),
		model:        model,
		registry:     registry,
	}, nil
}

func (a *LLMAgent) Name() string {
	return a.name
}

func (a *LLMAgent) Capabilities() []string {
	return a.capabilities
}

func (a *LLMAgent) Handle(ctx context.Context, req generate.GenerateRequest) (generate.GenerateResponse, error) {
	println("\n" + strings.Repeat("=", 80))
	printf("[LLM AGENT] 开始处理请求\n")
	printf("[LLM AGENT] Agent名称: %s\n", a.name)
	printf("[LLM AGENT] Agent能力: %v\n", a.capabilities)

	last := lastUserMessage(req.Messages)
	printf("[LLM AGENT] 用户输入长度: %d 字符\n", len(last))
	printf("[LLM AGENT] 用户输入预览:\n%s\n", truncateString(last, 300))

	resp, toolResults, err := a.run(ctx, last, false, nil)
	if err != nil {
		printf("[LLM AGENT] ✗ 运行失败: %v\n", err)
		return generate.GenerateResponse{}, err
	}
	if resp == nil {
		printf("[LLM AGENT] ⚠ 响应为空\n")
		return generate.GenerateResponse{}, nil
	}

	printf("[LLM AGENT] ✓ 处理完成\n")
	printf("[LLM AGENT] 响应内容长度: %d 字符\n", len(resp.Content))
	printf("[LLM AGENT] 工具调用次数: %d\n", len(resp.ToolCalls))
	printf("[LLM AGENT] 工具结果数量: %d\n", len(toolResults))
	println(strings.Repeat("=", 80) + "\n")

	return generate.GenerateResponse{
		Message: message.Message{
			Role:    message.RoleAssistant,
			Content: resp.Content,
		},
		ToolCalls: parseSchemaToolCalls(resp.ToolCalls),
		Meta:      buildToolMeta(toolResults),
	}, nil
}

func (a *LLMAgent) HandleStream(ctx context.Context, req generate.GenerateRequest) (<-chan generate.GenerateEvent, error) {
	last := lastUserMessage(req.Messages)
	out := make(chan generate.GenerateEvent, 16)
	go func() {
		defer close(out)
		msg, toolResults, err := a.run(ctx, last, true, out)
		if err != nil {
			out <- generate.GenerateEvent{Type: "error", Delta: err.Error(), Done: true}
			return
		}
		if msg != nil {
			out <- generate.GenerateEvent{Type: "message", Message: toPublicMessagePtr(msg)}
			for _, call := range parseSchemaToolCalls(msg.ToolCalls) {
				c := call
				out <- generate.GenerateEvent{Type: "tool_call", ToolCall: &c}
			}
		}
		if len(toolResults) > 0 {
			for _, result := range toolResults {
				r := result
				out <- generate.GenerateEvent{Type: "tool_result", ToolResult: &r}
			}
		}
		out <- generate.GenerateEvent{Type: "done", Done: true}
	}()
	return out, nil
}

func (a *LLMAgent) run(ctx context.Context, userPrompt string, streaming bool, out chan<- generate.GenerateEvent) (*schema.Message, []tool.ToolResult, error) {
	println("\n[LLM AGENT RUN] 开始构建Agent")

	agent, err := a.buildAgent(ctx)
	if err != nil {
		printf("[LLM AGENT RUN] ✗ 构建Agent失败: %v\n", err)
		return nil, nil, err
	}

	printf("[LLM AGENT RUN] ✓ Agent构建成功\n")
	printf("[LLM AGENT RUN] 创建Runner (streaming=%v)\n", streaming)

	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent, EnableStreaming: streaming})
	iter := runner.Run(ctx, []adk.Message{schema.UserMessage(userPrompt)})

	var finalMsg *schema.Message
	toolResults := make([]tool.ToolResult, 0)
	eventCount := 0

	printf("[LLM AGENT RUN] 开始事件循环\n")

	for {
		event, ok := iter.Next()
		if !ok {
			printf("[LLM AGENT RUN] 事件循环结束，共处理 %d 个事件\n", eventCount)
			break
		}
		eventCount++

		if event.Err != nil {
			printf("[LLM AGENT RUN] ✗ 事件错误: %v\n", event.Err)
			return nil, nil, event.Err
		}

		mv := getMessageVariant(event)
		if mv == nil {
			continue
		}

		printf("[LLM AGENT RUN] 事件 %d: Role=%s, IsStreaming=%v\n", eventCount, mv.Role, mv.IsStreaming)

		if mv.IsStreaming && mv.MessageStream != nil {
			if mv.Role == schema.Assistant {
				printf("[LLM AGENT RUN]   处理流式Assistant消息\n")
				msg, toolCalls, err := streamAssistant(mv, out)
				if err != nil {
					printf("[LLM AGENT RUN]   ✗ 流式Assistant处理失败: %v\n", err)
					return nil, nil, err
				}
				if msg != nil {
					finalMsg = msg
					printf("[LLM AGENT RUN]   ✓ Assistant消息长度: %d, 工具调用: %d\n", len(msg.Content), len(toolCalls))
					if len(toolCalls) > 0 && out != nil {
						for _, call := range toolCalls {
							c := call
							out <- generate.GenerateEvent{Type: "tool_call", ToolCall: &c}
						}
					}
				}
				continue
			}
			if mv.Role == schema.Tool {
				printf("[LLM AGENT RUN]   处理流式Tool消息\n")
				msg, result, err := streamTool(mv)
				if err != nil {
					printf("[LLM AGENT RUN]   ✗ 流式Tool处理失败: %v\n", err)
					return nil, nil, err
				}
				if result != nil {
					toolResults = append(toolResults, *result)
					printf("[LLM AGENT RUN]   ✓ Tool结果: Name=%s, Error=%s\n", result.Name, result.Error)
				}
				if msg != nil && finalMsg == nil {
					finalMsg = msg
				}
				continue
			}
		}
		if mv.Message != nil {
			if mv.Role == schema.Assistant {
				finalMsg = mv.Message
				printf("[LLM AGENT RUN]   非流式Assistant消息长度: %d\n", len(mv.Message.Content))
			} else if mv.Role == schema.Tool {
				if result := parseToolResult(mv.Message); result != nil {
					toolResults = append(toolResults, *result)
					printf("[LLM AGENT RUN]   非流式Tool结果: Name=%s\n", result.Name)
				}
			}
		}
	}

	printf("[LLM AGENT RUN] ✓ 运行完成\n")
	printf("[LLM AGENT RUN] 最终消息: %v, 工具结果数: %d\n", finalMsg != nil, len(toolResults))

	return finalMsg, toolResults, nil
}

func (a *LLMAgent) buildAgent(ctx context.Context) (adk.Agent, error) {
	printf("[BUILD AGENT] Agent名称: %s\n", a.name)
	printf("[BUILD AGENT] 系统提示词长度: %d 字符\n", len(a.prompt))
	printf("[BUILD AGENT] 系统提示词预览:\n%s\n", truncateString(a.prompt, 300))

	config := &adk.ChatModelAgentConfig{
		Name:        a.name,
		Description: strings.Join(a.capabilities, ","),
		Instruction: a.prompt,
		Model:       a.model,
	}

	if a.registry != nil {
		printf("[BUILD AGENT] 构建工具列表\n")
		tools := tool.BuildEinoTools(a.registry, nil)
		printf("[BUILD AGENT] 工具数量: %d\n", len(tools))
		if len(tools) > 0 {
			config.ToolsConfig = adk.ToolsConfig{
				ToolsNodeConfig: compose.ToolsNodeConfig{
					Tools: tools,
				},
			}
			printf("[BUILD AGENT] ✓ 工具配置完成\n")
		} else {
			printf("[BUILD AGENT] ⚠ 没有可用工具\n")
		}
	} else {
		printf("[BUILD AGENT] ⚠ 工具注册表为空\n")
	}

	printf("[BUILD AGENT] 创建ChatModelAgent\n")
	return adk.NewChatModelAgent(ctx, config)
}

func getMessageVariant(event *adk.AgentEvent) *adk.MessageVariant {
	if event == nil || event.Output == nil || event.Output.MessageOutput == nil {
		return nil
	}
	return event.Output.MessageOutput
}

func buildMessages(systemPrompt string, userPrompt string) []*schema.Message {
	out := make([]*schema.Message, 0, 2)
	if strings.TrimSpace(systemPrompt) != "" {
		out = append(out, schema.SystemMessage(systemPrompt))
	}
	if strings.TrimSpace(userPrompt) != "" {
		out = append(out, schema.UserMessage(userPrompt))
	}
	return out
}

type toolCallBuffer struct {
	Name      string
	Arguments strings.Builder
}

func mergeSchemaToolCalls(buffers map[int]*toolCallBuffer, calls []schema.ToolCall) {
	for _, call := range calls {
		if call.Index == nil {
			continue
		}
		idx := *call.Index
		buf := buffers[idx]
		if buf == nil {
			buf = &toolCallBuffer{}
			buffers[idx] = buf
		}
		if call.Function.Name != "" {
			buf.Name = call.Function.Name
		}
		if call.Function.Arguments != "" {
			buf.Arguments.WriteString(call.Function.Arguments)
		}
	}
}

func buildSchemaToolCalls(buffers map[int]*toolCallBuffer) []schema.ToolCall {
	if len(buffers) == 0 {
		return nil
	}
	indexes := make([]int, 0, len(buffers))
	for idx := range buffers {
		indexes = append(indexes, idx)
	}
	sort.Ints(indexes)
	out := make([]schema.ToolCall, 0, len(indexes))
	for _, idx := range indexes {
		buf := buffers[idx]
		if buf == nil || strings.TrimSpace(buf.Name) == "" {
			continue
		}
		index := idx
		out = append(out, schema.ToolCall{
			Index: &index,
			Function: schema.FunctionCall{
				Name:      buf.Name,
				Arguments: strings.TrimSpace(buf.Arguments.String()),
			},
		})
	}
	return out
}

func streamAssistant(mv *adk.MessageVariant, out chan<- generate.GenerateEvent) (*schema.Message, []tool.ToolCall, error) {
	var full strings.Builder
	buffers := map[int]*toolCallBuffer{}
	for {
		chunk, err := mv.MessageStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		if chunk == nil {
			continue
		}
		if chunk.Content != "" {
			full.WriteString(chunk.Content)
			if out != nil {
				out <- generate.GenerateEvent{Type: "delta", Delta: chunk.Content}
			}
		}
		if len(chunk.ToolCalls) > 0 {
			mergeSchemaToolCalls(buffers, chunk.ToolCalls)
		}
	}
	msg := &schema.Message{
		Role:      schema.Assistant,
		Content:   full.String(),
		ToolCalls: buildSchemaToolCalls(buffers),
	}
	return msg, parseSchemaToolCalls(msg.ToolCalls), nil
}

func streamTool(mv *adk.MessageVariant) (*schema.Message, *tool.ToolResult, error) {
	var full strings.Builder
	toolCallID := ""
	toolName := mv.ToolName
	for {
		chunk, err := mv.MessageStream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, nil, err
		}
		if chunk == nil {
			continue
		}
		if toolCallID == "" && chunk.ToolCallID != "" {
			toolCallID = chunk.ToolCallID
		}
		if toolName == "" && chunk.ToolName != "" {
			toolName = chunk.ToolName
		}
		if chunk.Content != "" {
			full.WriteString(chunk.Content)
		}
	}
	msg := &schema.Message{
		Role:       schema.Tool,
		Content:    full.String(),
		ToolName:   toolName,
		ToolCallID: toolCallID,
	}
	return msg, parseToolResult(msg), nil
}

func parseSchemaToolCalls(calls []schema.ToolCall) []tool.ToolCall {
	if len(calls) == 0 {
		return nil
	}
	out := make([]tool.ToolCall, 0, len(calls))
	for _, call := range calls {
		if strings.TrimSpace(call.Function.Name) == "" {
			continue
		}
		args := map[string]interface{}{}
		if strings.TrimSpace(call.Function.Arguments) != "" {
			_ = json.Unmarshal([]byte(call.Function.Arguments), &args)
		}
		out = append(out, tool.ToolCall{
			Name:      call.Function.Name,
			Arguments: args,
		})
	}
	return out
}

func parseToolResult(msg *schema.Message) *tool.ToolResult {
	if msg == nil || strings.TrimSpace(msg.Content) == "" {
		return nil
	}
	var result tool.ToolResult
	if err := json.Unmarshal([]byte(msg.Content), &result); err != nil {
		return &tool.ToolResult{Name: msg.ToolName, Output: msg.Content}
	}
	if result.Name == "" {
		result.Name = msg.ToolName
	}
	return &result
}

func buildToolMeta(results []tool.ToolResult) map[string]string {
	if len(results) == 0 {
		return nil
	}
	payload, err := json.Marshal(results)
	if err != nil {
		return nil
	}
	return map[string]string{"tool_results": string(payload)}
}

func toPublicMessagePtr(msg *schema.Message) *message.Message {
	if msg == nil {
		return nil
	}
	m := message.Message{
		Role:    message.Role(msg.Role),
		Content: msg.Content,
		Name:    msg.Name,
	}
	if msg.Role == schema.Tool {
		m.ToolCallID = msg.ToolCallID
		m.Name = msg.ToolName
	}
	return &m
}

// 辅助函数
func printf(format string, args ...interface{}) {
	fmt.Printf(format, args...)
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
