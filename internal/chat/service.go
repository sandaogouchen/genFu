package chat

import (
	"context"
	"encoding/json"
	"errors"
	"io"
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

type Service struct {
	model    model.ToolCallingChatModel
	repo     *Repository
	registry *tool.Registry
}

const defaultChatUserID = "default"

func NewService(model model.ToolCallingChatModel, repo *Repository, registry *tool.Registry) *Service {
	return &Service{model: model, repo: repo, registry: registry}
}

func (s *Service) Chat(ctx context.Context, req generate.GenerateRequest) (generate.GenerateResponse, string, error) {
	if s == nil || s.model == nil || s.repo == nil {
		return generate.GenerateResponse{}, "", errors.New("chat_service_not_ready")
	}
	sessionID, err := s.repo.EnsureSession(ctx, req.SessionID, defaultChatUserID)
	if err != nil {
		return generate.GenerateResponse{}, "", err
	}
	history, err := s.repo.ListMessages(ctx, sessionID, 0)
	if err != nil {
		return generate.GenerateResponse{}, "", err
	}
	incoming := toSchemaMessages(req.Messages)
	newMessages := trimHistoryPrefix(history, incoming)
	if len(newMessages) > 0 {
		if err := s.repo.AppendMessages(ctx, sessionID, newMessages); err != nil {
			return generate.GenerateResponse{}, "", err
		}
	}
	messages := append(history, newMessages...)
	agent, err := s.newAgent(ctx, req.Tools)
	if err != nil {
		return generate.GenerateResponse{}, "", err
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent})
	iter := runner.Run(ctx, messages)
	var final *schema.Message
	persist := make([]*schema.Message, 0)
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return generate.GenerateResponse{}, "", event.Err
		}
		mv := getMessageVariant(event)
		if mv == nil {
			continue
		}
		if mv.IsStreaming {
			msg, streamErr := concatMessageStream(mv.MessageStream)
			if streamErr != nil {
				return generate.GenerateResponse{}, "", streamErr
			}
			if msg != nil {
				if mv.Role == schema.Assistant {
					final = msg
				}
				persist = append(persist, msg)
			}
			continue
		}
		if mv.Message != nil {
			if mv.Role == schema.Assistant {
				final = mv.Message
			}
			persist = append(persist, mv.Message)
		}
	}
	if len(persist) > 0 {
		if err := s.repo.AppendMessages(ctx, sessionID, persist); err != nil {
			return generate.GenerateResponse{}, "", err
		}
	}
	if final == nil {
		return generate.GenerateResponse{}, "", errors.New("empty_chat_response")
	}
	resp := generate.GenerateResponse{
		Message: toPublicMessage(final),
		Meta:    map[string]string{"session_id": sessionID},
	}
	if len(final.ToolCalls) > 0 {
		resp.ToolCalls = toInternalToolCalls(final.ToolCalls)
	}
	return resp, sessionID, nil
}

func (s *Service) ChatStream(ctx context.Context, req generate.GenerateRequest) (<-chan generate.GenerateEvent, string, error) {
	if s == nil || s.model == nil || s.repo == nil {
		return nil, "", errors.New("chat_service_not_ready")
	}
	sessionID, err := s.repo.EnsureSession(ctx, req.SessionID, defaultChatUserID)
	if err != nil {
		return nil, "", err
	}
	history, err := s.repo.ListMessages(ctx, sessionID, 0)
	if err != nil {
		return nil, "", err
	}
	incoming := toSchemaMessages(req.Messages)
	newMessages := trimHistoryPrefix(history, incoming)
	if len(newMessages) > 0 {
		if err := s.repo.AppendMessages(ctx, sessionID, newMessages); err != nil {
			return nil, "", err
		}
	}
	messages := append(history, newMessages...)
	agent, err := s.newAgent(ctx, req.Tools)
	if err != nil {
		return nil, "", err
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent, EnableStreaming: true})
	iter := runner.Run(ctx, messages)
	out := make(chan generate.GenerateEvent, 16)
	go func() {
		defer close(out)
		out <- generate.GenerateEvent{Type: "session", Delta: sessionID}
		persist := make([]*schema.Message, 0)
		for {
			event, ok := iter.Next()
			if !ok {
				break
			}
			if event.Err != nil {
				out <- generate.GenerateEvent{Type: "error", Delta: event.Err.Error(), Done: true}
				return
			}
			mv := getMessageVariant(event)
			if mv == nil {
				continue
			}
			if mv.IsStreaming {
				if mv.Role == schema.Assistant {
					msg, toolCalls, streamErr := streamAssistant(mv, out)
					if streamErr != nil {
						out <- generate.GenerateEvent{Type: "error", Delta: streamErr.Error(), Done: true}
						return
					}
					if msg != nil {
						persist = append(persist, msg)
						if len(toolCalls) > 0 {
							for _, call := range toolCalls {
								c := call
								out <- generate.GenerateEvent{Type: "tool_call", ToolCall: &c}
							}
						}
					}
				} else if mv.Role == schema.Tool {
					msg, result, streamErr := streamTool(mv)
					if streamErr != nil {
						out <- generate.GenerateEvent{Type: "error", Delta: streamErr.Error(), Done: true}
						return
					}
					if result != nil {
						out <- generate.GenerateEvent{Type: "tool_result", ToolResult: result}
					}
					if msg != nil {
						persist = append(persist, msg)
					}
				}
				continue
			}
			if mv.Message != nil {
				if mv.Role == schema.Assistant {
					out <- generate.GenerateEvent{Type: "message", Message: toPublicMessagePtr(mv.Message)}
					if len(mv.Message.ToolCalls) > 0 {
						for _, call := range toInternalToolCalls(mv.Message.ToolCalls) {
							c := call
							out <- generate.GenerateEvent{Type: "tool_call", ToolCall: &c}
						}
					}
				} else if mv.Role == schema.Tool {
					if result := parseToolResult(mv.Message); result != nil {
						out <- generate.GenerateEvent{Type: "tool_result", ToolResult: result}
					}
				}
				persist = append(persist, mv.Message)
			}
		}
		if len(persist) > 0 {
			_ = s.repo.AppendMessages(ctx, sessionID, persist)
		}
		out <- generate.GenerateEvent{Type: "done", Done: true}
	}()
	return out, sessionID, nil
}

