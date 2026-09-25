package finnhub

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

const secret = "sekret-token-123"

// captureTransport records every request and answers with an empty JSON body,
// or fails with an error that echoes the URL the way net/http's *url.Error does.
type captureTransport struct {
	reqs []*http.Request
	fail bool
}

func (c *captureTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	c.reqs = append(c.reqs, r)
	if c.fail {
		return nil, errors.New(`Get "` + r.URL.String() + `&token=` + secret + `": dial tcp: lookup finnhub.io: no such host`)
	}
	body := "{}"
	if strings.Contains(r.URL.Path, "news") || strings.Contains(r.URL.Path, "earnings") ||
		strings.Contains(r.URL.Path, "recommendation") || strings.Contains(r.URL.Path, "symbol") {
		body = "[]"
	}
	return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader(body)), Header: http.Header{}}, nil
}

type noWait struct{}

func (noWait) Wait(context.Context) error { return nil }

func client(tr *captureTransport) *Client {
	return &Client{Token: secret, HTTP: &http.Client{Transport: tr}, Limiter: noWait{}}
}

func everyCall(c *Client) {
	ctx := context.Background()
	now := time.Now()
	_, _ = c.Quote(ctx, "AAPL")
	_, _ = c.CompanyNews(ctx, "AAPL")
	_, _ = c.CompanyNewsRange(ctx, "AAPL", now.AddDate(0, 0, -1), now)
	_, _ = c.CryptoNews(ctx)
	_, _ = c.Metrics(ctx, "AAPL")
	_, _ = c.Profile2(ctx, "AAPL")
	_, _ = c.FinancialsReported(ctx, "AAPL", "annual")
	_, _ = c.Earnings(ctx, "AAPL")
	_, _ = c.Recommendation(ctx, "AAPL")
	_, _ = c.InsiderTransactions(ctx, "AAPL")
	_, _ = c.InvestorOwnership(ctx, "AAPL", 5)
	_, _ = c.CalendarEconomic(ctx, "2026-09-01", "2026-09-02")
	_, _ = c.CalendarEarnings(ctx, "2026-09-01", "2026-09-02", "AAPL")
	_, _ = c.MarketNews(ctx, "general")
	_, _ = c.StockSymbols(ctx, "US")
}

// The token must never be in a URL: net/http errors embed the URL and those
// errors are logged (the token was found in data-fundamental's logs, 2026-09-25).
func TestTokenIsSentAsHeaderNeverInTheURL(t *testing.T) {
	tr := &captureTransport{}
	everyCall(client(tr))

	if len(tr.reqs) != 15 {
		t.Fatalf("requests = %d, want one per endpoint (15)", len(tr.reqs))
	}
	for _, r := range tr.reqs {
		if strings.Contains(r.URL.String(), secret) || r.URL.Query().Has("token") {
			t.Errorf("%s: token in URL %q", r.URL.Path, r.URL.String())
		}
		if got := r.Header.Get("X-Finnhub-Token"); got != secret {
			t.Errorf("%s: X-Finnhub-Token = %q", r.URL.Path, got)
		}
	}
}

func TestTransportErrorsAreRedacted(t *testing.T) {
	c := client(&captureTransport{fail: true})

	_, err := c.InsiderTransactions(context.Background(), "AAPL")
	if err == nil {
		t.Fatal("want an error")
	}
	if strings.Contains(err.Error(), secret) {
		t.Errorf("error leaks the token: %v", err)
	}
	if !strings.Contains(err.Error(), "no such host") {
		t.Errorf("redaction lost the cause: %v", err)
	}
}
