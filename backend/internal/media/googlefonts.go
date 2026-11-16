package media

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"maps"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// The Google Fonts families a page picks at run time, kept here so browsers never
// contact Google. The first request for a family fetches its stylesheet from the
// Google Fonts CSS API and every WOFF2 file it names from Google's font host, and keeps
// them under <dataDir>/google-fonts: the stylesheet in css/, named by the SHA-256 of
// the family's name, its every source rewritten to this server's file route; each
// file in files/, named by the SHA-256 of its bytes, so a name always serves the same
// bytes. The folder keeps a bounded number of families and bytes, the least recently
// served family going first, and may be emptied at any time: what is missing is
// fetched again.

const (
	googleCSSURL  = "https://fonts.googleapis.com/css2"
	googleFileURL = "https://fonts.gstatic.com/"
	// googleFontSpec is what each stylesheet asks for beside the family: the regular,
	// bold, extra bold and italic faces, shown in the fallback face until they load.
	googleFontSpec = ":ital,wght@0,400;0,700;0,800;1,400&display=swap"
	// googleUserAgent is a current Chrome's, which Google answers with WOFF2 files
	// split by unicode range.
	googleUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 " +
		"(KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"
	// googleRequestTimeout bounds each request to Google, its body included, and
	// googleFamilyTimeout the whole of one family's fetch.
	googleRequestTimeout = 20 * time.Second
	googleFamilyTimeout  = 90 * time.Second
	// googleDownloads is how many of a family's files download at once.
	googleDownloads = 6
	// woff2Magic is how every WOFF2 file begins.
	woff2Magic = "wOF2"
)

// The limits on what the folder keeps. A family in a CJK script runs to about 125
// files and 6 MB; a Latin one to a handful of files and well under 1 MB.
const (
	defaultGoogleFamilies    = 100
	defaultGoogleTotalBytes  = 256 << 20
	defaultGoogleCSSBytes    = 1 << 20
	defaultGoogleFileBytes   = 4 << 20
	defaultGoogleFamilyBytes = 24 << 20
	defaultGoogleFiles       = 300
)

var (
	// ErrUnknownFamily is a family Google doesn't have.
	ErrUnknownFamily = errors.New("google fonts has no such family")
	// ErrGoogleUnreachable is Google not answering, or answering with a failure.
	ErrGoogleUnreachable = errors.New("google fonts couldn't be reached")
	// ErrGoogleRefused is an answer from Google that breaks a rule of what is kept: a
	// source off the font host, a file that isn't WOFF2, or a size or count over its
	// limit. Nothing of the family is kept.
	ErrGoogleRefused = errors.New("google fonts sent what this server won't keep")
)

// googleFileName is the name of a kept file: the SHA-256 of its bytes, in hex.
var googleFileName = regexp.MustCompile(`^[0-9a-f]{64}\.woff2$`)

// keptFileRef finds the kept files a stylesheet names.
var keptFileRef = regexp.MustCompile(`[0-9a-f]{64}\.woff2`)

// cssSource is one url(...) of a stylesheet, its address unquoted, single-quoted or
// double-quoted.
var cssSource = regexp.MustCompile(`url\(\s*(?:'([^']*)'|"([^"]*)"|([^'"()\s]*))\s*\)`)

// GoogleFonts is the Google Fonts families kept under a data folder.
type GoogleFonts struct {
	root string
	// filesURL is the path every source of a kept stylesheet starts with, the file's
	// name following it.
	filesURL string

	// CSSURL is the Google Fonts CSS API, and FileURL the prefix every source of a
	// stylesheet from it must start with: Google's own when empty. A test points them
	// at a stand-in.
	CSSURL, FileURL string
	// HTTP makes the requests to Google; nil is a client with googleRequestTimeout
	// that follows no redirect.
	HTTP *http.Client

	maxFamilies, maxFiles                   int
	maxTotal, maxCSS, maxFile, maxFamilyLen int64

	// fetching is held by the one fetch that runs at a time, so a family asked for
	// twice at once is fetched once and the folder changes in one place.
	fetching chan struct{}

	mu sync.Mutex
	// used is when each family (by key) was last served since the server started;
	// one not served since is as old as its stylesheet.
	used map[string]time.Time
}

