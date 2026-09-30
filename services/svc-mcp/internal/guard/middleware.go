package guard

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/netip"
	"strings"
	"time"
)

type Options struct {
	Log               *slog.Logger
	TrustedProxies    []netip.Prefix
	IPAllowlistEnable bool
	IPAllowlist       []netip.Prefix
	Lockout           *Lockout
	PerToken          *Limiter
	Global            *Limiter
	AuthPerIP         *Limiter
}

// Anthropic calls these from its own servers; the browser only visits
// /authorize and the GitHub callback, from the user's IP.
func allowlisted(path string) bool { return path == "/mcp" || path == "/token" }

func authPath(path string) bool {
	return path == "/authorize" || path == "/token" || strings.HasPrefix(path, "/oauth/")
}

// Wrap applies, in order: client IP, request log, IP allowlist, sign-in
// lockout and auth rate limit, MCP rate limits.
func Wrap(next http.Handler, o Options) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ip := resolveIP(r, o.TrustedProxies)
		ipStr := ip.String()
		if !ip.IsValid() {
			ipStr = "unknown"
		}
		r = r.WithContext(withIP(r.Context(), ipStr))
		rec := &recorder{ResponseWriter: w, status: http.StatusOK}
		entry := []any{"ip", ipStr, "method", r.Method, "path", r.URL.Path}
		if tok := bearer(r); tok != "" {
			entry = append(entry, "token_id", tokenID(tok))
		}
		if r.Method == http.MethodPost && r.URL.Path == "/mcp" {
			entry = append(entry, rpcFields(r)...)
		}
		defer func() {
			entry = append(entry, "status", rec.status, "bytes", rec.bytes, "dur_ms", time.Since(start).Milliseconds())
			o.Log.Info("request", entry...)
		}()

		w = rec
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")

		if o.IPAllowlistEnable && allowlisted(r.URL.Path) && !(ip.IsValid() && contains(o.IPAllowlist, ip)) {
			o.Log.Warn("blocked: outside IP allowlist", "ip", ipStr, "path", r.URL.Path)
			http.Error(w, "forbidden", http.StatusForbidden)
			return
		}
		if authPath(r.URL.Path) {
			if o.Lockout.Locked(ipStr) {
				w.Header().Set("Retry-After", "900")
				http.Error(w, "too many failed sign-in attempts", http.StatusTooManyRequests)
				return
			}
			if !o.AuthPerIP.Allow(ipStr) {
				w.Header().Set("Retry-After", "60")
				http.Error(w, "rate limited", http.StatusTooManyRequests)
				return
			}
		}
		if r.URL.Path == "/mcp" {
			key := "ip:" + ipStr
			if tok := bearer(r); tok != "" {
				key = "tok:" + tokenID(tok)
			}
			if !o.Global.Allow("all") || !o.PerToken.Allow(key) {
				w.Header().Set("Retry-After", "60")
				http.Error(w, "rate limited", http.StatusTooManyRequests)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func bearer(r *http.Request) string {
	f := strings.Fields(r.Header.Get("Authorization"))
	if len(f) == 2 && strings.EqualFold(f[0], "bearer") {
		return f[1]
	}
	return ""
}

func tokenID(tok string) string {
	h := sha256.Sum256([]byte(tok))
	return hex.EncodeToString(h[:6])
}

// maxLoggedBody bounds what is read to find the JSON-RPC method; the SDK
// enforces its own request-size limit on the full body.
const maxLoggedBody = 64 << 10

// rpcFields names the JSON-RPC method, tool and argument keys of a /mcp call.
// Argument values are not logged, and responses are never read.
func rpcFields(r *http.Request) []any {
	body, err := io.ReadAll(io.LimitReader(r.Body, maxLoggedBody))
	rest := r.Body
	r.Body = struct {
		io.Reader
		io.Closer
	}{io.MultiReader(bytes.NewReader(body), rest), rest}
	if err != nil {
		return nil
	}
	var msg struct {
		Method string `json:"method"`
		Params struct {
			Name      string                     `json:"name"`
			Arguments map[string]json.RawMessage `json:"arguments"`
		} `json:"params"`
	}
	if json.Unmarshal(body, &msg) != nil || msg.Method == "" {
		return []any{"rpc", "unparsed"}
	}
	out := []any{"rpc", clip(msg.Method, 60)}
	if msg.Method == "tools/call" {
		keys := make([]string, 0, len(msg.Params.Arguments))
		for k := range msg.Params.Arguments {
			keys = append(keys, clip(k, 40))
		}
		out = append(out, "tool", clip(msg.Params.Name, 60), "arg_keys", strings.Join(keys, ","))
	}
	return out
}

func clip(s string, n int) string {
	if len(s) > n {
		return s[:n]
	}
	return s
}

type recorder struct {
	http.ResponseWriter
	status int
	bytes  int
	wrote  bool
}

func (r *recorder) WriteHeader(code int) {
	if !r.wrote {
		r.status, r.wrote = code, true
	}
	r.ResponseWriter.WriteHeader(code)
}

func (r *recorder) Write(b []byte) (int, error) {
	r.wrote = true
	n, err := r.ResponseWriter.Write(b)
	r.bytes += n
	return n, err
}

func (r *recorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *recorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }
