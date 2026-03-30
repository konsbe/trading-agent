package fred

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"golang.org/x/time/rate"
)

const endpoint = "https://api.stlouisfed.org/fred/series/observations"

type Client struct {
	APIKey  string
	HTTP    *http.Client
	Limiter *rate.Limiter
}

func New(apiKey string) *Client {
	return &Client{
		APIKey:  apiKey,
		HTTP:    &http.Client{Timeout: 40 * time.Second},
		Limiter: rate.NewLimiter(rate.Every(500*time.Millisecond), 2),
	}
}

type Observation struct {
	Date  string
	Value float64
	Valid bool
}

func (c *Client) FetchSeries(ctx context.Context, seriesID string) ([]Observation, error) {
	if c.APIKey == "" {
		return nil, fmt.Errorf("FRED_API_KEY is empty")
	}
	if err := c.Limiter.Wait(ctx); err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("series_id", seriesID)
	q.Set("api_key", c.APIKey)
	q.Set("file_type", "json")
	u := endpoint + "?" + q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("fred: %s", resp.Status)
	}
	var body struct {
		Observations []struct {
			Date  string `json:"date"`
			Value string `json:"value"`
		} `json:"observations"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return nil, err
	}
	out := make([]Observation, 0, len(body.Observations))
	for _, o := range body.Observations {
		if o.Value == "." {
			out = append(out, Observation{Date: o.Date, Valid: false})
			continue
		}
		v, err := strconv.ParseFloat(o.Value, 64)
		if err != nil {
			out = append(out, Observation{Date: o.Date, Valid: false})
			continue
		}
		out = append(out, Observation{Date: o.Date, Value: v, Valid: true})
	}
	return out, nil
}
