package web

import (
	"context"
	"encoding/base64"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"testing/fstest"
)

// A build as Vite lays it out: the page, a hashed bundle, the service worker with its
// registration, Workbox, the manifest and a sound.
var build = fstest.MapFS{
	"index.html": {Data: []byte(`<!doctype html><meta property="csp-nonce" nonce="__CSP_NONCE__">` +
		`<div id="app"></div><script type="module" nonce="__CSP_NONCE__" src="/assets/index-abc123.js"></script>`)},
	"assets/index-abc123.js": {Data: []byte("console.log(1)")},
	"sw.js":                  {Data: []byte("self.skipWaiting()")},
	"registerSW.js":          {Data: []byte("navigator.serviceWorker")},
	"workbox-dcde9eb3.js":    {Data: []byte("workbox")},
	"manifest.webmanifest":   {Data: []byte(`{"name":"Senpan"}`)},
	"sounds/queue_pop.mp3":   {Data: []byte("ID3")},
}

func get(t *testing.T, h http.Handler, method, target string, header map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	r := httptest.NewRequest(method, target, nil)
	for k, v := range header {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func newHandler(t *testing.T) *Handler {
	t.Helper()
	h, err := New(build)
	if err != nil {
		t.Fatal(err)
	}
	return h
}

func TestPagesServeTheAppNeverCached(t *testing.T) {
	h := newHandler(t)
	for _, target := range []string{"/", "/admin/bingo", "/raffles/12", "/index.html"} {
		w := get(t, h, http.MethodGet, target, nil)
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `id="app"`) {
			t.Fatalf("%s: %d %q, want the app's page", target, w.Code, w.Body.String())
		}
		if got := w.Header().Get("Cache-Control"); got != noStore {
			t.Fatalf("%s: Cache-Control %q, want %q", target, got, noStore)
		}
		if got := w.Header().Get("Content-Type"); !strings.HasPrefix(got, "text/html") {
			t.Fatalf("%s: Content-Type %q", target, got)
		}
		if got := w.Header().Get("ETag"); got != "" {
			t.Fatalf("%s: ETag %q on the page, whose body changes with every answer", target, got)
		}
	}
}

// pageNonce is the nonce a page's policy names in script-src, which must be the one
// in style-src too.
func pageNonce(t *testing.T, policy string) string {
	t.Helper()
	nonces := regexp.MustCompile(`'nonce-([^']+)'`).FindAllStringSubmatch(policy, -1)
	if len(nonces) != 2 || nonces[0][1] != nonces[1][1] {
		t.Fatalf("policy %q: want one nonce in script-src and style-src", policy)
	}
	return nonces[0][1]
}

func TestEveryPageAnswerHasAFreshNonceItsBodyCarries(t *testing.T) {
	h := newHandler(t)
	seen := map[string]bool{}
	for _, target := range []string{"/", "/", "/admin/bingo", "/index.html"} {
		w := get(t, h, http.MethodGet, target, nil)
		nonce := pageNonce(t, w.Header().Get("Content-Security-Policy"))
		if raw, err := base64.StdEncoding.DecodeString(nonce); err != nil || len(raw) != 16 {
			t.Fatalf("%s: nonce %q is not 128 bits of base64", target, nonce)
		}
		if seen[nonce] {
			t.Fatalf("%s: nonce %q was used before", target, nonce)
		}
		seen[nonce] = true
		body := w.Body.String()
		if strings.Contains(body, noncePlaceholder) {
			t.Fatalf("%s: the placeholder is left in the page: %s", target, body)
		}
		if got := strings.Count(body, `nonce="`+nonce+`"`); got != 2 {
			t.Fatalf("%s: the page carries its nonce %d times, want 2: %s", target, got, body)
		}
		if got := w.Header().Get("Content-Length"); got != strconv.Itoa(len(body)) {
			t.Fatalf("%s: Content-Length %s for %d bytes", target, got, len(body))
		}
	}
}

// The policy admits nothing inline but by nonce, and names hosts, never a scheme.
func TestThePolicyAllowsNoInlineAndNoWildcard(t *testing.T) {
	h := newHandler(t)
	for _, target := range []string{"/", "/assets/index-abc123.js", "/sw.js", "/gone.js"} {
		policy := get(t, h, http.MethodGet, target, nil).Header().Get("Content-Security-Policy")
		directives := map[string][]string{}
		for directive := range strings.SplitSeq(policy, ";") {
			fields := strings.Fields(directive)
			if len(fields) > 0 {
				directives[fields[0]] = fields[1:]
			}
		}
		for name, sources := range directives {
			for _, source := range sources {
				if source == "'unsafe-inline'" || source == "'unsafe-eval'" || source == "https:" ||
					source == "http:" || strings.Contains(source, "*") {
					t.Fatalf("%s: %s allows %s: %q", target, name, source, policy)
				}
			}
		}
		// Fonts are the site's own: no font service is named anywhere.
		if strings.Contains(policy, "fonts.googleapis.com") || strings.Contains(policy, "fonts.gstatic.com") {
			t.Fatalf("%s: the policy names a font service: %q", target, policy)
		}
		for directive, want := range map[string]string{
			"default-src": "'self'", "frame-ancestors": "'none'", "object-src": "'none'", "base-uri": "'self'",
			"font-src": "'self'",
		} {
			if got := strings.Join(directives[directive], " "); got != want {
				t.Fatalf("%s: %s %q, want %q", target, directive, got, want)
			}
		}
		if target != "/" && strings.Contains(policy, "'nonce-") {
			t.Fatalf("%s: a file's policy names a nonce: %q", target, policy)
		}
	}
}

// A page kept from an earlier answer carries a nonce the next answer's policy doesn't
// name, so a revalidation always gets the page anew.
func TestThePageIsNeverAnswered304(t *testing.T) {
	h := newHandler(t)
	for _, inm := range []string{"*", `"0123456789abcdef"`} {
		w := get(t, h, http.MethodGet, "/", map[string]string{"If-None-Match": inm})
		if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `id="app"`) {
			t.Fatalf("If-None-Match %s: %d, want the page", inm, w.Code)
		}
	}
}

