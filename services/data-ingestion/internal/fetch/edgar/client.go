// Package edgar reads SEC EDGAR's public JSON APIs (company_tickers,
// submissions, XBRL companyfacts). SEC requires a User-Agent naming the
// requester with a contact address (SEC_EDGAR_USER_AGENT) and allows at most
// 10 requests per second.
package edgar

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	tickersURL     = "https://www.sec.gov/files/company_tickers.json"
	submissionsURL = "https://data.sec.gov/submissions/CIK%s.json"
	factsURL       = "https://data.sec.gov/api/xbrl/companyfacts/CIK%s.json"
	tickersTTL     = 24 * time.Hour
)

type Client struct {
	userAgent string
	http      *http.Client
	limiter   *rate.Limiter

	mu        sync.Mutex
	ciks      map[string]string
	ciksAt    time.Time
	tickerURL string // overridable in tests
	subURL    string
	factURL   string
}

// New returns nil when userAgent is empty: SEC rejects anonymous clients.
func New(userAgent string, perSec float64, timeout time.Duration) *Client {
	if strings.TrimSpace(userAgent) == "" {
		return nil
	}
	if perSec <= 0 || perSec > 10 {
		perSec = 3
	}
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	return &Client{
		userAgent: userAgent,
		http:      &http.Client{Timeout: timeout},
		limiter:   rate.NewLimiter(rate.Limit(perSec), 1),
		tickerURL: tickersURL, subURL: submissionsURL, factURL: factsURL,
	}
}

func (c *Client) get(ctx context.Context, url string) ([]byte, error) {
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("edgar %s: %s", url, resp.Status)
	}
	return body, nil
}

// CIK returns the zero-padded 10-digit CIK for a ticker ("" when SEC lists
// none). The ticker map is fetched once a day.
func (c *Client) CIK(ctx context.Context, ticker string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.ciks == nil || time.Since(c.ciksAt) > tickersTTL {
		body, err := c.get(ctx, c.tickerURL)
		if err != nil {
			return "", err
		}
		var raw map[string]struct {
			CIK    int64  `json:"cik_str"`
			Ticker string `json:"ticker"`
		}
		if err := json.Unmarshal(body, &raw); err != nil {
			return "", fmt.Errorf("edgar company_tickers: %w", err)
		}
		m := make(map[string]string, len(raw))
		for _, v := range raw {
			m[strings.ToUpper(v.Ticker)] = fmt.Sprintf("%010d", v.CIK)
		}
		c.ciks, c.ciksAt = m, time.Now()
	}
	t := strings.ToUpper(strings.TrimSpace(ticker))
	if cik, ok := c.ciks[t]; ok {
		return cik, nil
	}
	return c.ciks[strings.ReplaceAll(t, ".", "-")], nil
}

// CompanyFacts is the raw XBRL companyfacts document.
func (c *Client) CompanyFacts(ctx context.Context, cik string) ([]byte, error) {
	return c.get(ctx, fmt.Sprintf(c.factURL, cik))
}

// Filing is one entry of a company's filing index.
type Filing struct {
	Form      string `json:"form"`
	Filed     string `json:"filed"`      // YYYY-MM-DD
	PeriodEnd string `json:"period_end"` // reportDate, YYYY-MM-DD
	Accession string `json:"accession"`
}

// Latest20F returns the newest 20-F (or 20-F/A) in the company's recent
// filings, nil when there is none.
func (c *Client) Latest20F(ctx context.Context, cik string) (*Filing, error) {
	body, err := c.get(ctx, fmt.Sprintf(c.subURL, cik))
	if err != nil {
		return nil, err
	}
	return latest20F(body)
}

func latest20F(body []byte) (*Filing, error) {
	var sub struct {
		Filings struct {
			Recent struct {
				Form       []string `json:"form"`
				FilingDate []string `json:"filingDate"`
				ReportDate []string `json:"reportDate"`
				Accession  []string `json:"accessionNumber"`
			} `json:"recent"`
		} `json:"filings"`
	}
	if err := json.Unmarshal(body, &sub); err != nil {
		return nil, fmt.Errorf("edgar submissions: %w", err)
	}
	r := sub.Filings.Recent
	var best *Filing
	for i, form := range r.Form {
		if form != "20-F" && form != "20-F/A" || i >= len(r.FilingDate) || i >= len(r.ReportDate) || i >= len(r.Accession) {
			continue
		}
		if best == nil || r.FilingDate[i] > best.Filed {
			best = &Filing{Form: form, Filed: r.FilingDate[i], PeriodEnd: r.ReportDate[i], Accession: r.Accession[i]}
		}
	}
	return best, nil
}
