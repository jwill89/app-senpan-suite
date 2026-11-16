package media

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

const testFilesURL = "/api/v1/google-fonts/files/"

// googleStandIn stands in for the Google Fonts CSS API (/css2) and its font host
// (/s/). Each family's stylesheet names two files of its own, unless css says
// otherwise for it; a file's bytes are WOFF2's magic and its path.
type googleStandIn struct {
	srv *httptest.Server

	mu sync.Mutex
	// css answers a family's stylesheet in place of the usual one; status, a
	// family's status in place of 200.
	css    map[string]string
	status map[string]int
	// file answers a file's bytes in place of the usual ones, and fileStatus its
	// status in place of 200, by its path.
	file       map[string][]byte
	fileStatus map[string]int

	cssRequests  atomic.Int32
	fileRequests atomic.Int32
	lastQuery    atomic.Value
	lastAgent    atomic.Value
}

func newGoogleStandIn(t *testing.T) *googleStandIn {
	t.Helper()
	g := &googleStandIn{
		css: map[string]string{}, status: map[string]int{}, file: map[string][]byte{}, fileStatus: map[string]int{},
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /css2", func(w http.ResponseWriter, r *http.Request) {
		g.cssRequests.Add(1)
		g.lastQuery.Store(r.URL.RawQuery)
		g.lastAgent.Store(r.UserAgent())
		raw, _, _ := strings.Cut(strings.TrimPrefix(r.URL.RawQuery, "family="), ":")
		family := strings.ReplaceAll(raw, "+", " ")
		g.mu.Lock()
		css, custom := g.css[family]
		status := g.status[family]
		g.mu.Unlock()
		if status != 0 {
			w.WriteHeader(status)
			return
		}
		if !custom {
			slug := strings.ToLower(strings.ReplaceAll(family, " ", ""))
			css = fmt.Sprintf("/* latin */\n@font-face {\n  font-family: '%s';\n  src: url(%s/s/%s/a.woff2) format('woff2');\n}\n"+
				"/* latin-ext */\n@font-face {\n  font-family: '%s';\n  src: url(%[2]s/s/%[3]s/b.woff2) format('woff2');\n}\n",
				family, g.srv.URL, slug, family)
		}
		w.Header().Set("Content-Type", "text/css; charset=utf-8")
		_, _ = io.WriteString(w, css)
	})
	mux.HandleFunc("GET /s/", func(w http.ResponseWriter, r *http.Request) {
		g.fileRequests.Add(1)
		g.mu.Lock()
		data, custom := g.file[r.URL.Path]
		status := g.fileStatus[r.URL.Path]
		g.mu.Unlock()
		if status != 0 {
			w.WriteHeader(status)
			return
		}
		if !custom {
			data = []byte(woff2Magic + r.URL.Path)
		}
		w.Header().Set("Content-Type", "font/woff2")
		_, _ = w.Write(data)
	})
	g.srv = httptest.NewServer(mux)
	t.Cleanup(g.srv.Close)
	return g
}

func (g *googleStandIn) set(fn func()) {
	g.mu.Lock()
	defer g.mu.Unlock()
	fn()
}

// newGoogleFonts is a folder of kept families over a temporary data folder, fetching
// from stand.
func newGoogleFonts(t *testing.T, stand *googleStandIn) *GoogleFonts {
	t.Helper()
	g := NewGoogleFonts(t.TempDir(), testFilesURL)
	g.CSSURL = stand.srv.URL + "/css2"
	g.FileURL = stand.srv.URL + "/s/"
	return g
}

// keptCSS is a kept family's stylesheet, failing the test when it isn't kept.
func keptCSS(t *testing.T, g *GoogleFonts, family string) string {
	t.Helper()
	f, _, err := g.Stylesheet(family)
	if err != nil {
		t.Fatalf("stylesheet of %q: %v", family, err)
	}
	defer f.Close()
	b, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func isKept(g *GoogleFonts, family string) bool {
	f, _, err := g.Stylesheet(family)
	if err != nil {
		return false
	}
	_ = f.Close()
	return true
}

// keptFiles are the names in the files folder.
func keptFiles(t *testing.T, g *GoogleFonts) []string {
	t.Helper()
	entries, err := os.ReadDir(g.filesDir())
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	return names
}

func hashName(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]) + ".woff2"
}

