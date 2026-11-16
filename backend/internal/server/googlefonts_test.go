package server_test

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// googleStandIn stands in for Google Fonts: the CSS API at /css2, whose stylesheet for
// a family names two files under /s/, and those files. A family it is told is
// unknown answers 400, and one sent foreign answers a stylesheet naming a file off
// the font host.
type googleStandIn struct {
	srv          *httptest.Server
	mu           sync.Mutex
	unknown      map[string]bool
	foreign      map[string]bool
	cssRequests  atomic.Int32
	fileRequests atomic.Int32
}

func newGoogleStandIn(t *testing.T) *googleStandIn {
	t.Helper()
	g := &googleStandIn{unknown: map[string]bool{}, foreign: map[string]bool{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /css2", func(w http.ResponseWriter, r *http.Request) {
		g.cssRequests.Add(1)
		raw, _, _ := strings.Cut(strings.TrimPrefix(r.URL.RawQuery, "family="), ":")
		family := strings.ReplaceAll(raw, "+", " ")
		g.mu.Lock()
		unknown, foreign := g.unknown[family], g.foreign[family]
		g.mu.Unlock()
		if unknown || strings.HasPrefix(family, "Nope") {
			http.Error(w, "Missing font family", http.StatusBadRequest)
			return
		}
		host := g.srv.URL
		if foreign {
			host = "https://elsewhere.example"
		}
		slug := strings.ToLower(strings.ReplaceAll(family, " ", ""))
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = fmt.Fprintf(w, "/* latin */\n@font-face {\n  font-family: '%s';\n  src: url(%s/s/%s/a.woff2) format('woff2');\n}\n"+
			"/* latin-ext */\n@font-face {\n  font-family: '%[1]s';\n  src: url(%[2]s/s/%[3]s/b.woff2) format('woff2');\n}\n",
			family, host, slug)
	})
	mux.HandleFunc("GET /s/", func(w http.ResponseWriter, r *http.Request) {
		g.fileRequests.Add(1)
		w.Header().Set("Content-Type", "font/woff2")
		_, _ = io.WriteString(w, googleFontBytes(r.URL.Path))
	})
	g.srv = httptest.NewServer(mux)
	t.Cleanup(g.srv.Close)
	return g
}

// googleFontBytes are the bytes the stand-in serves at path: WOFF2's magic and the path.
func googleFontBytes(path string) string { return "wOF2" + path }

func googleFontName(path string) string {
	sum := sha256.Sum256([]byte(googleFontBytes(path)))
	return hex.EncodeToString(sum[:]) + ".woff2"
}

// googleFontEnv is a test env fetching Google Fonts from a stand-in.
func googleFontEnv(t *testing.T) (*testEnv, *googleStandIn) {
	t.Helper()
	env := newTestEnv(t)
	google := newGoogleStandIn(t)
	env.srv.RouteGoogleFontsForTest(google.srv.URL)
	return env, google
}

func googleBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

var googleSources = regexp.MustCompile(`url\(([^)]*)\)`)

