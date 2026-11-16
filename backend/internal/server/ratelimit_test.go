package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestClientIP(t *testing.T) {
	// From the loopback proxy, the address is the last X-Forwarded-For entry, the one
	// Apache appends; what a client put before it, and CF-Connecting-IP, are never
	// read. Any other peer is taken at its own address.
	cases := []struct {
		name   string
		xff    []string
		cf     string
		remote string
		want   string
	}{
		{"loopback takes the last entry", []string{"1.2.3.4, 5.6.7.8, 9.9.9.9"}, "", "127.0.0.1:5555", "9.9.9.9"},
		{"a forged leftmost entry is ignored", []string{"6.6.6.6, 203.0.113.9"}, "", "127.0.0.1:5555", "203.0.113.9"},
		{"loopback trims space", []string{" 1.2.3.4 , 5.6.7.8 "}, "", "127.0.0.1:5555", "5.6.7.8"},
		{"cf-connecting-ip is never read", []string{"203.0.113.9"}, "8.8.8.8", "127.0.0.1:5555", "203.0.113.9"},
		{"cf-connecting-ip alone is never read", nil, "8.8.8.8", "127.0.0.1:5555", "127.0.0.1"},
		{"the last of several header lines", []string{"6.6.6.6", "203.0.113.9"}, "", "127.0.0.1:5555", "203.0.113.9"},
		{"ipv6 loopback peer", []string{"2001:db8::1"}, "", "[::1]:5555", "2001:db8::1"},
		{"ipv6 entry in canonical form", []string{"2001:DB8:0:0::1"}, "", "127.0.0.1:5555", "2001:db8::1"},
		{"ipv6 entry with a port", []string{"[2001:db8::1]:443"}, "", "127.0.0.1:5555", "2001:db8::1"},
		{"ipv4 entry with a port", []string{"203.0.113.9:443"}, "", "127.0.0.1:5555", "203.0.113.9"},
		{"ipv4-mapped entry", []string{"::ffff:203.0.113.9"}, "", "127.0.0.1:5555", "203.0.113.9"},
		{"ipv4-mapped loopback peer", []string{"203.0.113.9"}, "", "[::ffff:127.0.0.1]:5555", "203.0.113.9"},
		{"a last entry that is no address keeps the peer", []string{"203.0.113.9, unknown"}, "", "127.0.0.1:5555", "127.0.0.1"},
		{"an empty last entry keeps the peer", []string{"203.0.113.9,"}, "", "[::1]:5555", "::1"},
		{"loopback without headers keeps the peer", nil, "", "127.0.0.1:5555", "127.0.0.1"},
		// Off-loopback peer: the headers are anyone's to send, so the peer's own address.
		{"non-loopback ignores xff", []string{"1.2.3.4"}, "8.8.8.8", "9.9.9.9:5555", "9.9.9.9"},
		{"non-loopback ipv6 ignores xff", []string{"1.2.3.4"}, "", "[2001:db8::7]:5555", "2001:db8::7"},
		{"private peer is not the proxy", []string{"1.2.3.4"}, "", "10.0.0.2:5555", "10.0.0.2"},
		{"remoteaddr without port", nil, "", "9.9.9.9", "9.9.9.9"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = c.remote
			for _, line := range c.xff {
				r.Header.Add("X-Forwarded-For", line)
			}
			if c.cf != "" {
				r.Header.Set("CF-Connecting-IP", c.cf)
			}
			if got := clientIP(r); got != c.want {
				t.Fatalf("clientIP = %q, want %q", got, c.want)
			}
		})
	}
}

func TestRateLimiterThreshold(t *testing.T) {
	rl := newRateLimiter(3, time.Hour)
	defer close(rl.stop)
	const ip = "1.2.3.4"

	if rl.isLimited(ip) {
		t.Fatal("a fresh IP must not be limited")
	}
	rl.recordFailure(ip)
	rl.recordFailure(ip)
	if rl.isLimited(ip) {
		t.Fatal("2 failures (< 3) must not be limited")
	}
	rl.recordFailure(ip) // count == 3 == maxFails
	if !rl.isLimited(ip) {
		t.Fatal("3 failures (>= maxFails) must be limited")
	}

	// A successful login clears the IP.
	rl.resetFailures(ip)
	if rl.isLimited(ip) {
		t.Fatal("after resetFailures the IP must not be limited")
	}
}

func TestRateLimiterWindowExpiry(t *testing.T) {
	// Construct directly (no background goroutine) so we can age the window.
	rl := &rateLimiter{
		attempts: map[string]*attemptInfo{},
		maxFails: 1,
		window:   50 * time.Millisecond,
	}
	const ip = "5.6.7.8"

	rl.recordFailure(ip)
	if !rl.isLimited(ip) {
		t.Fatal("1 failure (>= maxFails 1) must be limited")
	}

	// Age the window past expiry; isLimited should both report unlimited and
	// purge the stale entry.
	rl.attempts[ip].windowStart = time.Now().Add(-time.Hour)
	if rl.isLimited(ip) {
		t.Fatal("an expired window must not be limited")
	}
	if _, ok := rl.attempts[ip]; ok {
		t.Fatal("isLimited must delete the expired entry")
	}
}

func TestRateLimiterRecordAfterWindowResets(t *testing.T) {
	rl := &rateLimiter{
		attempts: map[string]*attemptInfo{},
		maxFails: 2,
		window:   50 * time.Millisecond,
	}
	const ip = "7.7.7.7"
	rl.recordFailure(ip)
	rl.recordFailure(ip)
	if !rl.isLimited(ip) {
		t.Fatal("2 >= 2 should be limited")
	}
	// Expire, then a new failure should start a fresh window at count 1.
	rl.attempts[ip].windowStart = time.Now().Add(-time.Hour)
	rl.recordFailure(ip)
	if rl.attempts[ip].count != 1 {
		t.Fatalf("expected count reset to 1 after window expiry, got %d", rl.attempts[ip].count)
	}
}

func TestRateLimiterCountsAnIPv6ClientByItsSlash64(t *testing.T) {
	rl := newRateLimiter(2, time.Hour)
	defer close(rl.stop)

	// Two addresses in one /64 share a count; one in the next /64 doesn't.
	rl.Fail("2001:db8:1:2::1")
	rl.Fail("2001:db8:1:2:ffff::9")
	if !rl.isLimited("2001:db8:1:2::abcd") {
		t.Fatal("addresses in one /64 must count together")
	}
	if rl.isLimited("2001:db8:1:3::1") {
		t.Fatal("an address in another /64 must count apart")
	}
	if rl.RetryAfter("2001:db8:1:2::77") == 0 {
		t.Fatal("RetryAfter must answer for the whole /64")
	}
	rl.Reset("2001:db8:1:2::5")
	if rl.isLimited("2001:db8:1:2::1") {
		t.Fatal("Reset must clear the whole /64")
	}

	// IPv4, an IPv4 address in IPv6 form, and a key that isn't an address count as they are.
	rl.Fail("1.2.3.4")
	rl.Fail("::ffff:1.2.3.4")
	if !rl.isLimited("1.2.3.4") {
		t.Fatal("an IPv4 address and its IPv6 form must count together")
	}
	if rl.isLimited("1.2.3.5") {
		t.Fatal("IPv4 addresses must count one by one")
	}
	if got := limitKey("not-an-address"); got != "not-an-address" {
		t.Fatalf("limitKey(non-address) = %q", got)
	}
}
