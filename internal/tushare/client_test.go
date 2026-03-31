package tushare

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient_EmptyToken(t *testing.T) {
	cfg := Config{Token: ""}
	c := NewClient(cfg)
	if c != nil {
		t.Error("expected nil client for empty token")
	}
}

func TestNewClient_Defaults(t *testing.T) {
	cfg := Config{Token: "test_token"}
	c := NewClient(cfg)
	if c == nil {
		t.Fatal("expected non-nil client")
	}
	if c.config.BaseURL != "http://api.tushare.pro" {
		t.Errorf("expected default base URL, got %s", c.config.BaseURL)
	}
	if c.config.Timeout != 15*time.Second {
		t.Errorf("expected 15s timeout, got %v", c.config.Timeout)
	}
	if c.config.MaxRetries != 3 {
		t.Errorf("expected 3 retries, got %d", c.config.MaxRetries)
	}
	if c.config.RateLimit != 200 {
		t.Errorf("expected 200 rate limit, got %d", c.config.RateLimit)
	}
}

func TestNewClient_RateLimitClamping(t *testing.T) {
	cfg := Config{Token: "test", RateLimit: 600}
	c := NewClient(cfg)
	if c.config.RateLimit != 200 {
		t.Errorf("expected clamped rate limit 200, got %d", c.config.RateLimit)
	}

	cfg2 := Config{Token: "test", RateLimit: -1}
	c2 := NewClient(cfg2)
	if c2.config.RateLimit != 200 {
		t.Errorf("expected default rate limit 200, got %d", c2.config.RateLimit)
	}
}

func TestMaskedToken(t *testing.T) {
	tests := []struct {
		token    string
		expected string
	}{
		{"abc", "***"},
		{"abcdef", "**cdef"},
		{"abcdefghij", "******ghij"},
	}
	for _, tt := range tests {
		c := NewClient(Config{Token: tt.token})
		got := c.MaskedToken()
		if got != tt.expected {
			t.Errorf("MaskedToken(%q) = %q, want %q", tt.token, got, tt.expected)
		}
	}
}

func TestQuery_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := response{
			Code: 0,
			Msg:  "",
			Data: &struct {
				Fields []string        `json:"fields"`
				Items  [][]interface{} `json:"items"`
			}{
				Fields: []string{"ts_code", "trade_date", "close"},
				Items: [][]interface{}{
					{"000001.SZ", "20260101", 15.5},
					{"000001.SZ", "20260102", 16.0},
				},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient(Config{
		Token:   "test_token",
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	})

	rows, err := c.Query(context.Background(), "daily", map[string]string{"ts_code": "000001.SZ"}, []string{"ts_code", "trade_date", "close"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(rows))
	}
	if rows[0]["ts_code"] != "000001.SZ" {
		t.Errorf("expected ts_code=000001.SZ, got %v", rows[0]["ts_code"])
	}
	if rows[0]["close"] != 15.5 {
		t.Errorf("expected close=15.5, got %v", rows[0]["close"])
	}
}

func TestQuery_PermissionDenied(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := response{Code: 2002, Msg: "权限不足"}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient(Config{
		Token:      "test_token",
		BaseURL:    server.URL,
		Timeout:    5 * time.Second,
		MaxRetries: 1,
	})

	_, err := c.Query(context.Background(), "income", map[string]string{}, nil)
	if err != ErrPermissionDenied {
		t.Errorf("expected ErrPermissionDenied, got %v", err)
	}
}

func TestQuery_ServerError_Retry(t *testing.T) {
	attempts := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		if attempts <= 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		resp := response{
			Code: 0,
			Data: &struct {
				Fields []string        `json:"fields"`
				Items  [][]interface{} `json:"items"`
			}{
				Fields: []string{"ts_code"},
				Items:  [][]interface{}{{"000001.SZ"}},
			},
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient(Config{
		Token:      "test_token",
		BaseURL:    server.URL,
		Timeout:    5 * time.Second,
		MaxRetries: 3,
	})

	rows, err := c.Query(context.Background(), "daily", map[string]string{}, nil)
	if err != nil {
		t.Fatalf("expected success after retries, got: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestQuery_EmptyData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := response{Code: 0, Data: nil}
		json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	c := NewClient(Config{
		Token:   "test_token",
		BaseURL: server.URL,
		Timeout: 5 * time.Second,
	})

	rows, err := c.Query(context.Background(), "daily", map[string]string{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rows != nil {
		t.Errorf("expected nil rows for empty data, got %v", rows)
	}
}

func TestColumnarToMaps(t *testing.T) {
	fields := []string{"a", "b", "c"}
	items := [][]interface{}{
		{1.0, "hello", true},
		{2.0, "world", false},
	}
	result := columnarToMaps(fields, items)
	if len(result) != 2 {
		t.Fatalf("expected 2 maps, got %d", len(result))
	}
	if result[0]["a"] != 1.0 {
		t.Errorf("expected a=1.0, got %v", result[0]["a"])
	}
	if result[1]["b"] != "world" {
		t.Errorf("expected b=world, got %v", result[1]["b"])
	}
}