var keptSource = regexp.MustCompile(`url\(([^)]*)\)`)

func TestGoogleFontFetchKeepsTheFamilyWithItsSourcesRewritten(t *testing.T) {
	stand := newGoogleStandIn(t)
	g := newGoogleFonts(t, stand)

	if err := g.Fetch(t.Context(), "Test Font"); err != nil {
		t.Fatal(err)
	}
	css := keptCSS(t, g, "Test Font")
	if strings.Contains(css, stand.srv.URL) {
		t.Fatalf("a source still names the font host:\n%s", css)
	}
	want := []string{
		testFilesURL + hashName([]byte(woff2Magic+"/s/testfont/a.woff2")),
		testFilesURL + hashName([]byte(woff2Magic+"/s/testfont/b.woff2")),
	}
	var got []string
	for _, m := range keptSource.FindAllStringSubmatch(css, -1) {
		got = append(got, m[1])
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Fatalf("sources %q, want %q", got, want)
	}
	if !strings.Contains(css, "font-family: 'Test Font'") || !strings.Contains(css, "format('woff2')") {
		t.Fatalf("the rest of the stylesheet changed:\n%s", css)
	}
	for _, source := range want {
		name := strings.TrimPrefix(source, testFilesURL)
		f, info, err := g.File(name)
		if err != nil {
			t.Fatalf("file %s: %v", name, err)
		}
		_ = f.Close()
		if info.Size() == 0 {
			t.Fatalf("file %s is empty", name)
		}
	}
	if q, _ := stand.lastQuery.Load().(string); q != "family=Test+Font:ital,wght@0,400;0,700;0,800;1,400&display=swap" {
		t.Fatalf("the CSS API was asked %q", q)
	}
	if ua, _ := stand.lastAgent.Load().(string); !strings.Contains(ua, "Chrome/") {
		t.Fatalf("the CSS API was asked as %q, which doesn't get WOFF2", ua)
	}
	if n := stand.cssRequests.Load(); n != 1 {
		t.Fatalf("%d stylesheet requests, want 1", n)
	}
	if n := stand.fileRequests.Load(); n != 2 {
		t.Fatalf("%d file requests, want 2", n)
	}
}

func TestGoogleFontKeptIsNotFetchedAgain(t *testing.T) {
	stand := newGoogleStandIn(t)
	g := newGoogleFonts(t, stand)

	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Go(func() { errs <- g.Fetch(t.Context(), "Twice") })
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := g.Fetch(t.Context(), "Twice"); err != nil {
		t.Fatal(err)
	}
	if n := stand.cssRequests.Load(); n != 1 {
		t.Fatalf("%d stylesheet requests for one family, want 1", n)
	}
	if n := stand.fileRequests.Load(); n != 2 {
		t.Fatalf("%d file requests for one family, want 2", n)
	}
}