func (s *Service) History(ctx context.Context, sessionID string, limit int) ([]message.Message, error) {
	msgs, err := s.repo.ListMessages(ctx, sessionID, limit)
	if err != nil {
		return nil, err
	}
	out := make([]message.Message, 0, len(msgs))
	for _, msg := range msgs {
		if msg == nil {
			continue
		}
		out = append(out, toPublicMessage(msg))
	}
	return out, nil
}

func (s *Service) newAgent(ctx context.Context, allow []tool.ToolSpec) (adk.Agent, error) {
	tools := tool.BuildEinoTools(s.registry, allow)
	config := &adk.ChatModelAgentConfig{
		Name:        "chat",
		Description: "chat",
		Model:       s.model,
	}
	if len(tools) > 0 {
		config.ToolsConfig = adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: tools,
			},
		}
	}
	return adk.NewChatModelAgent(ctx, config)
}

func getMessageVariant(event *adk.AgentEvent) *adk.MessageVariant {
	if event == nil || event.Output == nil || event.Output.MessageOutput == nil {
		return nil
	}
	return event.Output.MessageOutput
}

func concatMessageStream(stream *schema.StreamReader[*schema.Message]) (*schema.Message, error) {
	if stream == nil {
		return nil, nil
	}
	return schema.ConcatMessageStream(stream)
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
			out <- generate.GenerateEvent{Type: "delta", Delta: chunk.Content}
		}
		if len(chunk.ToolCalls) > 0 {
			mergeSchemaToolCalls(buffers, chunk.ToolCalls)
		}
	}
	var toolCalls []schema.ToolCall
	if len(buffers) > 0 {
		toolCalls = buildSchemaToolCalls(buffers)
	}
	msg := &schema.Message{
		Role:     schema.Assistant,
		Content:  full.String(),
		ToolCalls: toolCalls,
	}
	out <- generate.GenerateEvent{Type: "message", Message: toPublicMessagePtr(msg)}
	internalCalls := toInternalToolCalls(toolCalls)
	return msg, internalCalls, nil
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
		Role:      schema.Tool,
		Content:   full.String(),
		ToolName:  toolName,
		ToolCallID: toolCallID,
	}
	result := parseToolResult(msg)
	return msg, result, nil
}

type toolCallBuffer struct {
	ID        string
	Type      string
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
		if call.ID != "" {
			buf.ID = call.ID
		}
		if call.Type != "" {
			buf.Type = call.Type
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
			ID:    buf.ID,
			Type:  buf.Type,
			Function: schema.FunctionCall{
				Name:      buf.Name,
				Arguments: strings.TrimSpace(buf.Arguments.String()),
			},
		})
	}
	return out
}

func toSchemaMessages(items []message.Message) []*schema.Message {
	if len(items) == 0 {
		return nil
	}
	out := make([]*schema.Message, 0, len(items))
	for _, msg := range items {
		m := &schema.Message{
			Role:    schema.RoleType(msg.Role),
			Content: msg.Content,
			Name:    msg.Name,
		}
		if msg.Role == message.RoleTool {
			m.ToolCallID = msg.ToolCallID
			m.ToolName = msg.Name
		}
		out = append(out, m)
	}
	return out
}

func toPublicMessage(msg *schema.Message) message.Message {
	if msg == nil {
		return message.Message{}
	}
	out := message.Message{
		Role:    message.Role(msg.Role),
		Content: msg.Content,
		Name:    msg.Name,
	}
	if msg.Role == schema.Tool {
		out.ToolCallID = msg.ToolCallID
		out.Name = msg.ToolName
	}
	return out
}

func toPublicMessagePtr(msg *schema.Message) *message.Message {
	if msg == nil {
		return nil
	}
	m := toPublicMessage(msg)
	return &m
}

func toInternalToolCalls(calls []schema.ToolCall) []tool.ToolCall {
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

func trimHistoryPrefix(history []*schema.Message, incoming []*schema.Message) []*schema.Message {
	if len(history) == 0 || len(incoming) == 0 {
		return incoming
	}
	i := 0
	for i < len(history) && i < len(incoming) {
		if !sameMessage(history[i], incoming[i]) {
			break
		}
		i++
	}
	if i == 0 {
		return incoming
	}
	return incoming[i:]
}

func sameMessage(a *schema.Message, b *schema.Message) bool {
	if a == nil || b == nil {
		return a == b
	}
	ba, errA := json.Marshal(a)
	bb, errB := json.Marshal(b)
	if errA != nil || errB != nil {
		return false
	}
	return string(ba) == string(bb)
}
