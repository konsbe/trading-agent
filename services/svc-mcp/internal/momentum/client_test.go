package momentum

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRedact(t *testing.T) {
	in := `{"providers":{"tiingo":{"last_error":"GET https://api.tiingo.com/x?token=abc123&columns=close: 429","api_key":"k","daily_used":12}},` +
		`"note":"Authorization: Bearer eyJhbGciOiJIUzI1NiJ9.x","dsn":"postgres://u:p@h/db","url":"postgres://user:pw@db:5432/trading",` +
		`"list":[{"refresh_token":"r","ok":1.50}]}`
	out, err := Redact([]byte(in))
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, bad := range []string{"abc123", `"api_key"`, "eyJhbGci", `"dsn"`, "user:pw", `"refresh_token"`} {
		if strings.Contains(s, bad) {
			t.Errorf("redacted body still has %q: %s", bad, s)
		}
	}
	for _, keep := range []string{`"daily_used":12`, "columns=close", `"ok":1.50`, "token=[redacted]"} {
		if !strings.Contains(s, keep) {
			t.Errorf("redaction lost %q: %s", keep, s)
		}
	}
}

func TestGetIsGETOnlyAndPathBound(t *testing.T) {
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Method+" "+r.URL.RequestURI())
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()
	c := New(srv.URL)
	if _, err := c.Get(context.Background(), "/api/v1/watchlist", nil); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"/healthz", "/api/v1/../x", "http://evil/api/v1/x"} {
		if _, err := c.Get(context.Background(), p, nil); err == nil {
			t.Errorf("%s accepted", p)
		}
	}
	if len(seen) != 1 || seen[0] != "GET /api/v1/watchlist" {
		t.Fatalf("requests: %v", seen)
	}
}

func TestNonJSONRefused(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html>stack trace with DATABASE_URL=postgres://u:p@h</html>"))
	}))
	defer srv.Close()
	if _, err := New(srv.URL).Get(context.Background(), "/api/v1/x", nil); err == nil {
		t.Fatal("non-JSON body passed through")
	}
}
