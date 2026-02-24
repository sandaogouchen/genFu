package agent

import (
	"context"
	"io"
	"log"
	"testing"

	"github.com/cloudwego/eino/adk"
	"github.com/cloudwego/eino/schema"

	"genFu/internal/llm"
	"genFu/internal/testutil"
)

func TestLLMAgentHandle(t *testing.T) {
	// if os.Getenv("GENFU_LIVE_TESTS") == "" {
	// 	t.Skip("skip live test")
	// }

	cfg := testutil.LoadConfig(t)
	model, err := llm.NewEinoChatModel(cfg.LLM)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	agent, err := adk.NewChatModelAgent(context.Background(), &adk.ChatModelAgentConfig{
		Name:        "a",
		Description: "x",
		Instruction: "sys",
		Model:       model,
	})
	if err != nil {
		t.Fatalf("agent: %v", err)
	}
	runner := adk.NewRunner(context.Background(), adk.RunnerConfig{Agent: agent})
	iter := runner.Run(context.Background(), []adk.Message{schema.UserMessage("hi")})
	var content string
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			t.Fatalf("run: %v", event.Err)
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		mv := event.Output.MessageOutput
		//mv.IsStreaming = true
		if mv.IsStreaming {
			msg, err := schema.ConcatMessageStream(mv.MessageStream)
			log.Printf("msg: %v", msg)
			if err != nil {
				t.Fatalf("concat: %v", err)
			}
			if msg != nil {
				content = msg.Content
			}
			continue
		}
		if mv.Message != nil && mv.Role == schema.Assistant {
			content = mv.Message.Content
		}
	}
	if content == "" {
		t.Fatalf("empty content")
	}
}

func TestLLMAgentHandleStream(t *testing.T) {
	// if os.Getenv("GENFU_LIVE_TESTS") == "" {
	// 	t.Skip("skip live test")
	// }

	cfg := testutil.LoadConfig(t)
	model, err := llm.NewEinoChatModel(cfg.LLM)
	if err != nil {
		t.Fatalf("client: %v", err)
	}
	agent, err := adk.NewChatModelAgent(context.Background(), &adk.ChatModelAgentConfig{
		Name:        "a",
		Description: "x",
		Instruction: "sys",
		Model:       model,
	})
	if err != nil {
		t.Fatalf("agent: %v", err)
	}
	runner := adk.NewRunner(context.Background(), adk.RunnerConfig{Agent: agent, EnableStreaming: true})
	iter := runner.Run(context.Background(), []adk.Message{schema.UserMessage("hi")})
	var combined string
	for {
		event, ok := iter.Next()
		if !ok {
			break
		}
		if event.Err != nil {
			t.Fatalf("run: %v", event.Err)
		}
		if event.Output == nil || event.Output.MessageOutput == nil {
			continue
		}
		mv := event.Output.MessageOutput
		if !mv.IsStreaming || mv.MessageStream == nil || mv.Role != schema.Assistant {
			continue
		}
		for {
			msg, err := mv.MessageStream.Recv()
			log.Printf("msg: %v", msg)

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
	}
	if combined == "" {
		t.Fatalf("empty delta")
	}
}
