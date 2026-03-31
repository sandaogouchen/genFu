package llm

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"

	"genFu/internal/config"
)

type EinoChatModel struct {
	endpoint    string
	apiKey      string
	model       string
	temperature float32
	httpClient  *http.Client
	retryCount  int
	retryDelay  time.Duration
	sem         chan struct{}
	tools       []*schema.ToolInfo
}

func NewEinoChatModel(cfg config.NormalizedLLMConfig) (*EinoChatModel, error) {
	if strings.TrimSpace(cfg.Endpoint) == "" {
		return nil, errors.New("missing_llm_endpoint")
	}
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, errors.New("missing_llm_api_key")
	}
	if strings.TrimSpace(cfg.Model) == "" {
		return nil, errors.New("missing_llm_model")
	}
	maxInflight := cfg.MaxInflight
	if maxInflight <= 0 {
		maxInflight = 4
	}
	retryDelay := cfg.RetryDelay
	if retryDelay == 0 {
		retryDelay = 3 * time.Minute
	}
	if retryDelay < 3*time.Minute {
		retryDelay = 3 * time.Minute
	}
	timeout := cfg.Timeout
	if timeout <= 0 {
		timeout = 90 * time.Second
	}
	return &EinoChatModel{
		endpoint:    normalizeEndpoint(cfg.Endpoint),
		apiKey:      cfg.APIKey,
		model:       cfg.Model,
		temperature: float32(cfg.Temperature),
		httpClient:  &http.Client{Timeout: timeout},
		retryCount:  cfg.RetryCount,
		retryDelay:  retryDelay,
		sem:         make(chan struct{}, maxInflight),
	}, nil
}

func (m *EinoChatModel) WithTools(tools []*schema.ToolInfo) (model.ToolCallingChatModel, error) {
	cp := *m
	cp.tools = tools
	return &cp, nil
}

func (m *EinoChatModel) Generate(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.Message, error) {
	if m == nil {
		return nil, errors.New("llm_client_not_initialized")
	}
	if err := m.acquire(ctx); err != nil {
		return nil, err
	}
	defer m.release()
	options := model.GetCommonOptions(nil, opts...)
	attempts := m.retryCount + 1
	var lastErr error
	for attempt := 1; attempt <= attempts; attempt++ {
		resp, err := m.generateOnce(ctx, input, options)
		if err == nil {
			return resp, nil
		}
		lastErr = err
		if attempt == attempts || !isRetryableError(err) {
			break
		}
		sleepWithContext(ctx, m.retryDelay)
	}
	return nil, lastErr
}

func (m *EinoChatModel) Stream(ctx context.Context, input []*schema.Message, opts ...model.Option) (*schema.StreamReader[*schema.Message], error) {
	if m == nil {
		return nil, errors.New("llm_client_not_initialized")
	}
	if err := m.acquire(ctx); err != nil {
		return nil, err
	}
	options := model.GetCommonOptions(nil, opts...)
	reader, writer := schema.Pipe[*schema.Message](16)
	go func() {
		defer m.release()
		defer writer.Close()
		attempts := m.retryCount + 1
		for attempt := 1; attempt <= attempts; attempt++ {
			hadChunk := false
			err := m.streamOnce(ctx, input, options, func(msg *schema.Message, err error) {
				if msg != nil || err != nil {
					hadChunk = true
				}
				writer.Send(msg, err)
			})
			if err == nil {
				return
			}
			if hadChunk {
				writer.Send(nil, err)
				return
			}
			if attempt == attempts || !isRetryableError(err) {
				writer.Send(nil, err)
				return
			}
			sleepWithContext(ctx, m.retryDelay)
		}
	}()
	return reader, nil
}