func TestReportOnlySendsThePolicyToReport(t *testing.T) {
	h := newHandler(t)
	h.ReportOnly = true
	for _, target := range []string{"/", "/assets/index-abc123.js"} {
		w := get(t, h, http.MethodGet, target, nil)
		if got := w.Header().Get("Content-Security-Policy"); got != "" {
			t.Fatalf("%s: enforced policy %q in report-only mode", target, got)
		}
		if got := w.Header().Get("Content-Security-Policy-Report-Only"); !strings.Contains(got, "default-src 'self'") {
			t.Fatalf("%s: report-only policy %q", target, got)
		}
	}
	if w := get(t, h, http.MethodGet, "/", nil); !strings.Contains(w.Header().Get("Content-Security-Policy-Report-Only"), "'nonce-") {
		t.Fatal("the page's report-only policy names no nonce")
	}
}

func TestFilesCarryTheirTypeAndCache(t *testing.T) {
	h := newHandler(t)
	cases := []struct{ target, contentType, cache string }{
		{"/assets/index-abc123.js", "text/javascript", immutable},
		{"/workbox-dcde9eb3.js", "text/javascript", immutable},
		{"/sw.js", "text/javascript", noStore},
		{"/registerSW.js", "text/javascript", noStore},
		{"/manifest.webmanifest", "application/manifest+json", revalidate},
		{"/sounds/queue_pop.mp3", "audio/mpeg", revalidate},
	}
	for _, c := range cases {
		w := get(t, h, http.MethodGet, c.target, nil)
		if w.Code != http.StatusOK {
			t.Fatalf("%s: %d", c.target, w.Code)
		}
		if got := w.Header().Get("Content-Type"); !strings.HasPrefix(got, c.contentType) {
			t.Fatalf("%s: Content-Type %q, want %q", c.target, got, c.contentType)
		}
		if got := w.Header().Get("Cache-Control"); got != c.cache {
			t.Fatalf("%s: Cache-Control %q, want %q", c.target, got, c.cache)
		}
	}
}

func TestEveryResponseCarriesTheSecurityHeaders(t *testing.T) {
	h := newHandler(t)
	for _, c := range []struct{ method, target string }{
		{http.MethodGet, "/"}, {http.MethodHead, "/admin"}, {http.MethodGet, "/assets/index-abc123.js"},
		{http.MethodGet, "/gone.js"}, {http.MethodPost, "/"}, {http.MethodPost, "/sw.js"},
	} {
		w := get(t, h, c.method, c.target, nil)
		for name, want := range map[string]string{
			"X-Content-Type-Options": "nosniff",
			"Referrer-Policy":        "same-origin",
			"X-Frame-Options":        "DENY",
		} {
			if got := w.Header().Get(name); got != want {
				t.Fatalf("%s %s: %s %q, want %q", c.method, c.target, name, got, want)
			}
		}
		if w.Header().Get("Permissions-Policy") == "" {
			t.Fatalf("%s %s: no Permissions-Policy", c.method, c.target)
		}
		if csp := w.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") {
			t.Fatalf("%s %s: CSP %q", c.method, c.target, csp)
		}
		// The vhost sends HSTS; sent here too, a response would carry it twice.
		if got := w.Header().Get("Strict-Transport-Security"); got != "" {
			t.Fatalf("%s %s: Strict-Transport-Security %q", c.method, c.target, got)
		}
	}
}

