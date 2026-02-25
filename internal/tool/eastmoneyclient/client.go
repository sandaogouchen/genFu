package eastmoneyclient

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/andybalholm/brotli"
	tls "github.com/refraction-networking/utls"
)

const (
	defaultTimeout     = 15 * time.Second
	defaultMaxRetries  = 3
	defaultMinInterval = 200 * time.Millisecond
	defaultReferer     = "https://quote.eastmoney.com/center/gridlist.html"
	defaultUserAgent   = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"
)

type Config struct {
	TLSSpec     tls.ClientHelloID
	Timeout     time.Duration
	MaxRetries  int
	MinInterval time.Duration
	Referer     string
	UserAgent   string
}

func DefaultConfig() Config {
	return Config{
		TLSSpec:     tls.HelloChrome_120,
		Timeout:     defaultTimeout,
		MaxRetries:  defaultMaxRetries,
		MinInterval: defaultMinInterval,
		Referer:     defaultReferer,
		UserAgent:   defaultUserAgent,
	}
}

type Client struct {
	cfg     Config
	mu      sync.Mutex
	lastReq time.Time
}

func NewClient() *Client {
	return &Client{cfg: DefaultConfig()}
}

func NewClientWithConfig(cfg Config) *Client {
	defaults := DefaultConfig()
	if cfg.TLSSpec.Client == "" {
		cfg.TLSSpec = defaults.TLSSpec
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaults.Timeout
	}
	if cfg.MaxRetries <= 0 {
		cfg.MaxRetries = defaults.MaxRetries
	}
	if cfg.MinInterval < 0 {
		cfg.MinInterval = 0
	}
	if cfg.MinInterval == 0 {
		cfg.MinInterval = defaults.MinInterval
	}
	if cfg.Referer == "" {
		cfg.Referer = defaults.Referer
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = defaults.UserAgent
	}
	return &Client{cfg: cfg}
}

func (c *Client) throttle() {
	if c.cfg.MinInterval <= 0 {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	elapsed := time.Since(c.lastReq)
	if elapsed < c.cfg.MinInterval {
		time.Sleep(c.cfg.MinInterval - elapsed)
	}
	c.lastReq = time.Now()
}

func (c *Client) effectiveDeadline(ctx context.Context) time.Time {
	deadline := time.Now().Add(c.cfg.Timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	return deadline
}

func (c *Client) dialTLS(ctx context.Context, host string) (*tls.UConn, error) {
	dialer := &net.Dialer{Timeout: c.cfg.Timeout}
	tcpConn, err := dialer.DialContext(ctx, "tcp", host+":443")
	if err != nil {
		return nil, fmt.Errorf("tcp dial %s:443: %w", host, err)
	}

	deadline := c.effectiveDeadline(ctx)
	_ = tcpConn.SetDeadline(deadline)

	tlsConn := tls.UClient(tcpConn, &tls.Config{ServerName: host}, c.cfg.TLSSpec)
	if err := tlsConn.Handshake(); err != nil {
		_ = tcpConn.Close()
		return nil, fmt.Errorf("tls handshake with %s: %w", host, err)
	}
	return tlsConn, nil
}

func (c *Client) buildRequest(method string, u *url.URL) []byte {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "%s %s HTTP/1.1\r\n", method, u.RequestURI())
	fmt.Fprintf(&buf, "Host: %s\r\n", u.Host)
	fmt.Fprintf(&buf, "Connection: keep-alive\r\n")
	fmt.Fprintf(&buf, "sec-ch-ua: %s\r\n", `"Not_A Brand";v="8", "Chromium";v="120", "Google Chrome";v="120"`)
	fmt.Fprintf(&buf, "sec-ch-ua-mobile: ?0\r\n")
	fmt.Fprintf(&buf, "sec-ch-ua-platform: \"Windows\"\r\n")
	fmt.Fprintf(&buf, "User-Agent: %s\r\n", c.cfg.UserAgent)
	fmt.Fprintf(&buf, "Accept: */*\r\n")
	fmt.Fprintf(&buf, "Sec-Fetch-Site: same-site\r\n")
	fmt.Fprintf(&buf, "Sec-Fetch-Mode: cors\r\n")
	fmt.Fprintf(&buf, "Sec-Fetch-Dest: empty\r\n")
	fmt.Fprintf(&buf, "Referer: %s\r\n", c.cfg.Referer)
	fmt.Fprintf(&buf, "Accept-Encoding: gzip, deflate, br\r\n")
	fmt.Fprintf(&buf, "Accept-Language: zh-CN,zh;q=0.9,en;q=0.8\r\n")
	fmt.Fprintf(&buf, "\r\n")
	return buf.Bytes()
}

func (c *Client) doRequest(ctx context.Context, targetURL string) (int, []byte, error) {
	u, err := url.Parse(targetURL)
	if err != nil {
		return 0, nil, fmt.Errorf("parse url: %w", err)
	}

	c.throttle()

	tlsConn, err := c.dialTLS(ctx, u.Hostname())
	if err != nil {
		return 0, nil, err
	}
	defer tlsConn.Close()

	_ = tlsConn.SetDeadline(c.effectiveDeadline(ctx))

	if _, err := tlsConn.Write(c.buildRequest(http.MethodGet, u)); err != nil {
		return 0, nil, fmt.Errorf("write request: %w", err)
	}

	resp, err := http.ReadResponse(bufio.NewReaderSize(tlsConn, 65536), nil)
	if err != nil {
		return 0, nil, fmt.Errorf("read response: %w", err)
	}
	defer resp.Body.Close()

	rawBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read body: %w", err)
	}

	decoded, err := decodeBody(rawBody, resp.Header.Get("Content-Encoding"))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("decode body: %w", err)
	}
	return resp.StatusCode, decoded, nil
}

func (c *Client) GetWithContext(ctx context.Context, targetURL string) ([]byte, error) {
	if ctx == nil {
		ctx = context.Background()
	}
	var lastErr error
	for attempt := 1; attempt <= c.cfg.MaxRetries; attempt++ {
		status, body, err := c.doRequest(ctx, targetURL)
		if err == nil && status >= 200 && status < 300 {
			return body, nil
		}
		if err != nil {
			lastErr = err
		} else {
			lastErr = fmt.Errorf("http %d: %s", status, truncate(string(body), 240))
		}
		if attempt < c.cfg.MaxRetries {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(time.Duration(attempt) * 200 * time.Millisecond):
			}
		}
	}
	return nil, fmt.Errorf("all %d attempts failed: %w", c.cfg.MaxRetries, lastErr)
}

func (c *Client) Get(targetURL string) ([]byte, error) {
	return c.GetWithContext(context.Background(), targetURL)
}

func (c *Client) GetJSONWithContext(ctx context.Context, targetURL string, result interface{}) error {
	body, err := c.GetWithContext(ctx, targetURL)
	if err != nil {
		return err
	}
	return json.Unmarshal(body, result)
}

func decodeBody(body []byte, encoding string) ([]byte, error) {
	switch encoding {
	case "br":
		return io.ReadAll(brotli.NewReader(bytes.NewReader(body)))
	case "gzip":
		r, err := gzip.NewReader(bytes.NewReader(body))
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return io.ReadAll(r)
	default:
		return body, nil
	}
}

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}