// NewGoogleFonts is the Google Fonts families kept under dataDir, whose stylesheets
// name their files at filesURL followed by the file's name.
func NewGoogleFonts(dataDir, filesURL string) *GoogleFonts {
	return &GoogleFonts{
		root:         dataDir,
		filesURL:     filesURL,
		maxFamilies:  defaultGoogleFamilies,
		maxFiles:     defaultGoogleFiles,
		maxTotal:     defaultGoogleTotalBytes,
		maxCSS:       defaultGoogleCSSBytes,
		maxFile:      defaultGoogleFileBytes,
		maxFamilyLen: defaultGoogleFamilyBytes,
		fetching:     make(chan struct{}, 1),
		used:         map[string]time.Time{},
	}
}

// Dir is the folder the families are kept in.
func (g *GoogleFonts) Dir() string { return filepath.Join(g.root, "google-fonts") }

func (g *GoogleFonts) cssDir() string { return filepath.Join(g.Dir(), "css") }

func (g *GoogleFonts) filesDir() string { return filepath.Join(g.Dir(), "files") }

// familyKey is the name a family's stylesheet is kept under, whatever its characters.
func familyKey(family string) string {
	sum := sha256.Sum256([]byte(family))
	return hex.EncodeToString(sum[:])
}

// Stylesheet opens a kept family's stylesheet, fs.ErrNotExist when it isn't kept.
func (g *GoogleFonts) Stylesheet(family string) (*os.File, fs.FileInfo, error) {
	key := familyKey(family)
	f, info, err := openRegular(g.cssDir(), key+".css")
	if err != nil {
		return nil, nil, err
	}
	g.mu.Lock()
	g.used[key] = time.Now()
	g.mu.Unlock()
	return f, info, nil
}

// File opens a kept font file by its name, fs.ErrNotExist when the name can't be a
// kept file's or the file isn't kept.
func (g *GoogleFonts) File(name string) (*os.File, fs.FileInfo, error) {
	if !googleFileName.MatchString(name) {
		return nil, nil, fs.ErrNotExist
	}
	return openRegular(g.filesDir(), name)
}

// openRegular opens a regular file inside dir; the root keeps the open inside it.
func openRegular(dir, name string) (*os.File, fs.FileInfo, error) {
	f, err := os.OpenInRoot(dir, name)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil, fs.ErrNotExist
		}
		return nil, nil, openReason(err)
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		_ = f.Close() // read only
		return nil, nil, fs.ErrNotExist
	}
	return f, info, nil
}

// Fetch keeps family, unless it is kept already: its stylesheet from Google, each file
// the stylesheet names, and the stylesheet rewritten to name the files here. One fetch
// runs at a time, and a caller waits its turn until ctx ends; a fetch once begun runs
// to the end when its caller leaves, so the next request finds the family kept.
func (g *GoogleFonts) Fetch(ctx context.Context, family string) error {
	select {
	case g.fetching <- struct{}{}:
	case <-ctx.Done():
		return ctx.Err()
	}
	defer func() { <-g.fetching }()

	key := familyKey(family)
	if _, err := os.Stat(filepath.Join(g.cssDir(), key+".css")); err == nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), googleFamilyTimeout)
	defer cancel()

	css, err := g.stylesheet(ctx, family)
	if err != nil {
		return err
	}
	sources, err := g.sources(css)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(g.filesDir(), 0o750); err != nil {
		return err
	}
	names, created, err := g.download(ctx, sources, int64(len(css)))
	if err != nil {
		g.remove(created)
		return err
	}
	kept := cssSource.ReplaceAllStringFunc(string(css), func(match string) string {
		return "url(" + g.filesURL + names[sourceOf(match)] + ")"
	})
	if err := g.keep(key, []byte(kept), slices.Collect(maps.Values(names))); err != nil {
		g.remove(created)
		return err
	}
	return nil
}

