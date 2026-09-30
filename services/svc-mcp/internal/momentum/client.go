// Package momentum is svc-mcp's only way to reach the platform: GET requests
// to momentum-api on fixed paths. There is no method parameter, so no tool can
// reach momentum-api's PUT and DELETE routes.
package momentum

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// maxUpstreamBytes bounds what is read from momentum-api (the full Handbook is
// ~125 KB); tools apply their own, smaller cap to what they return.
const maxUpstreamBytes = 2 << 20

var ErrTooLarge = errors.New("upstream response too large")

type Client struct {
	base string
	http *http.Client
}

func New(base string) *Client {
	return &Client{base: strings.TrimRight(base, "/"), http: &http.Client{Timeout: 20 * time.Second}}
}

// Response is a momentum-api reply: status and the redacted JSON body.
type Response struct {
	Status int
	Body   []byte
}

// Get fetches base+path. path must start with "/api/v1/" and its segments are
// built by the caller from validated input only.
func (c *Client) Get(ctx context.Context, path string, q url.Values) (Response, error) {
	if !strings.HasPrefix(path, "/api/v1/") || strings.Contains(path, "..") {
		return Response{}, fmt.Errorf("refusing path %q", path)
	}
	u := c.base + path
	if len(q) > 0 {
		u += "?" + q.Encode()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Response{}, err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return Response{}, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxUpstreamBytes+1))
	if err != nil {
		return Response{}, err
	}
	if len(body) > maxUpstreamBytes {
		return Response{}, ErrTooLarge
	}
	clean, err := Redact(body)
	if err != nil {
		return Response{}, fmt.Errorf("upstream body is not JSON: %w", err)
	}
	return Response{Status: resp.StatusCode, Body: clean}, nil
}

var (
	secretKey = regexp.MustCompile(`(?i)(secret|passw|api_?key|apikey|token|authorization|cookie|credential|dsn|database_url)`)
	// Provider errors echoed into last_error can carry a request URL.
	secretQuery  = regexp.MustCompile(`(?i)\b((?:api_?key|apikey|token|access_token|key|secret|password|sig|signature)=)[^&\s"'<>]+`)
	bearer       = regexp.MustCompile(`(?i)\b(bearer|basic)\s+[A-Za-z0-9._~+/=-]{8,}`)
	userinfoURL  = regexp.MustCompile(`(?i)\b([a-z][a-z0-9+.-]*://)[^/\s:@]+:[^/\s@]+@`)
	redactedMark = "[redacted]"
)

// Redact drops object keys that name a secret and masks credentials inside
// strings. Numbers keep their exact text (json.Number).
func Redact(body []byte) ([]byte, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(walk(v)); err != nil {
		return nil, err
	}
	return bytes.TrimRight(buf.Bytes(), "\n"), nil
}

func walk(v any) any {
	switch t := v.(type) {
	case map[string]any:
		for k, x := range t {
			if secretKey.MatchString(k) {
				delete(t, k)
				continue
			}
			t[k] = walk(x)
		}
		return t
	case []any:
		for i, x := range t {
			t[i] = walk(x)
		}
		return t
	case string:
		return RedactString(t)
	default:
		return v
	}
}

func RedactString(s string) string {
	s = secretQuery.ReplaceAllString(s, "${1}"+redactedMark)
	s = bearer.ReplaceAllString(s, "${1} "+redactedMark)
	return userinfoURL.ReplaceAllString(s, "${1}"+redactedMark+"@")
}