func (m *EinoChatModel) generateOnce(ctx context.Context, input []*schema.Message, opts *model.Options) (*schema.Message, error) {
	reqBody := m.buildRequest(input, opts, false)
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, err
	}
	log.Printf("LLM对话 model=%s payload_bytes=%d", reqBody.Model, len(payload))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.endpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	log.Printf("LLM对话 请求负载：%s", payload)
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	log.Printf("response:%s", body)
	if err != nil {
		return nil, err
	}
	bodyPreview := strings.TrimSpace(string(body))
	if resp.StatusCode >= 400 {
		if len(bodyPreview) > 2048 {
			bodyPreview = bodyPreview[:2048]
		}
		return nil, fmt.Errorf("llm_request_failed status=%d body=%s", resp.StatusCode, bodyPreview)
	}
	var parsed einoChatCompletionResponse
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&parsed); err != nil {
		return nil, err
	}
	if parsed.Error != nil {
		if len(bodyPreview) > 2048 {
			bodyPreview = bodyPreview[:2048]
		}
		return nil, fmt.Errorf("llm_request_failed status=%d body=%s", resp.StatusCode, bodyPreview)
	}
	if len(parsed.Choices) == 0 {
		if len(bodyPreview) > 2048 {
			bodyPreview = bodyPreview[:2048]
		}
		return nil, fmt.Errorf("llm_empty_response body=%s", bodyPreview)
	}
	msg := parsed.Choices[0].Message
	return toSchemaMessageFromEino(msg), nil
}

