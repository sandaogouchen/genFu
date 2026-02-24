package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// EmbeddingConfig represents embedding service configuration
type EmbeddingConfig struct {
	Provider string // "openai" / "dashscope" / "custom"
	APIKey   string
	Model    string // "text-embedding-3-small" / "text-embedding-v2"
	BaseURL  string
	Timeout  time.Duration
}

// EmbeddingService represents independent text embedding service
type EmbeddingService struct {
	config EmbeddingConfig
	client *http.Client
}

// NewEmbeddingService creates a new embedding service
func NewEmbeddingService(config EmbeddingConfig) *EmbeddingService {
	timeout := config.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	return &EmbeddingService{
		config: config,
		client: &http.Client{Timeout: timeout},
	}
}

// Encode encodes texts to vectors
func (s *EmbeddingService) Encode(ctx context.Context, texts []string) ([][]float64, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	switch strings.ToLower(s.config.Provider) {
	case "openai":
		return s.encodeOpenAI(ctx, texts)
	case "dashscope":
		return s.encodeDashScope(ctx, texts)
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s", s.config.Provider)
	}
}

// EventDomain represents event domain type for news classification
type EventDomain string

// Classify implements news.EmbeddingClassifier interface for classification purposes
func (s *EmbeddingService) Classify(ctx context.Context, title, summary string) ([]EventDomain, []string, float64, error) {
	// Simple implementation: encode text and return empty domains/types
	// Full classification logic would be in the tagger
	_, err := s.Encode(ctx, []string{title + " " + summary})
	if err != nil {
		return nil, nil, 0, err
	}
	return nil, nil, 0, nil
}

// ──────────────────────────────────────────────
// OpenAI Embedding API
// ──────────────────────────────────────────────

type openAIEmbeddingRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type openAIEmbeddingResponse struct {
	Object string `json:"object"`
	Data   []struct {
		Object    string    `json:"object"`
		Index     int       `json:"index"`
		Embedding []float64 `json:"embedding"`
	} `json:"data"`
	Model string `json:"model"`
	Usage struct {
		PromptTokens int `json:"prompt_tokens"`
		TotalTokens  int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

func (s *EmbeddingService) encodeOpenAI(ctx context.Context, texts []string) ([][]float64, error) {
	baseURL := s.config.BaseURL
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}
	url := strings.TrimSuffix(baseURL, "/") + "/embeddings"

	model := s.config.Model
	if model == "" {
		model = "text-embedding-3-small"
	}

	reqBody := openAIEmbeddingRequest{
		Model: model,
		Input: texts,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.config.APIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var result openAIEmbeddingResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	if result.Error != nil {
		return nil, fmt.Errorf("OpenAI API error: %s (type: %s, code: %s)",
			result.Error.Message, result.Error.Type, result.Error.Code)
	}

	if len(result.Data) == 0 {
		return nil, fmt.Errorf("no embedding data in response")
	}

	// Sort by index to ensure correct order
	embeddings := make([][]float64, len(texts))
	for _, d := range result.Data {
		if d.Index >= 0 && d.Index < len(embeddings) {
			embeddings[d.Index] = d.Embedding
		}
	}

	return embeddings, nil
}

// ──────────────────────────────────────────────
// DashScope (Alibaba Cloud) Embedding API
// ──────────────────────────────────────────────

type dashScopeEmbeddingRequest struct {
	Model string `json:"model"`
	Input struct {
		Texts []string `json:"texts"`
	} `json:"input"`
	Parameters struct {
		TextType string `json:"text_type,omitempty"` // "query" or "document"
	} `json:"parameters,omitempty"`
}

type dashScopeEmbeddingResponse struct {
	Output struct {
		Embeddings []struct {
			TextIndex int       `json:"text_index"`
			Embedding []float64 `json:"embedding"`
		} `json:"embeddings"`
	} `json:"output"`
	Usage struct {
		TotalTokens int `json:"total_tokens"`
	} `json:"usage"`
	Code      string `json:"code,omitempty"`
	Message   string `json:"message,omitempty"`
	RequestID string `json:"request_id,omitempty"`
}

func (s *EmbeddingService) encodeDashScope(ctx context.Context, texts []string) ([][]float64, error) {
	baseURL := s.config.BaseURL
	if baseURL == "" {
		baseURL = "https://dashscope.aliyuncs.com/api/v1/services/embeddings"
	}
	url := strings.TrimSuffix(baseURL, "/") + "/text-embedding/text-embedding"

	model := s.config.Model
	if model == "" {
		model = "text-embedding-v2"
	}

	reqBody := dashScopeEmbeddingRequest{
		Model: model,
	}
	reqBody.Input.Texts = texts
	reqBody.Parameters.TextType = "document"

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.config.APIKey)

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading response: %w", err)
	}

	var result dashScopeEmbeddingResponse
	if err := json.Unmarshal(respBody, &result); err != nil {
		return nil, fmt.Errorf("unmarshaling response: %w", err)
	}

	if result.Code != "" && result.Code != "Success" {
		return nil, fmt.Errorf("DashScope API error: %s (code: %s, request_id: %s)",
			result.Message, result.Code, result.RequestID)
	}

	if len(result.Output.Embeddings) == 0 {
		return nil, fmt.Errorf("no embedding data in response")
	}

	// Sort by text_index to ensure correct order
	embeddings := make([][]float64, len(texts))
	for _, e := range result.Output.Embeddings {
		if e.TextIndex >= 0 && e.TextIndex < len(embeddings) {
			embeddings[e.TextIndex] = e.Embedding
		}
	}

	return embeddings, nil
}
