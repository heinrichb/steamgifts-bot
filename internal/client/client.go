// Package client wraps net/http with the bits every steamgifts request needs:
// a per-account cookie jar, a fixed User-Agent, sensible timeouts, jittered
// rate limiting between requests, and structured logging.
package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/heinrichb/steamgifts-bot/internal/ratelimit"
)

// BaseURL is the canonical steamgifts host. Tests override it via WithBaseURL.
const BaseURL = "https://www.steamgifts.com"

// Client is a per-account HTTP client. Construct with New.
type Client struct {
	httpc     *http.Client
	baseURL   string
	userAgent string
	limiter   *ratelimit.Limiter
	log       *slog.Logger
}

// Option configures a Client.
type Option func(*Client)

// WithBaseURL overrides the steamgifts host (for tests).
func WithBaseURL(u string) Option { return func(c *Client) { c.baseURL = u } }

// WithLogger attaches a structured logger.
func WithLogger(l *slog.Logger) Option { return func(c *Client) { c.log = l } }

// WithLimiter overrides the per-request jitter window.
func WithLimiter(l *ratelimit.Limiter) Option { return func(c *Client) { c.limiter = l } }

// WithTimeout overrides the per-request HTTP timeout.
func WithTimeout(d time.Duration) Option {
	return func(c *Client) {
		c.httpc.Timeout = d
	}
}

// New constructs a Client seeded with the given PHPSESSID cookie.
func New(cookie, userAgent string, opts ...Option) (*Client, error) {
	if strings.TrimSpace(cookie) == "" {
		return nil, errors.New("client: cookie is required")
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("client: cookie jar: %w", err)
	}
	c := &Client{
		httpc: &http.Client{
			Jar:     jar,
			Timeout: 30 * time.Second,
		},
		baseURL:   BaseURL,
		userAgent: userAgent,
		limiter:   ratelimit.New(3*time.Second, 8*time.Second),
		log:       slog.Default(),
	}
	for _, o := range opts {
		o(c)
	}
	if c.userAgent == "" {
		c.userAgent = "steamgifts-bot/0.1"
	}
	if err := c.seedCookie(cookie); err != nil {
		return nil, err
	}
	return c, nil
}

func (c *Client) seedCookie(value string) error {
	u, err := url.Parse(c.baseURL)
	if err != nil {
		return fmt.Errorf("client: parse base url: %w", err)
	}
	c.httpc.Jar.SetCookies(u, []*http.Cookie{
		{
			Name:   "PHPSESSID",
			Value:  value,
			Path:   "/",
			Domain: u.Hostname(),
		},
	})
	return nil
}

// BaseURL returns the configured host. Useful for tests and parsers building absolute URLs.
func (c *Client) BaseURL() string { return c.baseURL }

// Get fetches an absolute or relative path and returns the response body.
// Caller is responsible for closing nothing — the body is fully read and returned.
func (c *Client) Get(ctx context.Context, path string) ([]byte, error) {
	return c.do(ctx, http.MethodGet, path, nil, nil)
}

// PostForm submits an application/x-www-form-urlencoded request.
func (c *Client) PostForm(ctx context.Context, path string, form url.Values) ([]byte, error) {
	body := strings.NewReader(form.Encode())
	headers := map[string]string{
		"Content-Type":     "application/x-www-form-urlencoded; charset=UTF-8",
		"X-Requested-With": "XMLHttpRequest",
		"Accept":           "application/json, text/javascript, */*; q=0.01",
	}
	return c.do(ctx, http.MethodPost, path, body, headers)
}

func (c *Client) do(ctx context.Context, method, path string, body io.Reader, headers map[string]string) ([]byte, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	target, err := c.resolve(path)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, method, target, body)
	if err != nil {
		return nil, fmt.Errorf("client: build request: %w", err)
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Referer", c.baseURL+"/")
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	c.log.Debug("http request", "method", method, "url", target)
	resp, err := c.httpc.Do(req)
	if err != nil {
		return nil, fmt.Errorf("client: %s %s: %w", method, target, err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("client: read body: %w", err)
	}
	c.log.Debug("http response", "status", resp.StatusCode, "bytes", len(data))

	if resp.StatusCode >= 400 {
		return data, &HTTPError{Status: resp.StatusCode, URL: target, Body: snippet(data)}
	}
	return data, nil
}

func (c *Client) resolve(path string) (string, error) {
	if strings.HasPrefix(path, "http://") || strings.HasPrefix(path, "https://") {
		return path, nil
	}
	base, err := url.Parse(c.baseURL)
	if err != nil {
		return "", fmt.Errorf("client: parse base url: %w", err)
	}
	rel, err := url.Parse(path)
	if err != nil {
		return "", fmt.Errorf("client: parse path: %w", err)
	}
	return base.ResolveReference(rel).String(), nil
}

func snippet(b []byte) string {
	const max = 200
	if len(b) > max {
		return string(b[:max]) + "…"
	}
	return string(b)
}

// HTTPError represents a non-2xx response from steamgifts.
type HTTPError struct {
	Status int
	URL    string
	Body   string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("http %d from %s: %s", e.Status, e.URL, e.Body)
}
