package server

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
	"sync"
	"time"
)

// rateLimiter tracks failed attempts per IP for brute-force protection.
type rateLimiter struct {
	mu       sync.Mutex
	attempts map[string]*attemptInfo
	maxFails int
	window   time.Duration
	stop     chan struct{}
}

// attemptInfo tracks the number of failed attempts from a single IP within a
// time window that starts at the first one.
type attemptInfo struct {
	count       int
	windowStart time.Time
}

// newRateLimiter creates a rate limiter that allows maxFails failed attempts
// per IP within the given window duration. Starts a background goroutine
// that periodically purges expired entries. Call close(rl.stop) to terminate.
func newRateLimiter(maxFails int, window time.Duration) *rateLimiter {
	rl := &rateLimiter{
		attempts: make(map[string]*attemptInfo),
		maxFails: maxFails,
		window:   window,
		stop:     make(chan struct{}),
	}
	// Periodically clean up expired entries.
	go func() {
		ticker := time.NewTicker(window)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				rl.cleanup()
			case <-rl.stop:
				return
			}
		}
	}()
	return rl
}

// clientIP is the address the rate limiters count a request against: the same one
// the access log records (logClientIP), so a throttled address can be found in the log.
func clientIP(r *http.Request) string {
	return logClientIP(r)
}

// logClientIP is the visitor's address. The app listens on the loopback behind
// Apache, which restores the visitor's address from Cloudflare (mod_remoteip) and
// appends it to X-Forwarded-For, so the header's LAST entry is the one Apache wrote.
// The entries before it are whatever the client sent and are never read, and nor is
// CF-Connecting-IP: Apache has already turned it into the address it appends.
//
// The header is read only when the peer is on the loopback, where only the proxy
// reaches the app; any other peer is taken at its own address, so a client that
// reaches the app directly can't name another. A last entry that is no address
// leaves the peer's.
func logClientIP(r *http.Request) string {
	peer := r.RemoteAddr
	if host, _, err := net.SplitHostPort(peer); err == nil {
		peer = host
	}
	addr, err := netip.ParseAddr(peer)
	if err != nil {
		return peer
	}
	if addr.Unmap().IsLoopback() {
		if forwarded, ok := lastForwarded(r.Header.Values("X-Forwarded-For")); ok {
			return forwarded.String()
		}
	}
	return canonical(addr).String()
}

// lastForwarded is the last address in the X-Forwarded-For lines, taken as one list.
// An entry with a port ("192.0.2.1:443", "[2001:db8::1]:443") is read without it.
func lastForwarded(lines []string) (netip.Addr, bool) {
	list := strings.Join(lines, ",")
	last := strings.TrimSpace(list[strings.LastIndexByte(list, ',')+1:])
	if addr, err := netip.ParseAddr(last); err == nil {
		return canonical(addr), true
	}
	if addrPort, err := netip.ParseAddrPort(last); err == nil {
		return canonical(addrPort.Addr()), true
	}
	return netip.Addr{}, false
}

// canonical is addr in the one form each address is counted under: an IPv4 address
// in IPv6 form as plain IPv4, with no IPv6 zone.
func canonical(addr netip.Addr) netip.Addr {
	return addr.Unmap().WithZone("")
}

// limitKey is the key a client's attempts are counted under: an IPv6 address counts by
// its /64, the block one subscriber is usually given, so stepping through the addresses
// of one network gains nothing; an IPv4 address, or a key that isn't an address, counts
// as it is.
func limitKey(key string) string {
	addr, err := netip.ParseAddr(key)
	if err != nil {
		return key
	}
	addr = canonical(addr)
	if !addr.Is6() {
		return addr.String()
	}
	prefix, err := addr.Prefix(64)
	if err != nil {
		return key
	}
	return prefix.String()
}

// isLimited returns true if the given IP has exceeded the failure limit.
func (rl *rateLimiter) isLimited(ip string) bool {
	ip = limitKey(ip)
	rl.mu.Lock()
	defer rl.mu.Unlock()
	info, ok := rl.attempts[ip]
	if !ok {
		return false
	}
	if time.Since(info.windowStart) > rl.window {
		delete(rl.attempts, ip)
		return false
	}
	return info.count >= rl.maxFails
}

// recordFailure increments the failure count for an IP.
func (rl *rateLimiter) recordFailure(ip string) {
	ip = limitKey(ip)
	rl.mu.Lock()
	defer rl.mu.Unlock()
	info, ok := rl.attempts[ip]
	if !ok || time.Since(info.windowStart) > rl.window {
		rl.attempts[ip] = &attemptInfo{count: 1, windowStart: time.Now()}
		return
	}
	info.count++
}

// resetFailures clears the failure count for an IP (on successful login).
func (rl *rateLimiter) resetFailures(ip string) {
	ip = limitKey(ip)
	rl.mu.Lock()
	defer rl.mu.Unlock()
	delete(rl.attempts, ip)
}

// RetryAfter is how long until key may try again: 0 when it may try at once, else the
// rest of the window it used up (api.Limiter).
func (rl *rateLimiter) RetryAfter(key string) time.Duration {
	key = limitKey(key)
	rl.mu.Lock()
	defer rl.mu.Unlock()
	info, ok := rl.attempts[key]
	if !ok || info.count < rl.maxFails {
		return 0
	}
	return max(time.Until(info.windowStart.Add(rl.window)), 0)
}

// Fail counts an attempt against key (api.Limiter).
func (rl *rateLimiter) Fail(key string) { rl.recordFailure(key) }

// Reset forgets key's attempts (api.Limiter).
func (rl *rateLimiter) Reset(key string) { rl.resetFailures(key) }

// cleanup removes expired entries.
func (rl *rateLimiter) cleanup() {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	now := time.Now()
	for ip, info := range rl.attempts {
		if now.Sub(info.windowStart) > rl.window {
			delete(rl.attempts, ip)
		}
	}
}
