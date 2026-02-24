package chat

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/gorilla/websocket"

	"genFu/internal/generate"
)

type WSHandler struct {
	service  *Service
	upgrader websocket.Upgrader
}

func NewWSHandler(service *Service) *WSHandler {
	return &WSHandler{
		service: service,
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool { return true },
		},
	}
}

type wsEnvelope struct {
	Type    string          `json:"type"`
	Payload json.RawMessage `json:"payload"`
}

func (h *WSHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
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
		req, isCancel, decodeErr := decodeWSRequest(data)
		if decodeErr != nil {
			_ = sendWSEvent(conn, generate.GenerateEvent{Type: "error", Delta: decodeErr.Error(), Done: true})
			continue
		}
		if isCancel {
			_ = sendWSEvent(conn, generate.GenerateEvent{Type: "cancel", Done: true})
			continue
		}
		ctx, newCancel := context.WithCancel(r.Context())
		cancel = newCancel
		ch, _, err := h.service.ChatStream(ctx, req)
		if err != nil {
			_ = sendWSEvent(conn, generate.GenerateEvent{Type: "error", Delta: err.Error(), Done: true})
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
			_ = sendWSEvent(conn, evt)
		}
		if canceled {
			_ = sendWSEvent(conn, generate.GenerateEvent{Type: "cancel", Done: true})
		}
	}
}

func decodeWSRequest(data []byte) (generate.GenerateRequest, bool, error) {
	var env wsEnvelope
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

func sendWSEvent(conn *websocket.Conn, evt generate.GenerateEvent) error {
	data, err := json.Marshal(evt)
	if err != nil {
		return err
	}
	return conn.WriteMessage(websocket.TextMessage, data)
}
