package chat

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
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
	model          model.ToolCallingChatModel
	repo           *Repository
	registry       *tool.Registry
	intentRouter   IntentRouter
	decisionSvc    decisionWorkflow
	stockpickerSvc stockpickerWorkflow
	memoryRepo     *SessionMemoryRepository
	memoryAgent    MemorySummarizer
}

const defaultChatUserID = "default"
const intentFallbackThreshold = 0.60

type Option func(*Service)

func WithDecisionService(svc decisionWorkflow) Option {
	return func(s *Service) {
		s.decisionSvc = svc
	}
}

func WithStockPickerService(svc stockpickerWorkflow) Option {
	return func(s *Service) {
		s.stockpickerSvc = svc
	}
}

func WithIntentRouter(router IntentRouter) Option {
	return func(s *Service) {
		s.intentRouter = router
	}
}

func WithSessionMemoryAgent(agent MemorySummarizer) Option {
	return func(s *Service) {
		s.memoryAgent = agent
	}
}

func NewService(model model.ToolCallingChatModel, repo *Repository, registry *tool.Registry, opts ...Option) *Service {
	s := &Service{
		model:    model,
		repo:     repo,
		registry: registry,
	}
	if repo != nil {
		s.memoryRepo = NewSessionMemoryRepository(repo.db)
	}
	for _, opt := range opts {
		if opt != nil {
			opt(s)
		}
	}
	if s.intentRouter == nil {
		s.intentRouter = NewIntentRouterAgent(model)
	}
	if s.memoryAgent == nil {
		s.memoryAgent = NewSessionMemoryAgent(model)
	}
	return s
}

type turnState struct {
	sessionID   string
	history     []*schema.Message
	newMessages []*schema.Message
	allMessages []*schema.Message
	memory      SessionMemory
	lastUser    string
}

func (s *Service) Chat(ctx context.Context, req generate.GenerateRequest) (generate.GenerateResponse, string, error) {
	turn, err := s.prepareTurn(ctx, req)
	if err != nil {
		return generate.GenerateResponse{}, "", err
	}
	route := s.routeTurn(ctx, turn, req)
	var persist []*schema.Message
	var final *schema.Message
	switch route.Workflow {
	case WorkflowDomainDecision:
		assistant, runErr := s.runDecisionWorkflow(ctx, req, turn.sessionID, route)
		if runErr != nil {
			return generate.GenerateResponse{}, "", runErr
		}
		final = schema.AssistantMessage(assistant.Content, nil)
		persist = []*schema.Message{final}
	case WorkflowDomainPicker:
		assistant, runErr := s.runStockpickerWorkflow(ctx, req, turn.sessionID, route)
		if runErr != nil {
			return generate.GenerateResponse{}, "", runErr
		}
		final = schema.AssistantMessage(assistant.Content, nil)
		persist = []*schema.Message{final}
	default:
		input := s.buildModelInput(turn.allMessages, route, turn.memory.Summary)
		final, persist, err = s.runChatWorkflow(ctx, input, route, req.Tools)
		if err != nil {
			return generate.GenerateResponse{}, "", err
		}
	}
	if len(persist) > 0 {
		if err := s.repo.AppendMessages(ctx, turn.sessionID, persist); err != nil {
			return generate.GenerateResponse{}, "", err
		}
	}
	if final == nil {
		return generate.GenerateResponse{}, "", errors.New("empty_chat_response")
	}
	transcript := schemaMessagesToPublic(append(append([]*schema.Message{}, turn.newMessages...), persist...))
	s.updateSessionMemory(ctx, turn.sessionID, turn.memory.Summary, route.Intent, transcript)

	resp := generate.GenerateResponse{
		Message: toPublicMessage(final),
		Meta:    map[string]string{"session_id": turn.sessionID},
	}
	if len(final.ToolCalls) > 0 {
		resp.ToolCalls = toInternalToolCalls(final.ToolCalls)
	}
	return resp, turn.sessionID, nil
}

