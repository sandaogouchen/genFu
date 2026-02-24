//go:build live

package ws

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"

	"github.com/gorilla/websocket"

	"genFu/internal/generate"
	"genFu/internal/message"
	"genFu/internal/testutil"
)

func TestWSHandlerStream(t *testing.T) {
	if os.Getenv("GENFU_LIVE_TESTS") == "" {
		t.Skip("skip live test")
	}
	cfg := testutil.LoadConfig(t)
	if cfg.Server.Port == 0 {
		t.Fatalf("missing config")
	}
	wsURL := "ws://localhost:" + strconv.Itoa(cfg.Server.Port) + "/ws/chat"
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Skip("server not available")
	}
	defer conn.Close()

	req := generate.GenerateRequest{
		SessionID: "s",
		Messages:  []message.Message{{Role: message.RoleUser, Content: "hi"}},
	}
	env := map[string]interface{}{
		"type":    "generate",
		"payload": req,
	}
	data, err := json.Marshal(env)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
		t.Fatalf("write: %v", err)
	}
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var evt generate.GenerateEvent
	if err := json.Unmarshal(msg, &evt); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if evt.Type == "" {
		t.Fatalf("empty event type")
	}
}
