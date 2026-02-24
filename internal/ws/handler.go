package ws

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"

	"genFu/internal/generate"
	"genFu/internal/router"
	"genFu/internal/tool"
)

type Handler struct {
	router   *router.Router
	registry *tool.Registry
	upgrader websocket.Upgrader
}

type streamAgent interface {
	HandleStream(ctx context.Context, req generate.GenerateRequest) (<-chan generate.GenerateEvent, error)
}

func NewHandler(r *router.Router, registry *tool.Registry) *Handler {
	return &Handler{
		router:   r,
		registry: registry,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

type envelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	var cancel context.CancelFunc

	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			return
		}

		if cancel != nil {
			cancel()
			cancel = nil
		}

		req, isCancel, decodeErr := decodeRequest(data)
		if decodeErr != nil {
			_ = sendEvent(conn, generate.GenerateEvent{Type: "error", Delta: decodeErr.Error(), Done: true})
			continue
		}
		if isCancel {
			_ = sendEvent(conn, generate.GenerateEvent{Type: "cancel", Done: true})
			continue
		}

		ctx, newCancel := context.WithCancel(r.Context())
		cancel = newCancel

		agent := h.router.Pick(req)
		if agent == nil {
			_ = sendEvent(conn, generate.GenerateEvent{Type: "error", Delta: "no_agent", Done: true})
			continue
		}

		if streamer, ok := agent.(streamAgent); ok {
			ch, streamErr := streamer.HandleStream(ctx, req)
			if streamErr != nil {
				_ = sendEvent(conn, generate.GenerateEvent{Type: "error", Delta: streamErr.Error(), Done: true})
				continue
			}
			canceled := false
			for evt := range ch {
				select {
				case <-ctx.Done():
					canceled = true
				default:
				}
				if canceled {
					break
				}
				if evt.Type == "tool_call" && evt.ToolCall != nil && h.registry != nil {
					result, execErr := h.registry.Execute(ctx, *evt.ToolCall)
					if execErr != nil && result.Error == "" {
						result.Error = execErr.Error()
					}
					_ = sendEvent(conn, generate.GenerateEvent{Type: "tool_result", ToolResult: &result})
				}
				if evt.Type == "done" {
					continue
				}
				_ = sendEvent(conn, evt)
			}
			if canceled {
				_ = sendEvent(conn, generate.GenerateEvent{Type: "cancel", Done: true})
				continue
			}
			_ = sendEvent(conn, generate.GenerateEvent{Type: "done", Done: true})
			continue
		}

		resp, err := agent.Handle(ctx, req)
		if err != nil {
			_ = sendEvent(conn, generate.GenerateEvent{Type: "error", Delta: err.Error(), Done: true})
			continue
		}

		if resp.Message.Content != "" {
			for _, chunk := range splitChunks(resp.Message.Content, 8) {
				select {
				case <-ctx.Done():
					_ = sendEvent(conn, generate.GenerateEvent{Type: "cancel", Done: true})
					goto nextMessage
				default:
					_ = sendEvent(conn, generate.GenerateEvent{Type: "delta", Delta: chunk})
				}
			}
		}

		_ = sendEvent(conn, generate.GenerateEvent{Type: "message", Message: &resp.Message})

		for _, call := range resp.ToolCalls {
			c := call
			_ = sendEvent(conn, generate.GenerateEvent{Type: "tool_call", ToolCall: &c})
			if h.registry != nil {
				result, execErr := h.registry.Execute(ctx, c)
				if execErr != nil && result.Error == "" {
					result.Error = execErr.Error()
				}
				_ = sendEvent(conn, generate.GenerateEvent{Type: "tool_result", ToolResult: &result})
			}
		}

		_ = sendEvent(conn, generate.GenerateEvent{Type: "done", Done: true})
	nextMessage:
	}
}

func decodeRequest(data []byte) (generate.GenerateRequest, bool, error) {
	var env envelope
	if err := json.Unmarshal(data, &env); err == nil && env.Type != "" {
		switch strings.ToLower(env.Type) {
		case "cancel":
			return generate.GenerateRequest{}, true, nil
		case "generate", "request":
			var req generate.GenerateRequest
			if err := json.Unmarshal(env.Payload, &req); err != nil {
				return generate.GenerateRequest{}, false, err
			}
			return req, false, nil
		}
	}

	var req generate.GenerateRequest
	if err := json.Unmarshal(data, &req); err != nil {
		return generate.GenerateRequest{}, false, err
	}
	return req, false, nil
}

func sendEvent(conn *websocket.Conn, evt generate.GenerateEvent) error {
	data, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
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
