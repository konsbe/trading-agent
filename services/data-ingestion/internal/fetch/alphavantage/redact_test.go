package alphavantage

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"

	"golang.org/x/time/rate"
)

const secret = "av-sekret-456"

// failingTransport fails the way net/http does: the error text carries the URL.
type failingTransport struct{}

func (failingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	return nil, errors.New(`Get "` + r.URL.String() + `": dial tcp: lookup www.alphavantage.co: no such host`)
}

// The key has to travel in the query string, so transport errors are the leak
// path; they must come back without it.
func TestTransportErrorsAreRedacted(t *testing.T) {
	c := &Client{APIKey: secret, HTTP: &http.Client{Transport: failingTransport{}}, Limiter: rate.NewLimiter(rate.Inf, 1)}

	_, errOverview := c.Overview(context.Background(), "AAPL")
	_, errNews := c.NewsSentiment(context.Background(), "AAPL")
	for name, err := range map[string]error{"Overview": errOverview, "NewsSentiment": errNews} {
		if err == nil {
			t.Fatalf("%s: want an error", name)
		}
		if strings.Contains(err.Error(), secret) {
			t.Errorf("%s leaks the key: %v", name, err)
		}
		if !strings.Contains(err.Error(), "no such host") {
			t.Errorf("%s lost the cause: %v", name, err)
		}
	}
}
