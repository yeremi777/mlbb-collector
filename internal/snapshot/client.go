package snapshot

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const (
	site       = "https://www.mobilelegends.com"
	retryDelay = time.Second
	// minTotal is the roster size a real Snapshot exceeds; a smaller total
	// is taken as a changed endpoint, not a smaller game.
	minTotal = 100
)

// Client fetches Snapshots from Moonton's rank-statistics endpoint.
type Client struct {
	BaseURL   string
	UserAgent string
	HTTP      *http.Client
	// Sleep waits before a retry and between requests.
	Sleep func(time.Duration)
}

// NewClient returns a Client with a 30s request timeout.
func NewClient(baseURL, userAgent string) *Client {
	return &Client{BaseURL: baseURL, UserAgent: userAgent, HTTP: &http.Client{Timeout: 30 * time.Second}, Sleep: time.Sleep}
}

// Fetch returns every record of one combination. A transport error, a
// non-200 status, an undecodable body, or a short read is retried once after
// 1s; a body Moonton marks as failed is not.
func (c *Client) Fetch(ctx context.Context, combo Combination) ([]Record, error) {
	records, retry, err := c.fetchOnce(ctx, combo)
	if err == nil || !retry || ctx.Err() != nil {
		return records, err
	}
	c.Sleep(retryDelay)
	records, _, err = c.fetchOnce(ctx, combo)
	return records, err
}

type wireResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Total   int               `json:"total"`
		Records []json.RawMessage `json:"records"`
	} `json:"data"`
}

type wireRecord struct {
	Data struct {
		MoontonID       *int     `json:"main_heroid"`
		WinRate         *float64 `json:"main_hero_win_rate"`
		AppearanceShare *float64 `json:"main_hero_appearance_rate"`
		BanRate         *float64 `json:"main_hero_ban_rate"`
	} `json:"data"`
}

// fetchOnce makes one request; retry reports whether a second one may succeed.
func (c *Client) fetchOnce(ctx context.Context, combo Combination) (records []Record, retry bool, err error) {
	endpoint, bigrank, ok := upstreamKeys(combo)
	if !ok {
		return nil, false, fmt.Errorf("no Moonton endpoint for %d-day %s", combo.Window, combo.RankTier)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+"/"+strconv.Itoa(endpoint), bytes.NewReader(requestBody(bigrank)))
	if err != nil {
		return nil, false, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", site)
	req.Header.Set("Referer", site)
	req.Header.Set("User-Agent", c.UserAgent)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, true, fmt.Errorf("fetch: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, true, fmt.Errorf("status %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, true, fmt.Errorf("read body: %w", err)
	}
	var wire wireResponse
	if err := json.Unmarshal(raw, &wire); err != nil {
		return nil, true, fmt.Errorf("decode body: %w", err)
	}
	switch {
	case wire.Code != 0:
		return nil, false, fmt.Errorf("code %d: %s", wire.Code, wire.Message)
	case len(wire.Data.Records) == 0:
		return nil, false, fmt.Errorf("no records")
	case wire.Data.Total <= minTotal:
		return nil, false, fmt.Errorf("total %d is not above %d", wire.Data.Total, minTotal)
	case len(wire.Data.Records) != wire.Data.Total:
		return nil, true, fmt.Errorf("short read: %d records of %d", len(wire.Data.Records), wire.Data.Total)
	}
	records = make([]Record, len(wire.Data.Records))
	for i, payload := range wire.Data.Records {
		var rec wireRecord
		if err := json.Unmarshal(payload, &rec); err != nil {
			return nil, false, fmt.Errorf("record %d: %w", i, err)
		}
		d := rec.Data
		if d.MoontonID == nil || d.WinRate == nil || d.AppearanceShare == nil || d.BanRate == nil {
			return nil, false, fmt.Errorf("record %d lacks main_heroid or a rate", i)
		}
		records[i] = Record{MoontonID: *d.MoontonID, WinRate: *d.WinRate, AppearanceShare: *d.AppearanceShare, BanRate: *d.BanRate, Payload: payload}
	}
	return records, false, nil
}

type filter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
}

// requestBody asks for every hero of one Rank tier. The server's full default
// projection is returned for an empty field list.
func requestBody(bigrank string) []byte {
	b, err := json.Marshal(map[string]any{
		"pageSize":  200,
		"pageIndex": 1,
		"filters":   []filter{{"bigrank", "eq", bigrank}, {"match_type", "eq", "0"}},
		"sorts": []map[string]any{
			{"data": map[string]string{"field": "main_hero_win_rate", "order": "desc"}, "type": "sequence"},
		},
		"fields": []string{},
	})
	if err != nil {
		panic(err)
	}
	return b
}