func (s *Service) ChatStream(ctx context.Context, req generate.GenerateRequest) (<-chan generate.GenerateEvent, string, error) {
	turn, err := s.prepareTurn(ctx, req)
	if err != nil {
		return nil, "", err
	}
	route := s.routeTurn(ctx, turn, req)
	out := make(chan generate.GenerateEvent, 16)
	go func() {
		defer close(out)
		out <- generate.GenerateEvent{Type: "session", Delta: turn.sessionID}
		out <- buildIntentEvent(route)

		persist := make([]*schema.Message, 0, 4)
		switch route.Workflow {
		case WorkflowDomainDecision:
			assistant, runErr := s.runDecisionWorkflow(ctx, req, turn.sessionID, route)
			if runErr != nil {
				out <- generate.GenerateEvent{Type: "error", Delta: runErr.Error(), Done: true}
				return
			}
			for _, chunk := range splitChunks(assistant.Content, 24) {
				out <- generate.GenerateEvent{Type: "delta", Delta: chunk}
			}
			msg := schema.AssistantMessage(assistant.Content, nil)
			persist = append(persist, msg)
			out <- generate.GenerateEvent{Type: "message", Message: &assistant}
		case WorkflowDomainPicker:
			assistant, runErr := s.runStockpickerWorkflow(ctx, req, turn.sessionID, route)
			if runErr != nil {
				out <- generate.GenerateEvent{Type: "error", Delta: runErr.Error(), Done: true}
				return
			}
			for _, chunk := range splitChunks(assistant.Content, 24) {
				out <- generate.GenerateEvent{Type: "delta", Delta: chunk}
			}
			msg := schema.AssistantMessage(assistant.Content, nil)
			persist = append(persist, msg)
			out <- generate.GenerateEvent{Type: "message", Message: &assistant}
		default:
			input := s.buildModelInput(turn.allMessages, route, turn.memory.Summary)
			streamPersist, runErr := s.runChatWorkflowStream(ctx, input, route, req.Tools, out)
			if runErr != nil {
				out <- generate.GenerateEvent{Type: "error", Delta: runErr.Error(), Done: true}
				return
			}
			persist = append(persist, streamPersist...)
		}
		if len(persist) > 0 {
			if err := s.repo.AppendMessages(ctx, turn.sessionID, persist); err != nil {
				out <- generate.GenerateEvent{Type: "error", Delta: err.Error(), Done: true}
				return
			}
		}
		transcript := schemaMessagesToPublic(append(append([]*schema.Message{}, turn.newMessages...), persist...))
		s.updateSessionMemory(ctx, turn.sessionID, turn.memory.Summary, route.Intent, transcript)
		out <- generate.GenerateEvent{Type: "done", Done: true}
	}()
	return out, turn.sessionID, nil
}

