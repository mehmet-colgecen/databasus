package rabbitmq

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"time"
)

const (
	dialTimeout           = 15 * time.Second
	tlsHandshakeTimeout   = 15 * time.Second
	responseHeaderTimeout = 30 * time.Second
)

// NewHTTPClient builds an HTTP client for the RabbitMQ management API. The
// connect, TLS-handshake and response-header phases are bounded so a hung
// server fails fast, while the body stream itself is governed by the caller's
// context (the long-running backup timeout).
func NewHTTPClient(isHTTPS bool) *http.Client {
	transport := &http.Transport{
		DialContext: (&net.Dialer{
			Timeout: dialTimeout,
		}).DialContext,
		TLSHandshakeTimeout:   tlsHandshakeTimeout,
		ResponseHeaderTimeout: responseHeaderTimeout,
	}

	if isHTTPS {
		transport.TLSClientConfig = &tls.Config{
			InsecureSkipVerify: true, //nolint:gosec // self-signed internal servers are supported
		}
	}

	return &http.Client{Transport: transport}
}

// Get performs an authenticated GET against the management API. The caller
// owns the returned response body and must close it.
func (r *RabbitmqDatabase) Get(
	ctx context.Context,
	client *http.Client,
	path, password string,
) (*http.Response, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, r.BuildURL(path), nil)
	if err != nil {
		return nil, fmt.Errorf("failed to build request: %w", err)
	}

	request.SetBasicAuth(r.Username, password)
	request.Header.Set("Accept", "application/json")

	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("failed to reach RabbitMQ management API: %w", err)
	}

	return response, nil
}

func (r *RabbitmqDatabase) BuildURL(path string) string {
	scheme := "http"
	if r.IsHttps {
		scheme = "https"
	}

	return fmt.Sprintf("%s://%s:%d%s", scheme, r.Host, r.ManagementPort, path)
}
