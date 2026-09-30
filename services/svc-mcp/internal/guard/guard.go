// Package guard is the HTTP front of svc-mcp: client IP, Anthropic IP
// allowlist, rate limits, sign-in lockout and request logging.
package guard

import (
	"context"
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"
)

type ipKey struct{}

// ClientIP is the address resolved by WithClientIP ("" outside it).
func ClientIP(ctx context.Context) string {
	s, _ := ctx.Value(ipKey{}).(string)
	return s
}

func withIP(ctx context.Context, ip string) context.Context {
	return context.WithValue(ctx, ipKey{}, ip)
}

// resolveIP believes CF-Connecting-IP only from a trusted proxy (cloudflared
// on the Compose network); from any other peer the header is ignored.
func resolveIP(r *http.Request, trusted []netip.Prefix) netip.Addr {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	peer, err := netip.ParseAddr(host)
	if err != nil {
		return netip.Addr{}
	}
	peer = peer.Unmap()
	if contains(trusted, peer) {
		if h := strings.TrimSpace(r.Header.Get("CF-Connecting-IP")); h != "" {
			if a, err := netip.ParseAddr(h); err == nil {
				return a.Unmap()
			}
		}
	}
	return peer
}

func contains(ps []netip.Prefix, a netip.Addr) bool {
	for _, p := range ps {
		if p.Contains(a) {
			return true
		}
	}
	return false
}

// Lockout blocks the sign-in endpoints for an IP after repeated failures.
type Lockout struct {
	max    int
	window time.Duration
	now    func() time.Time

	mu    sync.Mutex
	fails map[string][]time.Time
	until map[string]time.Time
}

func NewLockout(max int, window time.Duration, now func() time.Time) *Lockout {
	if now == nil {
		now = time.Now
	}
	return &Lockout{max: max, window: window, now: now, fails: map[string][]time.Time{}, until: map[string]time.Time{}}
}

func (l *Lockout) Fail(ip string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	cut := now.Add(-l.window)
	keep := l.fails[ip][:0]
	for _, t := range l.fails[ip] {
		if t.After(cut) {
			keep = append(keep, t)
		}
	}
	keep = append(keep, now)
	l.fails[ip] = keep
	if len(keep) >= l.max {
		l.until[ip] = now.Add(l.window)
		delete(l.fails, ip)
	}
}

func (l *Lockout) Locked(ip string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	u, ok := l.until[ip]
	if ok && l.now().After(u) {
		delete(l.until, ip)
		return false
	}
	return ok
}

// Limiter is a token bucket per key: perMin tokens, refilled continuously.
type Limiter struct {
	perMin float64
	now    func() time.Time

	mu      sync.Mutex
	buckets map[string]*bucket
	swept   time.Time
}

type bucket struct {
	tokens float64
	last   time.Time
}

func NewLimiter(perMin int, now func() time.Time) *Limiter {
	if now == nil {
		now = time.Now
	}
	return &Limiter{perMin: float64(perMin), now: now, buckets: map[string]*bucket{}}
}

func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	if now.Sub(l.swept) > 10*time.Minute {
		for k, b := range l.buckets {
			if now.Sub(b.last) > 10*time.Minute {
				delete(l.buckets, k)
			}
		}
		l.swept = now
	}
	b, ok := l.buckets[key]
	if !ok {
		b = &bucket{tokens: l.perMin, last: now}
		l.buckets[key] = b
	}
	b.tokens = min(l.perMin, b.tokens+now.Sub(b.last).Minutes()*l.perMin)
	b.last = now
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}
