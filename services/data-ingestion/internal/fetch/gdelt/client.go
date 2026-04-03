// Package gdelt calls the GDELT 2.1 doc API (no API key).
// TODO [FUTURE]: Add GDELT GEO or Events API for country-level panels.
package gdelt

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/konsbe/trading-agent/services/data-ingestion/internal/httpclient"
)

const docAPI = "https://api.gdeltproject.org/api/v2/doc/doc"

// Client fetches article lists for a boolean query.
type Client struct {
	HTTP *http.Client
}

func New() *Client {
	return &Client{HTTP: httpclient.New(45 * time.Second)}
}

// ArtListResult is a subset of GDELT JSON for mode=ArtList.
type ArtListResult struct {
	Articles []struct {
		URL       string  `json:"url"`
		Title     string  `json:"title"`
		Seendate  string  `json:"seendate"`
		Tone      float64 `json:"tone"`
		Goldstein float64 `json:"goldsteinscale"`
	} `json:"articles"`
}

// FetchArtList returns up to maxRecords articles in the last `lookback` for query.
func (c *Client) FetchArtList(ctx context.Context, query string, maxRecords int, lookback time.Duration) (*ArtListResult, error) {
	if query == "" {
		return nil, fmt.Errorf("gdelt: empty query")
	}
	if maxRecords <= 0 {
		maxRecords = 100
	}
	end := time.Now().UTC()
	start := end.Add(-lookback)
	q := url.Values{}
	q.Set("query", query)
	q.Set("mode", "ArtList")
	q.Set("format", "json")
	q.Set("maxrecords", fmt.Sprintf("%d", maxRecords))
	q.Set("STARTDATETIME", start.Format("200601021504"))
	q.Set("ENDDATETIME", end.Format("200601021504"))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, docAPI+"?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("gdelt: %s", resp.Status)
	}
	var out ArtListResult
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	return &out, nil
}

// AggregateTone returns count, average tone, average Goldstein scale.
func AggregateTone(r *ArtListResult) (n int, avgTone, avgGold *float64) {
	if r == nil || len(r.Articles) == 0 {
		return 0, nil, nil
	}
	var ts, gs float64
	for _, a := range r.Articles {
		ts += a.Tone
		gs += a.Goldstein
	}
	n = len(r.Articles)
	t := ts / float64(n)
	g := gs / float64(n)
	return n, &t, &g
}
