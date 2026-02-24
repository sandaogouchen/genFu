package llm

import (
	"context"
	"io"
	"log"
	"testing"

	"github.com/cloudwego/eino/schema"

	"genFu/internal/testutil"
)

func TestChatStreamBasic(t *testing.T) {
	// if os.Getenv("GENFU_LIVE_TESTS") == "" {
	// 	t.Skip("skip live test")
	// }

	cfg := testutil.LoadConfig(t)
	client, err := NewEinoChatModel(cfg.LLM)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	resp, err := client.Generate(context.Background(), []*schema.Message{schema.UserMessage("你是谁")})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	if resp.Content == "" {
		t.Fatalf("empty content")
	}
	log.Printf("resp: %v", resp)
}

func TestChatStreamToolCalls(t *testing.T) {
	// if os.Getenv("GENFU_LIVE_TESTS") == "" {
	// 	t.Skip("skip live test")
	// }

	cfg := testutil.LoadConfig(t)
	client, err := NewEinoChatModel(cfg.LLM)
	if err != nil {
		t.Fatalf("new client: %v", err)
	}
	stream, err := client.Stream(context.Background(), []*schema.Message{schema.UserMessage("hi")})
	if err != nil {
		t.Fatalf("chat: %v", err)
	}
	var combined string
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("stream: %v", err)
		}
		if msg != nil && msg.Content != "" {
			combined += msg.Content
		}
	}
	if combined == "" {
		t.Fatalf("empty content")
	}
	log.Printf("resp: %v", combined)
}
