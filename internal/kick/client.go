package kick

import (
	"fmt"
	"math"
	"math/rand"
	"net/http"
	"time"
)

// defaultHeaders applied to every API request.
var defaultHeaders = http.Header{
	"Accept":          {"application/json, text/plain, */*"},
	"Accept-Language": {"en-US,en;q=0.9,sv;q=0.8"},
	"Connection":      {"keep-alive"},
	"Referer":         {"https://kick.com/"},
	"Origin":          {"https://kick.com"},
	"DNT":             {"1"},
	"Sec-Fetch-Dest":  {"empty"},
	"Sec-Fetch-Mode":  {"cors"},
	"Sec-Fetch-Site":  {"same-origin"},
	"User-Agent":      {"Mozilla/5.0 (X11; Linux x86_64; rv:153.0) Gecko/20100101 Firefox/153.0"},
}

// Client is a Kick.com API client with retry support.
type Client struct {
	http *http.Client
}

// NewClient creates a new Client that mimics a Chrome 120 browser.
func NewClient() *Client {
	return &Client{
		http: &http.Client{
			Timeout: 30 * time.Second,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
	}
}

// NewRequest builds an *http.Request with standard Kick headers and cookies.
func (c *Client) NewRequest(method, urlStr string, cookies map[string]string) (*http.Request, error) {
	req, err := http.NewRequest(method, urlStr, nil)
	if err != nil {
		return nil, err
	}

	for k, v := range defaultHeaders {
		req.Header[k] = v
	}

	for name, value := range cookies {
		req.AddCookie(&http.Cookie{Name: name, Value: value})
	}

	return req, nil
}

// SetAuthHeader adds the Bearer Authorization header from session_token.
func SetAuthHeader(req *http.Request, cookies map[string]string) {
	if token, ok := cookies["session_token"]; ok {
		req.Header.Set("Authorization", "Bearer "+token)
	}
}

// Do executes the request with retry on 5xx / transient errors.
func (c *Client) Do(req *http.Request, attempts int) (*http.Response, error) {
	baseDelay := 2 * time.Second

	for i := 0; i < attempts; i++ {
		resp, err := c.http.Do(req)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		if resp != nil {
			resp.Body.Close()
		}

		if i == attempts-1 {
			if err != nil {
				return nil, fmt.Errorf("request failed after %d attempts: %w", attempts, err)
			}
			return nil, fmt.Errorf("request failed after %d attempts (last status: %d)", attempts, resp.StatusCode)
		}

		// Exponential backoff with jitter
		delay := time.Duration(float64(baseDelay) * math.Pow(2, float64(i)))
		jitter := time.Duration(rand.Int63n(int64(delay) / 2))
		time.Sleep(delay + jitter)
	}

	return nil, fmt.Errorf("unreachable")
}
