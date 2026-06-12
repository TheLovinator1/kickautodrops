package kick

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"net"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
	utls "github.com/refraction-networking/utls"
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
	"User-Agent":      {"Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"},
}

// Client is a Kick.com API client with retry support and browser TLS fingerprinting.
type Client struct {
	http   *http.Client
	wsDial *websocket.Dialer
}

// newUTLSConn dials a raw TCP connection then wraps it with utls using a Chrome 120
// TLS fingerprint to bypass Cloudflare bot detection.
func newUTLSConn(ctx context.Context, network, addr string, dialer *net.Dialer) (net.Conn, error) {
	conn, err := dialer.DialContext(ctx, network, addr)
	if err != nil {
		return nil, err
	}

	// Extract the hostname from addr (format "host:port") for SNI
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("parse addr for SNI: %w", err)
	}

	// Start with Chrome 120's fingerprint spec
	spec, err := utls.UTLSIdToSpec(utls.HelloChrome_120)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("get chrome spec: %w", err)
	}

	// Chrome normally advertises "h2" + "http/1.1" via ALPN — but servers
	// pick h2, and Go's http.Transport with custom DialTLSContext doesn't
	// auto-upgrade to HTTP/2, causing "malformed HTTP response".  We strip
	// h2 from ALPN and remove the ApplicationSettingsExtension entirely.
	var cleaned []utls.TLSExtension
	for _, ext := range spec.Extensions {
		switch ext.(type) {
		case *utls.ALPNExtension:
			cleaned = append(cleaned, &utls.ALPNExtension{AlpnProtocols: []string{"http/1.1"}})
		case *utls.ApplicationSettingsExtension:
			// Only needed for h2; skip it
		default:
			cleaned = append(cleaned, ext)
		}
	}
	spec.Extensions = cleaned

	tlsConn := utls.UClient(conn, &utls.Config{
		ServerName: host,
	}, utls.HelloCustom)

	if err := tlsConn.ApplyPreset(&spec); err != nil {
		conn.Close()
		return nil, fmt.Errorf("apply preset: %w", err)
	}

	if err := tlsConn.HandshakeContext(ctx); err != nil {
		conn.Close()
		return nil, fmt.Errorf("utls handshake: %w", err)
	}

	return tlsConn, nil
}

// NewClient creates a new Client that mimics a Chrome 120 browser with
// utls TLS fingerprinting on both HTTP and WebSocket connections.
func NewClient() *Client {
	netDialer := &net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}

	transport := &http.Transport{
		DialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return newUTLSConn(ctx, network, addr, netDialer)
		},
		MaxIdleConns:       5,
		IdleConnTimeout:    90 * time.Second,
		DisableCompression: false,
	}

	wsDial := &websocket.Dialer{
		NetDialTLSContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return newUTLSConn(ctx, network, addr, netDialer)
		},
		HandshakeTimeout: 30 * time.Second,
	}

	return &Client{
		http: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 5 {
					return fmt.Errorf("too many redirects")
				}
				return nil
			},
		},
		wsDial: wsDial,
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