func TestGoogleFontStylesheetIsFetchedOnceAndServedFromTheDataFolder(t *testing.T) {
	env, google := googleFontEnv(t)

	resp := env.get(t, "/api/v1/google-fonts/css?family=Noto+Sans+JP")
	css := googleBody(t, resp)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d: %s", resp.StatusCode, css)
	}
	for header, want := range map[string]string{
		"Content-Type":           "text/css; charset=utf-8",
		"Cache-Control":          "public, max-age=86400",
		"X-Content-Type-Options": "nosniff",
	} {
		if got := resp.Header.Get(header); got != want {
			t.Errorf("%s %q, want %q", header, got, want)
		}
	}
	etag := resp.Header.Get("ETag")
	if etag == "" {
		t.Fatal("no ETag")
	}
	var sources []string
	for _, m := range googleSources.FindAllStringSubmatch(css, -1) {
		sources = append(sources, m[1])
	}
	want := []string{
		"/api/v1/google-fonts/files/" + googleFontName("/s/notosansjp/a.woff2"),
		"/api/v1/google-fonts/files/" + googleFontName("/s/notosansjp/b.woff2"),
	}
	if strings.Join(sources, " ") != strings.Join(want, " ") {
		t.Fatalf("sources %q, want %q", sources, want)
	}

	// Kept: the next request is answered from the data folder.
	again := env.get(t, "/api/v1/google-fonts/css?family=Noto+Sans+JP")
	if body := googleBody(t, again); again.StatusCode != http.StatusOK || body != css {
		t.Fatalf("again: status %d, body %q", again.StatusCode, body)
	}
	if n := google.cssRequests.Load(); n != 1 {
		t.Fatalf("%d stylesheet requests to Google, want 1", n)
	}
	if n := google.fileRequests.Load(); n != 2 {
		t.Fatalf("%d file requests to Google, want 2", n)
	}
	unchanged := env.getWithHeaders(t, "/api/v1/google-fonts/css?family=Noto+Sans+JP", map[string]string{"If-None-Match": etag})
	googleBody(t, unchanged)
	if unchanged.StatusCode != http.StatusNotModified {
		t.Fatalf("If-None-Match: status %d, want 304", unchanged.StatusCode)
	}

	for i, source := range want {
		file := env.get(t, source)
		body := googleBody(t, file)
		if file.StatusCode != http.StatusOK {
			t.Fatalf("%s: status %d", source, file.StatusCode)
		}
		path := []string{"/s/notosansjp/a.woff2", "/s/notosansjp/b.woff2"}[i]
		if body != googleFontBytes(path) {
			t.Fatalf("%s: body %q", source, body)
		}
		for header, want := range map[string]string{
			"Content-Type":   "font/woff2",
			"Cache-Control":  "public, max-age=31536000, immutable",
			"Content-Length": strconv.Itoa(len(body)),
			"ETag":           `"` + strings.TrimSuffix(googleFontName(path), ".woff2") + `"`,
		} {
			if got := file.Header.Get(header); got != want {
				t.Errorf("%s: %s %q, want %q", source, header, got, want)
			}
		}
		cached := env.getWithHeaders(t, source, map[string]string{"If-None-Match": file.Header.Get("ETag")})
		googleBody(t, cached)
		if cached.StatusCode != http.StatusNotModified {
			t.Fatalf("%s If-None-Match: status %d, want 304", source, cached.StatusCode)
		}
	}
	if n := google.fileRequests.Load(); n != 2 {
		t.Fatalf("%d file requests to Google once the files were served, want 2", n)
	}
}

func TestGoogleFontStylesheetProblems(t *testing.T) {
	env, google := googleFontEnv(t)
	google.mu.Lock()
	google.foreign["Elsewhere"] = true
	google.mu.Unlock()

	cases := []struct {
		path string
		want int
	}{
		{"/api/v1/google-fonts/css?family=Nope", http.StatusNotFound},
		{"/api/v1/google-fonts/css?family=Elsewhere", http.StatusInternalServerError},
		{"/api/v1/google-fonts/css?family=" + "a%3Bb", http.StatusUnprocessableEntity},
		{"/api/v1/google-fonts/css?family=%27x", http.StatusUnprocessableEntity},
		{"/api/v1/google-fonts/css?family=+", http.StatusUnprocessableEntity},
		{"/api/v1/google-fonts/css?family=" + strings.Repeat("a", 101), http.StatusUnprocessableEntity},
		{"/api/v1/google-fonts/css", http.StatusUnprocessableEntity},
	}
	for _, c := range cases {
		resp := env.get(t, c.path)
		body := googleBody(t, resp)
		if resp.StatusCode != c.want {
			t.Errorf("%s: status %d, want %d: %s", c.path, resp.StatusCode, c.want, body)
			continue
		}
		if ct := resp.Header.Get("Content-Type"); ct != "application/problem+json" {
			t.Errorf("%s: content type %q, want problem details", c.path, ct)
		}
		if cc := resp.Header.Get("Cache-Control"); cc != "private, no-store" {
			t.Errorf("%s: Cache-Control %q, want private, no-store", c.path, cc)
		}
	}
	if n := google.fileRequests.Load(); n != 0 {
		t.Fatalf("a family refused for a source off the font host still fetched %d files", n)
	}
}

