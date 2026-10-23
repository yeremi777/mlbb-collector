package patch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// retryDelay is Liquipedia's limit of one action=parse request per 30s.
const retryDelay = 30 * time.Second

// Client fetches Portal:Patches through Liquipedia's MediaWiki API.
type Client struct {
	BaseURL   string
	UserAgent string
	HTTP      *http.Client
	// Sleep waits before a retry.
	Sleep func(time.Duration)
}

// NewClient returns a Client with a 30s request timeout.
func NewClient(baseURL, userAgent string) *Client {
	return &Client{BaseURL: baseURL, UserAgent: userAgent, HTTP: &http.Client{Timeout: 30 * time.Second}, Sleep: time.Sleep}
}

// Fetch returns the whole calendar, newest first. A 429 or 5xx, a transport
// error, or a body with no tables is retried once, after 30s or the
// Retry-After value if longer.
func (c *Client) Fetch(ctx context.Context) ([]Patch, error) {
	patches, retryAfter, retry, err := c.fetchOnce(ctx)
	if err == nil || !retry || ctx.Err() != nil {
		return patches, err
	}
	c.Sleep(max(retryDelay, retryAfter))
	patches, _, _, err = c.fetchOnce(ctx)
	return patches, err
}

type wikiResponse struct {
	Parse struct {
		Text struct {
			HTML string `json:"*"`
		} `json:"text"`
	} `json:"parse"`
	Error *struct {
		Code string `json:"code"`
		Info string `json:"info"`
	} `json:"error"`
}

// fetchOnce makes one request; retry reports whether a second one may
// succeed, and retryAfter is the wait the server asked for.
func (c *Client) fetchOnce(ctx context.Context) (patches []Patch, retryAfter time.Duration, retry bool, err error) {
	q := url.Values{"action": {"parse"}, "page": {"Portal:Patches"}, "format": {"json"}, "prop": {"text"}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"?"+q.Encode(), nil)
	if err != nil {
		return nil, 0, false, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, true, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		retry := resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500
		return nil, retryAfterOf(resp), retry, fmt.Errorf("status %d", resp.StatusCode)
	}
	var wire wikiResponse
	if err := json.NewDecoder(resp.Body).Decode(&wire); err != nil {
		// A body that is not JSON is an error or throttle page, which has no tables.
		return nil, 0, true, fmt.Errorf("decode body: %w", err)
	}
	if wire.Error != nil {
		return nil, 0, false, fmt.Errorf("mediawiki error %s: %s", wire.Error.Code, wire.Error.Info)
	}
	patches, err = Parse(strings.NewReader(wire.Parse.Text.HTML))
	var pe *ParseError
	return patches, 0, errors.As(err, &pe) && pe.Retryable(), err
}

// retryAfterOf reads Retry-After in seconds; the HTTP-date form is ignored.
func retryAfterOf(resp *http.Response) time.Duration {
	secs, err := strconv.Atoi(resp.Header.Get("Retry-After"))
	if err != nil || secs <= 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}