func TestGoogleFontStylesheetsBreakingARuleAreRefused(t *testing.T) {
	stand := newGoogleStandIn(t)
	host := stand.srv.URL
	cases := []struct {
		name, family string
		arrange      func(g *GoogleFonts)
	}{
		{"a source off the font host", "Off Host", func(*GoogleFonts) {
			stand.css["Off Host"] = "@font-face { src: url(https://evil.example/s/x.woff2); }"
		}},
		{"a source outside the font host's folder", "Look Alike", func(*GoogleFonts) {
			stand.css["Look Alike"] = "@font-face { src: url(" + host + "/other/x.woff2); }"
		}},
		{"a source with credentials", "User Info", func(*GoogleFonts) {
			stand.css["User Info"] = "@font-face { src: url(http://me@" + strings.TrimPrefix(host, "http://") + "/s/x.woff2); }"
		}},
		{"an import", "Imports", func(*GoogleFonts) {
			stand.css["Imports"] = "@import url(" + host + "/s/more.css);"
		}},
		{"no sources", "Empty", func(*GoogleFonts) {
			stand.css["Empty"] = "/* nothing */"
		}},
		{"a stylesheet over its limit", "Long", func(g *GoogleFonts) {
			g.maxCSS = 64
			stand.css["Long"] = "@font-face { src: url(" + host + "/s/long.woff2); }" + strings.Repeat(" ", 64)
		}},
		{"a file over its limit", "Big", func(g *GoogleFonts) {
			g.maxFile = 32
			stand.file["/s/big/a.woff2"] = []byte(woff2Magic + strings.Repeat("x", 64))
			stand.css["Big"] = "@font-face { src: url(" + host + "/s/small.woff2); src: url(" + host + "/s/big/a.woff2); }"
		}},
		{"a family over its limit", "Heavy", func(g *GoogleFonts) {
			g.maxFamilyLen = 200
			stand.css["Heavy"] = "@font-face { src: url(" + host + "/s/heavy/a" + strings.Repeat("a", 100) + ".woff2); " +
				"src: url(" + host + "/s/heavy/b" + strings.Repeat("b", 100) + ".woff2); }"
		}},
		{"more files than its limit", "Many", func(g *GoogleFonts) {
			g.maxFiles = 1
			stand.css["Many"] = "@font-face { src: url(" + host + "/s/m/1.woff2); src: url(" + host + "/s/m/2.woff2); }"
		}},
		{"a file that isn't WOFF2", "Not Woff2", func(*GoogleFonts) {
			stand.file["/s/ttf/a.ttf"] = []byte("\x00\x01\x00\x00not a woff2")
			stand.css["Not Woff2"] = "@font-face { src: url(" + host + "/s/ok.woff2); src: url(" + host + "/s/ttf/a.ttf); }"
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			g := newGoogleFonts(t, stand)
			stand.set(func() { c.arrange(g) })
			if err := g.Fetch(t.Context(), c.family); !errors.Is(err, ErrGoogleRefused) {
				t.Fatalf("fetch: %v, want refused", err)
			}
			if isKept(g, c.family) {
				t.Fatal("a refused family was kept")
			}
			if files := keptFiles(t, g); len(files) != 0 {
				t.Fatalf("a refused family left files: %v", files)
			}
		})
	}
}

func TestGoogleFontUnknownFamily(t *testing.T) {
	stand := newGoogleStandIn(t)
	g := newGoogleFonts(t, stand)
	stand.set(func() { stand.status["Nope"] = http.StatusBadRequest; stand.status["Gone"] = http.StatusNotFound })
	for _, family := range []string{"Nope", "Gone"} {
		if err := g.Fetch(t.Context(), family); !errors.Is(err, ErrUnknownFamily) {
			t.Fatalf("%s: %v, want unknown", family, err)
		}
	}
}

func TestGoogleFontUnreachable(t *testing.T) {
	stand := newGoogleStandIn(t)
	g := newGoogleFonts(t, stand)
	stand.set(func() { stand.status["Down"] = http.StatusServiceUnavailable })
	if err := g.Fetch(t.Context(), "Down"); !errors.Is(err, ErrGoogleUnreachable) {
		t.Fatalf("an answer of 503: %v, want unreachable", err)
	}

	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()
	g.CSSURL = closed.URL + "/css2"
	if err := g.Fetch(t.Context(), "Anything"); !errors.Is(err, ErrGoogleUnreachable) {
		t.Fatalf("no answer: %v, want unreachable", err)
	}
}

func TestGoogleFontAFileThatFailsLeavesNothing(t *testing.T) {
	stand := newGoogleStandIn(t)
	g := newGoogleFonts(t, stand)
	stand.set(func() {
		stand.fileStatus["/s/half/b.woff2"] = http.StatusInternalServerError
		stand.css["Half"] = "@font-face { src: url(" + stand.srv.URL + "/s/half/a.woff2); src: url(" + stand.srv.URL + "/s/half/b.woff2); }"
	})
	if err := g.Fetch(t.Context(), "Half"); !errors.Is(err, ErrGoogleUnreachable) {
		t.Fatalf("fetch: %v, want unreachable", err)
	}
	if isKept(g, "Half") {
		t.Fatal("a family missing a file was kept")
	}
	if files := keptFiles(t, g); len(files) != 0 {
		t.Fatalf("a failed fetch left files: %v", files)
	}
}