func (s *Service) History(ctx context.Context, sessionID string, limit int) ([]message.Message, error) {
	if s == nil || s.repo == nil {
		return nil, errors.New("chat_service_not_ready")
	}
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

func (s *Service) prepareTurn(ctx context.Context, req generate.GenerateRequest) (turnState, error) {
	if s == nil || s.repo == nil {
		return turnState{}, errors.New("chat_service_not_ready")
	}
	sessionID, err := s.repo.EnsureSession(ctx, req.SessionID, defaultChatUserID)
	if err != nil {
		return turnState{}, err
	}
	history, err := s.repo.ListMessages(ctx, sessionID, 0)
	if err != nil {
		return turnState{}, err
	}
	incoming := toSchemaMessages(req.Messages)
	newMessages := trimHistoryPrefix(history, incoming)
	if len(newMessages) > 0 {
		if err := s.repo.AppendMessages(ctx, sessionID, newMessages); err != nil {
			return turnState{}, err
		}
	}
	memory := SessionMemory{SessionID: sessionID}
	if s.memoryRepo != nil {
		if loaded, loadErr := s.memoryRepo.Get(ctx, sessionID); loadErr == nil {
			memory = loaded
		} else {
			log.Printf("chat memory read failed session=%s err=%v", sessionID, loadErr)
		}
	}
	return turnState{
		sessionID:   sessionID,
		history:     history,
		newMessages: newMessages,
		allMessages: append(history, newMessages...),
		memory:      memory,
		lastUser:    lastUserMessage(req.Messages),
	}, nil
}

func (s *Service) routeTurn(ctx context.Context, turn turnState, req generate.GenerateRequest) RouteDecision {
	router := s.intentRouter
	if router == nil {
		return fallbackRoute("router_not_initialized")
	}
	route, err := router.Route(ctx, RouteInput{
		LastUserMessage: turn.lastUser,
		Meta:            req.Meta,
		SessionSummary:  turn.memory.Summary,
	})
	if err != nil {
		log.Printf("intent routing failed session=%s err=%v", turn.sessionID, err)
		return fallbackRoute("intent_routing_error")
	}
	if route.Intent == "" {
		return fallbackRoute("intent_missing")
	}
	if route.Workflow == "" {
		route.Workflow = workflowForIntent(route.Intent)
	}
	if route.AllowedTools == nil {
		route.AllowedTools = toolsForIntent(route.Intent)
	}
	if route.Confidence < intentFallbackThreshold {
		fb := fallbackRoute("low_confidence")
		fb.Confidence = route.Confidence
		return fb
	}
	return route
}

func (s *Service) buildModelInput(messages []*schema.Message, route RouteDecision, summary string) []*schema.Message {
	systemParts := make([]string, 0, 3)
	systemParts = append(systemParts, "你是中文助手，回答要准确、简洁。")
	switch route.Intent {
	case IntentPortfolio:
		systemParts = append(systemParts, "你当前处理投资账户操作类请求。只能使用已授权工具，不要杜撰执行结果。")
	default:
		systemParts = append(systemParts, "你当前处理通用问答请求。")
	}
	if strings.TrimSpace(summary) != "" {
		systemParts = append(systemParts, "会话摘要："+strings.TrimSpace(summary))
	}
	withSystem := make([]*schema.Message, 0, len(messages)+1)
	withSystem = append(withSystem, schema.SystemMessage(strings.Join(systemParts, "\n")))
	withSystem = append(withSystem, messages...)
	return withSystem
}

func (s *Service) runChatWorkflow(ctx context.Context, messages []*schema.Message, route RouteDecision, reqTools []tool.ToolSpec) (*schema.Message, []*schema.Message, error) {
	if s == nil || s.model == nil {
		return nil, nil, errors.New("chat_model_not_ready")
	}
	allow := resolveAllowedTools(route.AllowedTools, reqTools)
	agent, err := s.newAgent(ctx, allow, true)
	if err != nil {
		return nil, nil, err
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent})
	iter := runner.Run(ctx, messages)
	var final *schema.Message
	persist := make([]*schema.Message, 0, 4)
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return nil, nil, event.Err
		}
		mv := getMessageVariant(event)
		if mv == nil {
			continue
		}
		if mv.IsStreaming {
			msg, streamErr := concatMessageStream(mv.MessageStream)
			if streamErr != nil {
				return nil, nil, streamErr
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
	if final == nil {
		return nil, nil, errors.New("empty_chat_response")
	}
	return final, persist, nil
}

func (s *Service) runChatWorkflowStream(ctx context.Context, messages []*schema.Message, route RouteDecision, reqTools []tool.ToolSpec, out chan<- generate.GenerateEvent) ([]*schema.Message, error) {
	if s == nil || s.model == nil {
		return nil, errors.New("chat_model_not_ready")
	}
	allow := resolveAllowedTools(route.AllowedTools, reqTools)
	agent, err := s.newAgent(ctx, allow, true)
	if err != nil {
		return nil, err
	}
	runner := adk.NewRunner(ctx, adk.RunnerConfig{Agent: agent, EnableStreaming: true})
	iter := runner.Run(ctx, messages)
	persist := make([]*schema.Message, 0, 4)
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			return nil, event.Err
		}
		mv := getMessageVariant(event)
		if mv == nil {
			continue
		}
		if mv.IsStreaming {
			if mv.Role == schema.Assistant {
				msg, toolCalls, streamErr := streamAssistant(mv, out)
				if streamErr != nil {
					return nil, streamErr
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
					return nil, streamErr
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
	return persist, nil
}

func (s *Service) updateSessionMemory(ctx context.Context, sessionID string, previousSummary string, intent IntentType, transcript []message.Message) {
	if s == nil || s.memoryRepo == nil || s.memoryAgent == nil {
		return
	}
	summary, err := s.memoryAgent.Summarize(ctx, previousSummary, transcript)
	if err != nil {
		log.Printf("chat memory summarize failed session=%s err=%v", sessionID, err)
		return
	}
	if strings.TrimSpace(summary) == "" {
		return
	}
	if err := s.memoryRepo.Upsert(ctx, sessionID, summary, string(intent)); err != nil {
		log.Printf("chat memory persist failed session=%s err=%v", sessionID, err)
	}
}

func resolveAllowedTools(intentAllowed []string, requested []tool.ToolSpec) []tool.ToolSpec {
	if len(intentAllowed) == 0 {
		return []tool.ToolSpec{}
	}
	allowSet := make(map[string]struct{}, len(intentAllowed))
	for _, name := range intentAllowed {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		allowSet[name] = struct{}{}
	}
	if len(allowSet) == 0 {
		return []tool.ToolSpec{}
	}
	if len(requested) == 0 {
		out := make([]tool.ToolSpec, 0, len(allowSet))
		for _, name := range intentAllowed {
			if _, ok := allowSet[name]; !ok {
				continue
			}
			out = append(out, tool.ToolSpec{Name: name})
			delete(allowSet, name)
		}
		return out
	}
	out := make([]tool.ToolSpec, 0, len(requested))
	seen := map[string]struct{}{}
	for _, spec := range requested {
		name := strings.TrimSpace(spec.Name)
		if name == "" {
			continue
		}
		if _, ok := allowSet[name]; !ok {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		out = append(out, tool.ToolSpec{Name: name})
		seen[name] = struct{}{}
	}
	return out
}

func buildIntentEvent(route RouteDecision) generate.GenerateEvent {
	return generate.GenerateEvent{
		Type: "intent",
		Intent: &generate.IntentEventPayload{
			Intent:     string(route.Intent),
			Workflow:   string(route.Workflow),
			Confidence: route.Confidence,
			Fallback:   route.Fallback,
		},
	}
}

func schemaMessagesToPublic(items []*schema.Message) []message.Message {
	out := make([]message.Message, 0, len(items))
	for _, item := range items {
		if item == nil {
			continue
		}
		out = append(out, toPublicMessage(item))
	}
	return out
}

func lastUserMessage(messages []message.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		if messages[i].Role == message.RoleUser {
			return strings.TrimSpace(messages[i].Content)
		}
	}
	return ""
}

func (s *Service) newAgent(ctx context.Context, allow []tool.ToolSpec, explicitAllow bool) (adk.Agent, error) {
	resolved := tool.BuildEinoTools(s.registry, allow)
	config := &adk.ChatModelAgentConfig{
		Name:        "chat",
		Description: "chat",
		Model:       s.model,
	}
	if explicitAllow {
		if len(allow) > 0 && len(resolved) > 0 {
			config.ToolsConfig = adk.ToolsConfig{
				ToolsNodeConfig: compose.ToolsNodeConfig{
					Tools: resolved,
				},
			}
		}
		return adk.NewChatModelAgent(ctx, config)
	}
	if len(resolved) > 0 {
		config.ToolsConfig = adk.ToolsConfig{
			ToolsNodeConfig: compose.ToolsNodeConfig{
				Tools: resolved,
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
		Role:      schema.Assistant,
		Content:   full.String(),
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
		Role:       schema.Tool,
		Content:    full.String(),
		ToolName:   toolName,
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

func splitChunks(text string, size int) []string {
	if size <= 0 {
		return []string{text}
	}
	runes := []rune(text)
	if len(runes) <= size {
		return []string{text}
	}
	chunks := make([]string, 0, (len(runes)+size-1)/size)
	for i := 0; i < len(runes); i += size {
		end := i + size
		if end > len(runes) {
			end = len(runes)
		}
		chunks = append(chunks, string(runes[i:end]))
	}
	return chunks
}