// stylesheet is family's stylesheet from the Google Fonts CSS API.
func (g *GoogleFonts) stylesheet(ctx context.Context, family string) ([]byte, error) {
	base := g.CSSURL
	if base == "" {
		base = googleCSSURL
	}
	resp, err := g.get(ctx, base+"?family="+url.QueryEscape(family)+googleFontSpec)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrGoogleUnreachable, err)
	}
	defer func() { _ = resp.Body.Close() }() // read in full below, or given up on
	switch {
	case resp.StatusCode == http.StatusBadRequest || resp.StatusCode == http.StatusNotFound:
		return nil, ErrUnknownFamily
	case resp.StatusCode != http.StatusOK:
		return nil, fmt.Errorf("%w: the CSS API answered %d", ErrGoogleUnreachable, resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/css") {
		return nil, fmt.Errorf("%w: the CSS API answered %q", ErrGoogleRefused, ct)
	}
	css, err := io.ReadAll(io.LimitReader(resp.Body, g.maxCSS+1))
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrGoogleUnreachable, err)
	}
	if int64(len(css)) > g.maxCSS {
		return nil, fmt.Errorf("%w: a stylesheet over %d bytes", ErrGoogleRefused, g.maxCSS)
	}
	return css, nil
}

// sources are the distinct addresses css names, each checked to be a file of the font
// host. A stylesheet that imports another, names an address that isn't on the font
// host, or names none, is refused.
func (g *GoogleFonts) sources(css []byte) ([]string, error) {
	if bytes.Contains(bytes.ToLower(css), []byte("@import")) {
		return nil, fmt.Errorf("%w: a stylesheet that imports another", ErrGoogleRefused)
	}
	matches := cssSource.FindAllSubmatch(css, -1)
	if len(matches) == 0 || len(matches) != bytes.Count(bytes.ToLower(css), []byte("url(")) {
		return nil, fmt.Errorf("%w: a stylesheet whose sources can't be read", ErrGoogleRefused)
	}
	host := g.FileURL
	if host == "" {
		host = googleFileURL
	}
	var sources []string
	for _, m := range matches {
		source := sourceOf(string(m[0]))
		if !onHost(source, host) {
			return nil, fmt.Errorf("%w: a source off the font host", ErrGoogleRefused)
		}
		if !slices.Contains(sources, source) {
			sources = append(sources, source)
		}
	}
	if len(sources) > g.maxFiles {
		return nil, fmt.Errorf("%w: %d files, over %d", ErrGoogleRefused, len(sources), g.maxFiles)
	}
	return sources, nil
}

// sourceOf is the address one url(...) names.
func sourceOf(match string) string {
	m := cssSource.FindStringSubmatch(match)
	if m == nil {
		return ""
	}
	return m[1] + m[2] + m[3]
}