func TestGoogleFontFolderKeepsItsLimitOfFamilies(t *testing.T) {
	stand := newGoogleStandIn(t)
	g := newGoogleFonts(t, stand)
	g.maxFamilies = 2

	for _, family := range []string{"Alpha", "Bravo"} {
		if err := g.Fetch(t.Context(), family); err != nil {
			t.Fatal(err)
		}
	}
	// Alpha served since, so Bravo is the least recently served.
	keptCSS(t, g, "Alpha")
	if err := g.Fetch(t.Context(), "Charlie"); err != nil {
		t.Fatal(err)
	}
	if !isKept(g, "Alpha") || !isKept(g, "Charlie") || isKept(g, "Bravo") {
		t.Fatalf("kept Alpha %v, Bravo %v, Charlie %v; want Alpha and Charlie",
			isKept(g, "Alpha"), isKept(g, "Bravo"), isKept(g, "Charlie"))
	}
	files := strings.Join(keptFiles(t, g), " ")
	for _, path := range []string{"/s/alpha/a.woff2", "/s/charlie/b.woff2"} {
		if !strings.Contains(files, hashName([]byte(woff2Magic+path))) {
			t.Fatalf("%s's file went with Bravo's", path)
		}
	}
	if strings.Contains(files, hashName([]byte(woff2Magic+"/s/bravo/a.woff2"))) {
		t.Fatal("Bravo's files outlived it")
	}
	if n := len(keptFiles(t, g)); n != 4 {
		t.Fatalf("%d files kept, want 4", n)
	}
}

func TestGoogleFontFolderKeepsItsLimitOfBytes(t *testing.T) {
	stand := newGoogleStandIn(t)
	g := newGoogleFonts(t, stand)
	if err := g.Fetch(t.Context(), "Alpha"); err != nil {
		t.Fatal(err)
	}
	kept, err := g.kept()
	if err != nil {
		t.Fatal(err)
	}
	// Room for one family and a little over, not two.
	g.maxTotal = bytesOf(kept, g.fileSizes()) + 50
	if err := g.Fetch(t.Context(), "Bravo"); err != nil {
		t.Fatal(err)
	}
	if isKept(g, "Alpha") || !isKept(g, "Bravo") {
		t.Fatal("the folder went over its bytes")
	}
}

func TestGoogleFontFileNamesAreChecked(t *testing.T) {
	stand := newGoogleStandIn(t)
	g := newGoogleFonts(t, stand)
	if err := g.Fetch(t.Context(), "Alpha"); err != nil {
		t.Fatal(err)
	}
	kept := keptFiles(t, g)[0]
	if f, _, err := g.File(kept); err != nil {
		t.Fatalf("a kept file: %v", err)
	} else {
		_ = f.Close()
	}
	// A secret beside the folder, which no name may reach.
	if err := os.WriteFile(filepath.Join(g.Dir(), "secret.woff2"), []byte(woff2Magic), 0o644); err != nil {
		t.Fatal(err)
	}
	key := familyKey("Alpha")
	for _, name := range []string{
		"", "../secret.woff2", "..", "../css/" + key + ".css", key + ".css", strings.ToUpper(kept),
		strings.Repeat("0", 64) + ".woff2", strings.TrimSuffix(kept, ".woff2"), kept + "/x", `..\secret.woff2`,
		"/" + kept,
	} {
		if f, _, err := g.File(name); !errors.Is(err, fs.ErrNotExist) {
			if f != nil {
				_ = f.Close()
			}
			t.Errorf("File(%q): %v, want not found", name, err)
		}
	}
}

func TestGoogleFontFetchWaitsItsTurnOnlyAsLongAsItsCaller(t *testing.T) {
	stand := newGoogleStandIn(t)
	g := newGoogleFonts(t, stand)
	g.fetching <- struct{}{} // a fetch that never ends
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if err := g.Fetch(ctx, "Alpha"); !errors.Is(err, context.Canceled) {
		t.Fatalf("fetch: %v, want canceled", err)
	}
}
