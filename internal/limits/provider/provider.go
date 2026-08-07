package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/daniel-kindl/agentmeter/internal/limits"
)

// RequestTimeout bounds a single usage request. The dashboard blocks on these
// fetches, so a hung endpoint must not hang the page.
const RequestTimeout = 10 * time.Second

// maxResponseSize bounds how much of a usage response is read. These endpoints
// return a few hundred bytes; anything larger is not a response we understand.
const maxResponseSize = 1 << 20

// Provider fetches the current limit windows for one agent.
//
// Implementations must never place a credential in a returned error: those
// messages reach the dashboard, and they are operator-visible text.
//
// The interface itself lives in the limits package so that the report service
// can consume providers without importing this one.
type Provider = limits.Provider

// fetchJSON issues one authenticated GET and decodes the body into target.
func fetchJSON(ctx context.Context, client *http.Client, vendor, url string, headers map[string]string, target any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return fmt.Errorf("build %s usage request", vendor)
	}
	request.Header.Set("Accept", "application/json")
	for name, value := range headers {
		request.Header.Set(name, value)
	}

	response, err := client.Do(request)
	if err != nil {
		// The transport error can quote the request URL but never a header,
		// so it carries no credential.
		return fmt.Errorf("%s usage request failed: check the network connection", vendor)
	}
	defer func() { _ = response.Body.Close() }()

	if err := statusError(vendor, response.StatusCode); err != nil {
		return err
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, maxResponseSize)).Decode(target); err != nil {
		return fmt.Errorf("%s returned a usage response agentmeter could not read", vendor)
	}
	return nil
}

func statusError(vendor string, status int) error {
	switch status {
	case http.StatusOK:
		return nil
	case http.StatusUnauthorized, http.StatusForbidden:
		return fmt.Errorf("%s rejected the stored credentials: sign in again to refresh them", vendor)
	case http.StatusTooManyRequests:
		return fmt.Errorf("%s rate-limited the usage request: try again shortly", vendor)
	default:
		return fmt.Errorf("%s returned status %d for the usage request", vendor, status)
	}
}

func defaultClient(client *http.Client) *http.Client {
	if client != nil {
		return client
	}
	return &http.Client{Timeout: RequestTimeout}
}

func percentage(value float64) float64 {
	return min(max(value, 0), 100)
}