func TestGoogleFontStylesheetWhenGoogleIsDown(t *testing.T) {
	env := newTestEnv(t)
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	env.srv.RouteGoogleFontsForTest(closed.URL)

	resp := env.get(t, "/api/v1/google-fonts/css?family=Arapey")
	if body := googleBody(t, resp); resp.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503: %s", resp.StatusCode, body)
	}
}

func TestGoogleFontMissesAreRateLimited(t *testing.T) {
	env, google := googleFontEnv(t)

	// A family kept first, which is one miss.
	if resp := env.get(t, "/api/v1/google-fonts/css?family=Arapey"); resp.StatusCode != http.StatusOK {
		t.Fatalf("status %d", resp.StatusCode)
	} else {
		googleBody(t, resp)
	}
	for i := range 29 {
		resp := env.get(t, fmt.Sprintf("/api/v1/google-fonts/css?family=Nope+%d", i))
		if body := googleBody(t, resp); resp.StatusCode != http.StatusNotFound {
			t.Fatalf("miss %d: status %d: %s", i+2, resp.StatusCode, body)
		}
	}
	asked := google.cssRequests.Load()
	limited := env.get(t, "/api/v1/google-fonts/css?family=Roboto")
	googleBody(t, limited)
	if limited.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("the 31st miss: status %d, want 429", limited.StatusCode)
	}
	if limited.Header.Get("Retry-After") == "" {
		t.Fatal("a 429 without Retry-After")
	}
	if n := google.cssRequests.Load(); n != asked {
		t.Fatal("a limited miss still asked Google")
	}
	// A family kept is never counted, so it is served still.
	for range 3 {
		kept := env.get(t, "/api/v1/google-fonts/css?family=Arapey")
		if body := googleBody(t, kept); kept.StatusCode != http.StatusOK {
			t.Fatalf("a kept family past the limit: status %d: %s", kept.StatusCode, body)
		}
	}
}

func TestGoogleFontFilesAreOnlyKeptFiles(t *testing.T) {
	dataDir := t.TempDir()
	writeFile(t, dataDir, "database-copy.sqlite", []byte("secret"))
	writeFile(t, dataDir, "google-fonts/secret.woff2", []byte("secret"))
	writeFile(t, dataDir, "google-fonts/files/.tmp-123", []byte("secret"))
	writeFile(t, dataDir, "google-fonts/css/"+strings.Repeat("a", 64)+".css", []byte("secret"))
	env := newTestEnvWithDataDir(t, dataDir)

	for _, name := range []string{
		strings.Repeat("0", 64) + ".woff2",
		"..%2Fsecret.woff2",
		"..%2F..%2Fdatabase-copy.sqlite",
		"..%2Fcss%2F" + strings.Repeat("a", 64) + ".css",
		"%2E%2E",
		".tmp-123",
		"x.woff2",
		strings.Repeat("A", 64) + ".woff2",
	} {
		resp := env.get(t, "/api/v1/google-fonts/files/"+name)
		body := googleBody(t, resp)
		if resp.StatusCode == http.StatusOK || strings.Contains(body, "secret") {
			t.Errorf("%s: status %d, body %q", name, resp.StatusCode, body)
		}
	}
	resp := env.get(t, "/api/v1/google-fonts/files/"+strings.Repeat("0", 64)+".woff2")
	googleBody(t, resp)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("a file not kept: status %d, want 404", resp.StatusCode)
	}
}
