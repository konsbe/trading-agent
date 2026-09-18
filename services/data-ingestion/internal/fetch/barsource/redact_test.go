package barsource

import (
	"strings"
	"testing"
)

// The exact SHAPE that leaked: Go's *url.Error text from http.Client.Do, which
// embeds the full request URL including the credential. 23 of these were written
// into universe_symbols.backfill_last_error before redaction existed.
//
// The key here is a FAKE stand-in. The first draft of this test pasted the real
// error verbatim and thereby committed the live key to the repository — the same
// mistake the redaction exists to prevent, one layer up. A fixture that needs a
// secret needs a fake one.
const leakedError = `Get "https://api.twelvedata.com/time_series?adjust=all&apikey=f0f0f0f0DEADBEEFf0f0f0f0DEADBEEF&end_date=2026-09-17&interval=1day&order=ASC&outputsize=5000&start_date=2023-09-17&symbol=AMZN": context deadline exceeded (Client.Timeout exceeded while awaiting headers)`

func TestRedactSecrets_RemovesTheKeyFromTheRealLeakedError(t *testing.T) {
	got := RedactSecrets(leakedError)

	const secret = "f0f0f0f0DEADBEEFf0f0f0f0DEADBEEF"
	if strings.Contains(got, secret) {
		t.Fatalf("the API key survived redaction:\n%s", got)
	}
	if !strings.Contains(got, "apikey=REDACTED") {
		t.Errorf("want apikey=REDACTED, got:\n%s", got)
	}
	// Redaction must not destroy the diagnostic value of the error — the symbol,
	// the endpoint and the cause all have to survive, or the operator trades a
	// leak for an unreadable log.
	for _, keep := range []string{"symbol=AMZN", "time_series", "context deadline exceeded", "interval=1day"} {
		if !strings.Contains(got, keep) {
			t.Errorf("redaction removed diagnostic detail %q:\n%s", keep, got)
		}
	}
}

func TestRedactSecrets_CoversEveryProviderCredentialStyle(t *testing.T) {
	cases := []struct {
		name, in, mustNotContain string
	}{
		{"twelve data apikey", "https://x/y?apikey=SECRET123&a=1", "SECRET123"},
		{"tiingo token", "https://x/y?token=SECRET123&a=1", "SECRET123"},
		{"snake case", "https://x/y?api_key=SECRET123&a=1", "SECRET123"},
		{"camel case", "https://x/y?apiKey=SECRET123&a=1", "SECRET123"},
		{"first param", "https://x/y?token=SECRET123", "SECRET123"},
		{"trailing quote", `Get "https://x/y?token=SECRET123": boom`, "SECRET123"},
		{"repeated", "a?token=SECRET123 b?apikey=SECRET123", "SECRET123"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := RedactSecrets(c.in)
			if strings.Contains(got, c.mustNotContain) {
				t.Errorf("secret survived: %s", got)
			}
			if !strings.Contains(got, "REDACTED") {
				t.Errorf("nothing was redacted: %s", got)
			}
		})
	}
}

// Redaction keys off a delimiter so it rewrites parameters, not any substring
// that happens to end in the key name. Over-redacting would quietly strip real
// diagnostic fields.
func TestRedactSecrets_DoesNotMangleLookalikeParameters(t *testing.T) {
	in := "https://x/y?mytoken=KEEPME&refresh_apikeyed=KEEPME2&symbol=AAPL"
	got := RedactSecrets(in)
	for _, keep := range []string{"KEEPME", "KEEPME2", "symbol=AAPL"} {
		if !strings.Contains(got, keep) {
			t.Errorf("over-redacted %q: %s", keep, got)
		}
	}
}

func TestRedactSecrets_LeavesCleanStringsAlone(t *testing.T) {
	in := "twelvedata AMZN: code 404 (permanent): **symbol** not found"
	if got := RedactSecrets(in); got != in {
		t.Errorf("modified a string with no credential:\n in: %s\nout: %s", in, got)
	}
}