func (m *EinoChatModel) streamOnce(ctx context.Context, input []*schema.Message, opts *model.Options, emit func(*schema.Message, error)) error {
	reqBody := m.buildRequest(input, opts, true)
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, m.endpoint, bytes.NewReader(payload))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+m.apiKey)
	req.Header.Set("Accept", "text/event-stream")
	resp, err := m.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return fmt.Errorf("llm_request_failed status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	scanner := bufio.NewScanner(resp.Body)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	buffers := map[int]*einoStreamToolCallBuffer{}
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || !strings.HasPrefix(line, "data:") {
			continue
		}
		data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
		if data == "" {
			continue
		}
		if data == "[DONE]" {
			if len(buffers) > 0 {
				emit(buildEinoStreamToolCalls(buffers), nil)
			}
			return nil
		}
		var parsed einoStreamChatCompletionResponse
		if err := json.Unmarshal([]byte(data), &parsed); err != nil {
			return err
		}
		for _, choice := range parsed.Choices {
			if choice.Delta.Content != "" {
				emit(&schema.Message{Role: schema.Assistant, Content: choice.Delta.Content}, nil)
			}
			if len(choice.Delta.ToolCalls) > 0 {
				mergeEinoToolCalls(buffers, choice.Delta.ToolCalls)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if len(buffers) > 0 {
		emit(buildEinoStreamToolCalls(buffers), nil)
	}
	return nil
}

func (m *EinoChatModel) buildRequest(input []*schema.Message, opts *model.Options, stream bool) einoChatCompletionRequest {
	messages := make([]einoChatMessage, 0, len(input))
	for _, msg := range input {
		if msg == nil {
			continue
		}
		messages = append(messages, toEinoChatMessage(msg))
	}
	modelName := m.model
	if opts != nil && opts.Model != nil && strings.TrimSpace(*opts.Model) != "" {
		modelName = strings.TrimSpace(*opts.Model)
	}
	req := einoChatCompletionRequest{
		Model:    modelName,
		Stream:   stream,
		Messages: messages,
	}
	if opts != nil {
		if opts.Temperature != nil {
			req.Temperature = roundTwoDecimals(float64(*opts.Temperature))
		} else if m.temperature > 0 {
			req.Temperature = roundTwoDecimals(float64(m.temperature))
		}
		if opts.MaxTokens != nil && *opts.MaxTokens > 0 {
			req.MaxTokens = *opts.MaxTokens
		}
		if opts.TopP != nil && *opts.TopP > 0 {
			req.TopP = *opts.TopP
		}
		if len(opts.Stop) > 0 {
			req.Stop = opts.Stop
		}
		tools := opts.Tools
		if tools == nil {
			tools = m.tools
		}
		if len(tools) > 0 {
			req.Tools = buildOpenAITools(tools)
		}
		if opts.ToolChoice != nil {
			req.ToolChoice = buildToolChoice(*opts.ToolChoice, opts.AllowedToolNames)
		}
	} else if m.temperature > 0 {
		req.Temperature = roundTwoDecimals(float64(m.temperature))
	}
	return req
}

func roundTwoDecimals(v float64) float64 {
	return math.Round(v*100) / 100
}

func (m *EinoChatModel) acquire(ctx context.Context) error {
	select {
	case m.sem <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (m *EinoChatModel) release() {
	select {
	case <-m.sem:
	default:
	}
}

type einoChatCompletionRequest struct {
	Model       string            `json:"model"`
	Temperature float64           `json:"temperature,omitempty"`
	MaxTokens   int               `json:"max_tokens,omitempty"`
	TopP        float32           `json:"top_p,omitempty"`
	Stop        []string          `json:"stop,omitempty"`
	Messages    []einoChatMessage `json:"messages"`
	Tools       []openAITool      `json:"tools,omitempty"`
	ToolChoice  interface{}       `json:"tool_choice,omitempty"`
	Stream      bool              `json:"stream,omitempty"`
}

type einoChatMessage struct {
	Role       string           `json:"role"`
	Content    string           `json:"content,omitempty"`
	Name       string           `json:"name,omitempty"`
	ToolCallID string           `json:"tool_call_id,omitempty"`
	ToolCalls  []openAIToolCall `json:"tool_calls,omitempty"`
}

type einoChatCompletionResponse struct {
	Error *struct {
		Message string      `json:"message"`
		Type    string      `json:"type"`
		Code    interface{} `json:"code"`
	} `json:"error,omitempty"`
	Choices []struct {
		Message einoChatMessage `json:"message"`
	} `json:"choices"`
}

type openAITool struct {
	Type     string          `json:"type"`
	Function openAIFunction  `json:"function"`
	Extra    json.RawMessage `json:"extra,omitempty"`
}

type openAIFunction struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
}

type openAIToolCall struct {
	ID       string             `json:"id,omitempty"`
	Type     string             `json:"type,omitempty"`
	Function openAIFunctionCall `json:"function"`
}

type openAIFunctionCall struct {
	Name      string `json:"name,omitempty"`
	Arguments string `json:"arguments,omitempty"`
}

type einoStreamChatCompletionResponse struct {
	Choices []struct {
		Delta einoStreamDelta `json:"delta"`
	} `json:"choices"`
}

type einoStreamDelta struct {
	Content   string               `json:"content,omitempty"`
	ToolCalls []einoStreamToolCall `json:"tool_calls,omitempty"`
}

type einoStreamToolCall struct {
	Index    int    `json:"index,omitempty"`
	ID       string `json:"id,omitempty"`
	Type     string `json:"type,omitempty"`
	Function struct {
		Name      string `json:"name,omitempty"`
		Arguments string `json:"arguments,omitempty"`
	} `json:"function,omitempty"`
}

type einoStreamToolCallBuffer struct {
	ID        string
	Type      string
	Name      string
	Arguments string
}

func mergeEinoToolCalls(buffers map[int]*einoStreamToolCallBuffer, calls []einoStreamToolCall) {
	for _, call := range calls {
		buf := buffers[call.Index]
		if buf == nil {
			buf = &einoStreamToolCallBuffer{}
			buffers[call.Index] = buf
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
			buf.Arguments = mergeToolCallArguments(buf.Arguments, call.Function.Arguments)
		}
	}
}

func buildEinoStreamToolCalls(buffers map[int]*einoStreamToolCallBuffer) *schema.Message {
	if len(buffers) == 0 {
		return nil
	}
	indexes := make([]int, 0, len(buffers))
	for idx := range buffers {
		indexes = append(indexes, idx)
	}
	sort.Ints(indexes)
	toolCalls := make([]schema.ToolCall, 0, len(indexes))
	for _, idx := range indexes {
		buf := buffers[idx]
		if buf == nil || strings.TrimSpace(buf.Name) == "" {
			continue
		}
		index := idx
		call := schema.ToolCall{
			Index: &index,
			ID:    buf.ID,
			Type:  buf.Type,
			Function: schema.FunctionCall{
				Name:      buf.Name,
				Arguments: normalizeToolCallArguments(buf.Arguments),
			},
		}
		toolCalls = append(toolCalls, call)
	}
	if len(toolCalls) == 0 {
		return nil
	}
	return &schema.Message{Role: schema.Assistant, ToolCalls: toolCalls}
}

func toSchemaToolCallDelta(calls []einoStreamToolCall) *schema.Message {
	if len(calls) == 0 {
		return nil
	}
	toolCalls := make([]schema.ToolCall, 0, len(calls))
	for _, call := range calls {
		index := call.Index
		tc := schema.ToolCall{
			Index: &index,
			ID:    call.ID,
			Type:  call.Type,
			Function: schema.FunctionCall{
				Name:      call.Function.Name,
				Arguments: call.Function.Arguments,
			},
		}
		toolCalls = append(toolCalls, tc)
	}
	return &schema.Message{Role: schema.Assistant, ToolCalls: toolCalls}
}

func mergeToolCallArguments(current string, chunk string) string {
	current = strings.TrimSpace(current)
	chunk = strings.TrimSpace(chunk)
	if chunk == "" {
		return current
	}
	if current == "" {
		return chunk
	}
	// Provider-specific behavior differs: some stream incremental chunks, others stream cumulative values.
	if strings.HasPrefix(chunk, current) {
		return chunk
	}
	if strings.HasSuffix(current, chunk) {
		return current
	}
	if strings.HasPrefix(chunk, "{") && strings.HasSuffix(chunk, "}") {
		return chunk
	}
	return current + chunk
}

func normalizeToolCallArguments(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return raw
	}
	if json.Valid([]byte(raw)) {
		return raw
	}

	dec := json.NewDecoder(strings.NewReader(raw))
	var (
		last   interface{}
		hasAny bool
	)
	for {
		var value interface{}
		err := dec.Decode(&value)
		if err == io.EOF {
			break
		}
		if err != nil {
			hasAny = false
			break
		}
		last = value
		hasAny = true
	}
	if hasAny {
		payload, err := json.Marshal(last)
		if err == nil {
			return string(payload)
		}
	}
	return raw
}

func toEinoChatMessage(msg *schema.Message) einoChatMessage {
	role := string(msg.Role)
	out := einoChatMessage{
		Role:    role,
		Content: msg.Content,
		Name:    msg.Name,
	}
	if msg.Role == schema.Tool {
		out.ToolCallID = msg.ToolCallID
		out.Name = msg.ToolName
	}
	if msg.Role == schema.Assistant && len(msg.ToolCalls) > 0 {
		out.ToolCalls = make([]openAIToolCall, 0, len(msg.ToolCalls))
		for _, call := range msg.ToolCalls {
			out.ToolCalls = append(out.ToolCalls, openAIToolCall{
				ID:   call.ID,
				Type: call.Type,
				Function: openAIFunctionCall{
					Name:      call.Function.Name,
					Arguments: call.Function.Arguments,
				},
			})
		}
	}
	return out
}

func toSchemaMessageFromEino(msg einoChatMessage) *schema.Message {
	out := &schema.Message{
		Role:    schema.RoleType(msg.Role),
		Content: msg.Content,
		Name:    msg.Name,
	}
	if msg.Role == string(schema.Tool) {
		out.ToolCallID = msg.ToolCallID
		out.ToolName = msg.Name
	}
	if len(msg.ToolCalls) > 0 {
		out.ToolCalls = make([]schema.ToolCall, 0, len(msg.ToolCalls))
		for _, call := range msg.ToolCalls {
			out.ToolCalls = append(out.ToolCalls, schema.ToolCall{
				ID:   call.ID,
				Type: call.Type,
				Function: schema.FunctionCall{
					Name:      call.Function.Name,
					Arguments: call.Function.Arguments,
				},
			})
		}
	}
	return out
}

func buildOpenAITools(tools []*schema.ToolInfo) []openAITool {
	result := make([]openAITool, 0, len(tools))
	for _, t := range tools {
		if t == nil || strings.TrimSpace(t.Name) == "" {
			continue
		}
		fn := openAIFunction{
			Name:        t.Name,
			Description: t.Desc,
		}
		if t.ParamsOneOf != nil {
			if schemaDef, err := t.ParamsOneOf.ToJSONSchema(); err == nil {
				fn.Parameters = schemaDef
			}
		}
		result = append(result, openAITool{
			Type:     "function",
			Function: fn,
		})
	}
	return result
}

func buildToolChoice(choice schema.ToolChoice, allowed []string) interface{} {
	switch choice {
	case schema.ToolChoiceForbidden:
		return "none"
	case schema.ToolChoiceAllowed:
		if len(allowed) == 1 {
			return map[string]interface{}{
				"type": "function",
				"function": map[string]string{
					"name": allowed[0],
				},
			}
		}
		return "auto"
	case schema.ToolChoiceForced:
		if len(allowed) == 1 {
			return map[string]interface{}{
				"type": "function",
				"function": map[string]string{
					"name": allowed[0],
				},
			}
		}
		return "required"
	default:
		return "auto"
	}
}
