package tool

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"testing"
)

type testTimeoutError struct{}

func (testTimeoutError) Error() string   { return "i/o timeout" }
func (testTimeoutError) Timeout() bool   { return true }
func (testTimeoutError) Temporary() bool { return true }

type fundResilienceRoundTripper struct {
	mu             sync.Mutex
	timeoutFirst   bool
	totalCalls     int
	realtimeCalls  int
	historyCalls   int
	timeoutCalls   int
	successPayload string
}

func (rt *fundResilienceRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()

	rt.totalCalls++
	host := strings.ToLower(req.URL.Host)
	switch {
	case strings.Contains(host, "fundgz.1234567.com.cn"):
		rt.realtimeCalls++
		rt.timeoutCalls++
		return nil, &url.Error{Op: "Get", URL: req.URL.String(), Err: testTimeoutError{}}
	case strings.Contains(host, "fundf10.eastmoney.com"):
		rt.historyCalls++
		payload := rt.successPayload
		if strings.TrimSpace(payload) == "" {
			payload = `var apidata={content:"<table><tbody><tr><td>2026-02-20</td><td>1.2345</td><td>1.2345</td><td>0.00%</td></tr></tbody></table>",records:1,pages:1};`
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader(payload)),
			Request:    req,
		}, nil
	default:
		if rt.timeoutFirst && rt.totalCalls == 1 {
			rt.timeoutCalls++
			return nil, &url.Error{Op: "Get", URL: req.URL.String(), Err: testTimeoutError{}}
		}
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     make(http.Header),
			Body:       io.NopCloser(strings.NewReader("ok")),
			Request:    req,
		}, nil
	}
}

func TestDoFundRequestRetriesTimeout(t *testing.T) {
	origTransport := http.DefaultTransport
	rt := &fundResilienceRoundTripper{timeoutFirst: true}
	http.DefaultTransport = rt
	defer func() { http.DefaultTransport = origTransport }()

	body, err := doFundRequest(context.Background(), "https://example.com/test")
	if err != nil {
		t.Fatalf("expected retry success, got err=%v", err)
	}
	if strings.TrimSpace(string(body)) != "ok" {
		t.Fatalf("unexpected body: %q", string(body))
	}
	if rt.totalCalls < 2 {
		t.Fatalf("expected retry calls >=2, got=%d", rt.totalCalls)
	}
}

func TestFetchFundRealtimeFallbackToHistoryOnTimeout(t *testing.T) {
	origTransport := http.DefaultTransport
	rt := &fundResilienceRoundTripper{}
	http.DefaultTransport = rt
	defer func() { http.DefaultTransport = origTransport }()

	points, err := fetchFundRealtime(context.Background(), "014597")
	if err != nil {
		t.Fatalf("expected fallback success, got err=%v", err)
	}
	if len(points) != 1 {
		t.Fatalf("expected 1 point, got=%d", len(points))
	}
	if points[0].Price != 1.2345 {
		t.Fatalf("unexpected fallback price: %.4f", points[0].Price)
	}
	if points[0].Time != "2026-02-20" {
		t.Fatalf("unexpected fallback time: %q", points[0].Time)
	}
	if rt.realtimeCalls == 0 {
		t.Fatalf("expected realtime request attempted")
	}
	if rt.historyCalls == 0 {
		t.Fatalf("expected history fallback request attempted")
	}
}
