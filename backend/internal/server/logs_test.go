package server_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"app-suite/internal/server"
)

func TestLogClientIP_TakesTheAddressTheProxyAppended(t *testing.T) {
	r := httptest.NewRequest(http.MethodGet, "/api/x", nil)
	r.RemoteAddr = "[::1]:44686"
	r.Header.Set("X-Forwarded-For", "203.0.113.9, 198.51.100.7")
	r.Header.Set("CF-Connecting-IP", "192.0.2.44")
	// The last X-Forwarded-For entry; CF-Connecting-IP is never read.
	if got := server.LogClientIPForTest(r); got != "198.51.100.7" {
		t.Errorf("XFF last: got %q; want 198.51.100.7", got)
	}
	// Without the header, the RemoteAddr host (port stripped).
	r.Header.Del("X-Forwarded-For")
	if got := server.LogClientIPForTest(r); got != "::1" {
		t.Errorf("RemoteAddr host: got %q; want ::1", got)
	}
}

func TestLogClientIP_IgnoresForgedHeadersFromNonLoopback(t *testing.T) {
	// A client that reaches the backend directly (RemoteAddr is NOT the loopback
	// proxy) must not be able to forge its logged IP via X-Forwarded-For.
	r := httptest.NewRequest(http.MethodGet, "/api/x", nil)
	r.RemoteAddr = "1.2.3.4:5555"
	r.Header.Set("CF-Connecting-IP", "9.9.9.9")
	r.Header.Set("X-Forwarded-For", "8.8.8.8")
	if got := server.LogClientIPForTest(r); got != "1.2.3.4" {
		t.Errorf("forged headers from non-loopback peer: got %q; want 1.2.3.4 (RemoteAddr)", got)
	}

	// From the trusted loopback proxy, the last X-Forwarded-For entry is honored.
	r.RemoteAddr = "127.0.0.1:5555"
	if got := server.LogClientIPForTest(r); got != "8.8.8.8" {
		t.Errorf("from loopback proxy: got %q; want 8.8.8.8 (X-Forwarded-For)", got)
	}
}
