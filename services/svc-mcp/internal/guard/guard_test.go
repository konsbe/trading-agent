package guard

import (
	"net/http/httptest"
	"net/netip"
	"testing"
	"time"
)

func TestResolveIPTrustsHeaderOnlyFromProxy(t *testing.T) {
	trusted := []netip.Prefix{netip.MustParsePrefix("172.16.0.0/12")}
	cases := []struct {
		peer, header, want string
	}{
		{"172.18.0.5:4000", "160.79.104.9", "160.79.104.9"},
		{"203.0.113.9:4000", "160.79.104.9", "203.0.113.9"}, // spoof from outside
		{"172.18.0.5:4000", "", "172.18.0.5"},
		{"172.18.0.5:4000", "not-an-ip", "172.18.0.5"},
		{"[::ffff:172.18.0.5]:4000", "160.79.104.9", "160.79.104.9"},
	}
	for _, c := range cases {
		r := httptest.NewRequest("GET", "/mcp", nil)
		r.RemoteAddr = c.peer
		if c.header != "" {
			r.Header.Set("CF-Connecting-IP", c.header)
		}
		if got := resolveIP(r, trusted).String(); got != c.want {
			t.Errorf("peer %s header %q: got %s want %s", c.peer, c.header, got, c.want)
		}
	}
}

func TestLockout(t *testing.T) {
	now := time.Unix(0, 0)
	l := NewLockout(3, 15*time.Minute, func() time.Time { return now })
	l.Fail("a")
	l.Fail("a")
	if l.Locked("a") {
		t.Fatal("locked after 2 of 3")
	}
	l.Fail("a")
	if !l.Locked("a") || l.Locked("b") {
		t.Fatal("want a locked, b not")
	}
	now = now.Add(16 * time.Minute)
	if l.Locked("a") {
		t.Fatal("still locked after the window")
	}
}

func TestLockoutForgetsOldFailures(t *testing.T) {
	now := time.Unix(0, 0)
	l := NewLockout(3, 15*time.Minute, func() time.Time { return now })
	l.Fail("a")
	l.Fail("a")
	now = now.Add(20 * time.Minute)
	l.Fail("a")
	if l.Locked("a") {
		t.Fatal("failures outside the window counted")
	}
}

func TestLimiter(t *testing.T) {
	now := time.Unix(0, 0)
	l := NewLimiter(60, func() time.Time { return now })
	for i := range 60 {
		if !l.Allow("k") {
			t.Fatalf("call %d refused", i)
		}
	}
	if l.Allow("k") {
		t.Fatal("61st call in the same instant allowed")
	}
	if !l.Allow("other") {
		t.Fatal("keys share a bucket")
	}
	now = now.Add(time.Second)
	if !l.Allow("k") {
		t.Fatal("no refill after 1s at 60/min")
	}
	if l.Allow("k") {
		t.Fatal("refilled more than one token in 1s")
	}
}