// onHost reports whether source is an address under host (a scheme, a host and a path
// ending in a slash), with nothing that could lead the request elsewhere.
func onHost(source, host string) bool {
	h, err := url.Parse(host)
	if err != nil {
		return false
	}
	u, err := url.Parse(source)
	if err != nil || u.User != nil || u.Opaque != "" || strings.Contains(source, `\`) {
		return false
	}
	return u.Scheme == h.Scheme && u.Host == h.Host && strings.HasPrefix(u.EscapedPath(), h.EscapedPath()) &&
		!strings.Contains(u.Path, "/../")
}

// download fetches each source into the files folder, a few at a time, and answers
// each source's kept name and the names this fetch added to the folder. The first
// failure ends the rest. cssLen counts toward the family's byte limit.
func (g *GoogleFonts) download(ctx context.Context, sources []string, cssLen int64) (map[string]string, []string, error) {
	ctx, cancel := context.WithCancelCause(ctx)
	defer cancel(nil)
	var (
		mu      sync.Mutex
		names   = make(map[string]string, len(sources))
		created []string
		total   atomic.Int64
		wg      sync.WaitGroup
	)
	total.Store(cssLen)
	queue := make(chan string)
	for range min(googleDownloads, len(sources)) {
		wg.Go(func() {
			for source := range queue {
				name, added, err := g.downloadOne(ctx, source, &total)
				mu.Lock()
				if added {
					created = append(created, name)
				}
				names[source] = name
				mu.Unlock()
				if err != nil {
					cancel(err)
				}
			}
		})
	}
send:
	for _, source := range sources {
		select {
		case queue <- source:
		case <-ctx.Done():
			break send
		}
	}
	close(queue)
	wg.Wait()
	if err := context.Cause(ctx); err != nil {
		if !errors.Is(err, ErrGoogleRefused) && !errors.Is(err, ErrGoogleUnreachable) {
			err = fmt.Errorf("%w: %w", ErrGoogleUnreachable, err)
		}
		return nil, created, err
	}
	return names, created, nil
}

// downloadOne fetches one file into the files folder under its name, and reports
// whether the folder didn't hold it before. total counts the family's bytes so far.
func (g *GoogleFonts) downloadOne(ctx context.Context, source string, total *atomic.Int64) (string, bool, error) {
	resp, err := g.get(ctx, source)
	if err != nil {
		return "", false, fmt.Errorf("%w: %w", ErrGoogleUnreachable, err)
	}
	defer func() { _ = resp.Body.Close() }() // read in full below, or given up on
	if resp.StatusCode != http.StatusOK {
		return "", false, fmt.Errorf("%w: the font host answered %d", ErrGoogleUnreachable, resp.StatusCode)
	}
	if resp.ContentLength > g.maxFile {
		return "", false, fmt.Errorf("%w: a file over %d bytes", ErrGoogleRefused, g.maxFile)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, g.maxFile+1))
	if err != nil {
		return "", false, fmt.Errorf("%w: %w", ErrGoogleUnreachable, err)
	}
	switch {
	case int64(len(data)) > g.maxFile:
		return "", false, fmt.Errorf("%w: a file over %d bytes", ErrGoogleRefused, g.maxFile)
	case !bytes.HasPrefix(data, []byte(woff2Magic)):
		return "", false, fmt.Errorf("%w: a file that isn't WOFF2", ErrGoogleRefused)
	case total.Add(int64(len(data))) > g.maxFamilyLen:
		return "", false, fmt.Errorf("%w: a family over %d bytes", ErrGoogleRefused, g.maxFamilyLen)
	}
	sum := sha256.Sum256(data)
	name := hex.EncodeToString(sum[:]) + ".woff2"
	added, err := writeNew(g.filesDir(), name, data)
	return name, added, err
}

func (g *GoogleFonts) get(ctx context.Context, address string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", googleUserAgent)
	client := g.HTTP
	if client == nil {
		client = &http.Client{
			Timeout:       googleRequestTimeout,
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		}
	}
	return client.Do(req) //nolint:gosec // the CSS API, or a source checked to be on the font host
}

// writeNew writes data to dir/name through a temporary file, unless the file is there
// already, and reports whether it wrote it.
func writeNew(dir, name string, data []byte) (bool, error) {
	dst := filepath.Join(dir, name)
	if _, err := os.Stat(dst); err == nil {
		return false, nil
	}
	if err := writeAtomically(dir, dst, data); err != nil {
		return false, err
	}
	return true, nil
}

// writeAtomically writes data to dst, a path in dir, through a temporary file in dir
// that takes its name only once it is whole. A temporary file is a dotfile, which no
// route serves.
func writeAtomically(dir, dst string, data []byte) error {
	tmp, err := os.CreateTemp(dir, ".tmp-*")
	if err != nil {
		return err
	}
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		_ = os.Remove(tmp.Name()) // the write failed already
		return err
	}
	if err := os.Rename(tmp.Name(), dst); err != nil {
		_ = os.Remove(tmp.Name()) // the rename failed already
		return err
	}
	return nil
}

// remove deletes the named files from the files folder.
func (g *GoogleFonts) remove(names []string) {
	for _, name := range names {
		if err := os.Remove(filepath.Join(g.filesDir(), name)); err != nil && !errors.Is(err, fs.ErrNotExist) {
			slog.Warn("remove a google font file", "error", openReason(err))
		}
	}
}

// keptFamily is a family the folder holds.
type keptFamily struct {
	key   string
	size  int64
	files []string
	used  time.Time
}

// keep stores a family's rewritten stylesheet, naming files (already in the files
// folder), after making room for it: the least recently served families go until the
// folder holds fewer than its limit and its bytes leave room for this one's. Files no
// family names any more go with them.
func (g *GoogleFonts) keep(key string, css []byte, files []string) error {
	if err := os.MkdirAll(g.cssDir(), 0o750); err != nil {
		return err
	}
	kept, err := g.kept()
	if err != nil {
		return err
	}
	sizes := g.fileSizes()
	fresh := keptFamily{key: key, size: int64(len(css)), files: files}
	evicted := false
	for len(kept) > 0 && (len(kept)+1 > g.maxFamilies || bytesOf(append(slices.Clone(kept), fresh), sizes) > g.maxTotal) {
		oldest := slices.MinFunc(kept, func(a, b keptFamily) int { return a.used.Compare(b.used) })
		kept = slices.DeleteFunc(kept, func(f keptFamily) bool { return f.key == oldest.key })
		if err := os.Remove(filepath.Join(g.cssDir(), oldest.key+".css")); err != nil && !errors.Is(err, fs.ErrNotExist) {
			slog.Warn("remove a google font stylesheet", "error", openReason(err))
		}
		g.mu.Lock()
		delete(g.used, oldest.key)
		g.mu.Unlock()
		evicted = true
	}
	if err := writeAtomically(g.cssDir(), filepath.Join(g.cssDir(), key+".css"), css); err != nil {
		return err
	}
	g.mu.Lock()
	g.used[key] = time.Now()
	g.mu.Unlock()
	if evicted {
		g.sweep(append(kept, fresh))
	}
	return nil
}

// kept lists the families the folder holds, sweeping away what an interrupted write
// left behind.
func (g *GoogleFonts) kept() ([]keptFamily, error) {
	entries, err := os.ReadDir(g.cssDir())
	if err != nil {
		return nil, err
	}
	g.mu.Lock()
	used := maps.Clone(g.used)
	g.mu.Unlock()
	var kept []keptFamily
	for _, e := range entries {
		name := e.Name()
		if strings.HasPrefix(name, ".tmp-") {
			_ = os.Remove(filepath.Join(g.cssDir(), name)) // a write that never finished
			continue
		}
		key, ok := strings.CutSuffix(name, ".css")
		if !ok || !e.Type().IsRegular() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		css, err := os.ReadFile(filepath.Join(g.cssDir(), name)) //nolint:gosec // a name from the folder's own listing
		if err != nil {
			continue
		}
		last, ok := used[key]
		if !ok {
			last = info.ModTime()
		}
		kept = append(kept, keptFamily{key: key, size: info.Size(), files: keptFileRef.FindAllString(string(css), -1), used: last})
	}
	return kept, nil
}

// fileSizes are the sizes of the files the folder holds, by name.
func (g *GoogleFonts) fileSizes() map[string]int64 {
	sizes := map[string]int64{}
	entries, _ := os.ReadDir(g.filesDir()) // none read is none counted
	for _, e := range entries {
		if info, err := e.Info(); err == nil && googleFileName.MatchString(e.Name()) {
			sizes[e.Name()] = info.Size()
		}
	}
	return sizes
}

// bytesOf is what families take on disk: their stylesheets, and each file one of them
// names, once.
func bytesOf(families []keptFamily, sizes map[string]int64) int64 {
	var total int64
	seen := map[string]bool{}
	for _, f := range families {
		total += f.size
		for _, name := range f.files {
			if !seen[name] {
				seen[name] = true
				total += sizes[name]
			}
		}
	}
	return total
}

// sweep deletes every file of the files folder no family of families names, and what
// an interrupted write left there.
func (g *GoogleFonts) sweep(families []keptFamily) {
	named := map[string]bool{}
	for _, f := range families {
		for _, name := range f.files {
			named[name] = true
		}
	}
	entries, _ := os.ReadDir(g.filesDir()) // nothing read is nothing swept
	var orphans []string
	for _, e := range entries {
		if name := e.Name(); !named[name] && (googleFileName.MatchString(name) || strings.HasPrefix(name, ".tmp-")) {
			orphans = append(orphans, name)
		}
	}
	g.remove(orphans)
}
