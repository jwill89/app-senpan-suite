package server_test

import (
	"io"
	"net/http"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"

	"app-suite/internal/web"
)

// Every answer the server gives - an operation's, the API's own refusals, /healthz, a
// data-folder file and the SPA's - says not to guess its type and keeps this site's
// addresses out of other sites' Referer, and none sends HSTS, which the vhost does.
func TestEveryAnswerCarriesTheBaselineHeaders(t *testing.T) {
	dataDir := t.TempDir()
	env := newTestEnvWithDataDir(t, dataDir)
	writeDataFile(t, dataDir, "images/logo.png", "logo")
	site, err := web.New(fstest.MapFS{
		"index.html":           {Data: []byte(`<meta property="csp-nonce" nonce="__CSP_NONCE__"><div id="app"></div>`)},
		"assets/index.js":      {Data: []byte("1")},
		"manifest.webmanifest": {Data: []byte("{}")},
	})
	if err != nil {
		t.Fatal(err)
	}
	env.srv.SetSite(site)

	for _, c := range []struct{ method, path string }{
		{http.MethodGet, "/api/v1/version"},
		{http.MethodGet, "/api/no-such-route"},
		{http.MethodDelete, "/api/v1/version"},
		{http.MethodOptions, "/api/v1/version"},
		{http.MethodGet, "/healthz"},
		{http.MethodGet, "/images/logo.png"},
		{http.MethodGet, "/images/missing.png"},
		{http.MethodGet, "/"},
		{http.MethodGet, "/admin/bingo"},
		{http.MethodGet, "/assets/index.js"},
		{http.MethodGet, "/assets/gone.js"},
	} {
		req, err := http.NewRequest(c.method, env.url(c.path), nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := env.client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		_ = resp.Body.Close()
		for name, want := range map[string]string{
			"X-Content-Type-Options":    "nosniff",
			"Referrer-Policy":           "same-origin",
			"Strict-Transport-Security": "",
		} {
			if got := resp.Header.Get(name); got != want {
				t.Errorf("%s %s (%d): %s %q, want %q", c.method, c.path, resp.StatusCode, name, got, want)
			}
		}
	}
}

// The app's page, served through the server as production serves it, names a fresh
// nonce in its policy and carries it in its body, and is never answered 304.
func TestThePageThroughTheServerCarriesItsNonce(t *testing.T) {
	env := newTestEnv(t)
	site, err := web.New(fstest.MapFS{
		"index.html": {Data: []byte(`<meta property="csp-nonce" nonce="__CSP_NONCE__"><div id="app"></div>`)},
	})
	if err != nil {
		t.Fatal(err)
	}
	env.srv.SetSite(site)

	nonceIn := regexp.MustCompile(`script-src 'self' 'nonce-([^']+)'`)
	var last string
	for range 2 {
		resp := env.getWithHeaders(t, "/raffles", map[string]string{"If-None-Match": "*"})
		raw, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		body := string(raw)
		policy := resp.Header.Get("Content-Security-Policy")
		m := nonceIn.FindStringSubmatch(policy)
		if resp.StatusCode != http.StatusOK || m == nil {
			t.Fatalf("GET /raffles: %d, policy %q", resp.StatusCode, policy)
		}
		if strings.Contains(policy, "'unsafe-inline'") {
			t.Fatalf("policy %q allows inline", policy)
		}
		if !strings.Contains(body, `nonce="`+m[1]+`"`) {
			t.Fatalf("body %q lacks the policy's nonce %q", body, m[1])
		}
		if m[1] == last {
			t.Fatalf("nonce %q served twice", last)
		}
		last = m[1]
	}
}