// A missing file is a 404, never the app's page served under a script's or image's name.
func TestMissingFilesAndOtherFoldersAre404(t *testing.T) {
	h := newHandler(t)
	for _, target := range []string{
		"/assets/gone-999.js", "/favicon.ico", "/sounds/none.mp3",
		"/api", "/api/no-such-route",
		"/images/logo.png", "/images", "/fonts/x.woff2", "/carrd/site/", "/plugin/pluginmaster.json",
	} {
		if w := get(t, h, http.MethodGet, target, nil); w.Code != http.StatusNotFound {
			t.Fatalf("%s: %d, want 404", target, w.Code)
		}
	}
	// A page whose name only starts like a folder is still a page.
	if w := get(t, h, http.MethodGet, "/imagesettings", nil); w.Code != http.StatusOK {
		t.Fatalf("/imagesettings: %d, want the app's page", w.Code)
	}
}

func TestARevalidationIsAnswered304(t *testing.T) {
	h := newHandler(t)
	first := get(t, h, http.MethodGet, "/manifest.webmanifest", nil)
	etag := first.Header().Get("ETag")
	if etag == "" {
		t.Fatal("no ETag")
	}
	if w := get(t, h, http.MethodGet, "/manifest.webmanifest", map[string]string{"If-None-Match": etag}); w.Code != http.StatusNotModified {
		t.Fatalf("If-None-Match: %d, want 304", w.Code)
	}
}

func TestOnlyGetAndHeadAreAnswered(t *testing.T) {
	h := newHandler(t)
	if w := get(t, h, http.MethodHead, "/", nil); w.Code != http.StatusOK || w.Body.Len() != 0 || w.Header().Get("Content-Length") == "" {
		t.Fatalf("HEAD /: %d, %d bytes, Content-Length %q", w.Code, w.Body.Len(), w.Header().Get("Content-Length"))
	}
	w := get(t, h, http.MethodPost, "/", nil)
	if w.Code != http.StatusMethodNotAllowed || w.Header().Get("Allow") != "GET, HEAD" {
		t.Fatalf("POST /: %d, Allow %q", w.Code, w.Header().Get("Allow"))
	}
}

func TestABuildWithoutItsPageIsRefused(t *testing.T) {
	if _, err := New(fstest.MapFS{"assets/a.js": {Data: []byte("x")}}); err == nil {
		t.Fatal("a build without index.html was accepted")
	}
}

// The binary's own build has its page, so a release never ships without the front end.
func TestTheEmbeddedBuildHasItsPage(t *testing.T) {
	if _, err := New(Dist()); err != nil {
		t.Fatal(err)
	}
}

// directive is the value of one directive of a policy.
func directive(policy, name string) string {
	for _, part := range strings.Split(policy, ";") {
		if fields := strings.Fields(part); len(fields) > 0 && fields[0] == name {
			return strings.Join(fields[1:], " ")
		}
	}
	return ""
}

// An embed's page may be framed by this site and the allowlist's sites, with no
// X-Frame-Options to refuse them; every other page still refuses every framer, and an
// entry that isn't a plain origin never reaches the policy.
func TestAnEmbedsPageMayBeFramedByTheAllowedSitesAlone(t *testing.T) {
	h := newHandler(t)
	h.EmbedAncestors = func(context.Context) []string {
		return []string{"https://senpan.cafe", "https://*.senpan.cafe", "https://evil.example; script-src *", "*"}
	}

	w := get(t, h, http.MethodGet, "/embed/festival-maps/obon-2026", nil)
	policy := w.Header().Get("Content-Security-Policy")
	if got := directive(policy, "frame-ancestors"); got != "'self' https://senpan.cafe https://*.senpan.cafe" {
		t.Errorf("embed frame-ancestors = %q", got)
	}
	if strings.Count(policy, "script-src") != 1 {
		t.Errorf("an allowlist entry reached the policy: %s", policy)
	}
	if xfo := w.Header().Get("X-Frame-Options"); xfo != "" {
		t.Errorf("embed X-Frame-Options = %q, want none", xfo)
	}

	for _, target := range []string{"/", "/festival-maps/obon-2026", "/admin/login", "/embedded"} {
		w := get(t, h, http.MethodGet, target, nil)
		if got := directive(w.Header().Get("Content-Security-Policy"), "frame-ancestors"); got != "'none'" {
			t.Errorf("%s frame-ancestors = %q, want 'none'", target, got)
		}
		if xfo := w.Header().Get("X-Frame-Options"); xfo != "DENY" {
			t.Errorf("%s X-Frame-Options = %q, want DENY", target, xfo)
		}
	}

	h.EmbedAncestors = func(context.Context) []string { return nil }
	w = get(t, h, http.MethodGet, "/embed/festival-maps/1", nil)
	if got := directive(w.Header().Get("Content-Security-Policy"), "frame-ancestors"); got != "'self'" {
		t.Errorf("an empty allowlist: frame-ancestors = %q, want only 'self'", got)
	}
}
