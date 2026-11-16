// Package web serves the SPA the binary embeds: frontend/'s Vite build, which
// `npm run build` writes into dist/ here (vite.config.ts's outDir). Go embeds only
// files inside its own module, so the front end builds before any Go build, test or
// lint; without it, compiling this package fails on "pattern all:dist: no matching
// files found".
//
// The headers and cache rules the page and its files get are set here, the
// Content-Security-Policy among them. The data folder's public files (internal/media)
// are served beside this handler, which never answers for their paths.
package web

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"embed"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

//go:embed all:dist
var dist embed.FS

// Dist is the embedded build, rooted at its index.html.
func Dist() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err) // "dist" is a valid path, so fs.Sub cannot fail
	}
	return sub
}

// dataFolders are the paths of the data folder's own folders. The server serves the
// image library, the Carrd sites, the plugin repository and the sound effects from
// them, and the fonts never (the API streams those), so a path in one that reaches
// this handler names a file, never one of the app's pages.
var dataFolders = []string{"/images/", "/fonts/", "/carrd/", "/plugin/", "/sounds/"}

// noncePlaceholder is what the build writes into index.html wherever the page's nonce
// goes: on the tags Vite adds and in the csp-nonce meta the app reads it from. Each
// answer for the page puts a nonce of its own in its place.
const noncePlaceholder = "__CSP_NONCE__"

// contentSecurityPolicy is the policy every answer carries. The page's own answer
// passes its nonce, which admits the <style> and <script> elements the app adds at run
// time and the beacon Cloudflare writes into the page; the build's other files pass
// none. Beyond the site itself it names:
//
//   - challenges.cloudflare.com: Turnstile's script and its frame;
//   - static.cloudflareinsights.com and cloudflareinsights.com: Cloudflare Web
//     Analytics' beacon and where it reports;
//   - s4.anilist.co: the reading lists' AniList covers.
//
// Fonts are the site's own: the Google Fonts families the pages pick are served by
// the API from the data folder.
//
// data: images are the card export's capture, the plaque designer's pictures and the
// emoji picker's icons. The live event streams are same-origin, so no socket scheme
// is allowed.
//
// ancestors is who may show the page in a frame: 'none' for every page but an embed's.
func contentSecurityPolicy(nonce, ancestors string) string {
	own := "'self'"
	if nonce != "" {
		own += " 'nonce-" + nonce + "'"
	}
	return "default-src 'self'" +
		"; script-src " + own + " https://challenges.cloudflare.com https://static.cloudflareinsights.com" +
		"; style-src " + own +
		"; font-src 'self'" +
		"; img-src 'self' data: https://s4.anilist.co" +
		"; connect-src 'self' https://cloudflareinsights.com" +
		"; frame-src https://challenges.cloudflare.com" +
		"; frame-ancestors " + ancestors + "; base-uri 'self'; form-action 'self'; object-src 'none'"
}

// embedPrefix is where the pages made to be shown in another site's frame live.
const embedPrefix = "/embed/"

// ancestorSource is an origin a policy may name as a frame ancestor: a scheme and a
// host, a *. first label allowed, and an optional port, with nothing that could end
// the directive.
var ancestorSource = regexp.MustCompile(`^https?://(\*\.)?[a-z0-9-]+(\.[a-z0-9-]+)*(:[0-9]{1,5})?$`)

// frameAncestors is the policy's frame-ancestors for clean: 'none', unless it is an
// embed's page, which this site and the sites the allowlist names may frame.
func (h *Handler) frameAncestors(r *http.Request, clean string) string {
	if !strings.HasPrefix(clean, embedPrefix) || h.EmbedAncestors == nil {
		return "'none'"
	}
	sources := []string{"'self'"}
	for _, origin := range h.EmbedAncestors(r.Context()) {
		if ancestorSource.MatchString(origin) {
			sources = append(sources, origin)
		}
	}
	return strings.Join(sources, " ")
}

// newNonce is a fresh 128-bit nonce, base64-encoded.
func newNonce() string {
	var b [16]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never returns an error: it crashes the program instead
	return base64.StdEncoding.EncodeToString(b[:])
}

// contentTypes names the build's types Go's own table doesn't (it knows .css, .html,
// .js and .webp), since every response goes out with nosniff.
var contentTypes = map[string]string{
	".mp3":         "audio/mpeg",
	".ttf":         "font/ttf",
	".webmanifest": "application/manifest+json",
	".woff":        "font/woff",
	".woff2":       "font/woff2",
}

const (
	// The page, and the service worker with its registration: never cached, so a
	// deploy reaches every tab at once, and the worker can always update itself.
	noStore = "no-cache, no-store, must-revalidate"
	// Vite's content-hashed bundles, and Workbox's: a new build names them anew.
	immutable = "public, max-age=31536000, immutable"
	// Anything else: kept, but checked against its ETag before each use.
	revalidate = "no-cache"
)

type file struct {
	body        []byte
	etag        string
	contentType string
	cache       string
}

// Handler serves the build: its files, and the app's page for any other path, so the
// SPA's router can take it.
type Handler struct {
	files map[string]file
	// ReportOnly sends the policy as Content-Security-Policy-Report-Only, so the
	// browser reports what the policy would block and blocks nothing.
	ReportOnly bool
	// EmbedAncestors reads the sites whose pages may show an embed's page in a frame;
	// nil, or an empty list, lets only this site frame it.
	EmbedAncestors func(context.Context) []string
}

// New reads the build into memory, as the binary holds it anyway, with an ETag for
// each file but the page. It fails when the build has no index.html.
func New(build fs.FS) (*Handler, error) {
	files := map[string]file{}
	err := fs.WalkDir(build, ".", func(name string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		body, err := fs.ReadFile(build, name)
		if err != nil {
			return err
		}
		sum := sha256.Sum256(body)
		files[name] = file{
			body:        body,
			etag:        `"` + hex.EncodeToString(sum[:8]) + `"`,
			contentType: contentType(name),
			cache:       cachePolicy(name),
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("read the embedded front end: %w", err)
	}
	if _, ok := files["index.html"]; !ok {
		return nil, errors.New("the embedded front end has no index.html: build it (task build:web) before the Go build")
	}
	return &Handler{files: files}, nil
}

func contentType(name string) string {
	ext := path.Ext(name)
	if t, ok := contentTypes[ext]; ok {
		return t
	}
	if t := mime.TypeByExtension(ext); t != "" {
		return t
	}
	return "application/octet-stream"
}

func cachePolicy(name string) string {
	switch {
	case name == "index.html" || name == "sw.js" || name == "registerSW.js":
		return noStore
	case strings.HasPrefix(name, "assets/") || strings.HasPrefix(name, "workbox-"):
		return immutable
	default:
		return revalidate
	}
}

// ServeHTTP answers GET and HEAD: the app's page for /index.html and for any path that
// names no file, a file of the build, and a 404 for a file it lacks (a path with an
// extension, or one in an API or data folder).
func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	setSecurityHeaders(w.Header())
	clean := path.Clean("/" + r.URL.Path)
	name := strings.TrimPrefix(clean, "/")
	f, isFile := h.files[name]
	if name == "index.html" || (!isFile && !missingFile(clean)) {
		h.servePage(w, r, clean)
		return
	}
	h.setPolicy(w.Header(), "", "'none'")
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w)
		return
	}
	if !isFile {
		http.NotFound(w, r)
		return
	}
	serve(w, r, name, f)
}

// missingFile reports whether a path not in the build names a file rather than one of
// the app's own pages: it has an extension, or it is in the API's or a data folder.
func missingFile(clean string) bool {
	if clean == "/api" || strings.HasPrefix(clean, "/api/") || path.Ext(clean) != "" {
		return true
	}
	for _, folder := range dataFolders {
		if strings.HasPrefix(clean+"/", folder) {
			return true
		}
	}
	return false
}

// servePage answers with the app's page, its nonce placeholders filled with a fresh
// nonce that the policy names. The body differs on every answer, so it carries no
// ETag and is never answered 304: a copy kept from an earlier answer holds a nonce
// this answer's policy doesn't name.
//
// An embed's page may be framed by the sites its allowlist names, so it says so in
// frame-ancestors and sends no X-Frame-Options, which can only refuse every framer.
func (h *Handler) servePage(w http.ResponseWriter, r *http.Request, clean string) {
	nonce := newNonce()
	header := w.Header()
	h.setPolicy(header, nonce, h.frameAncestors(r, clean))
	if strings.HasPrefix(clean, embedPrefix) {
		header.Del("X-Frame-Options")
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		methodNotAllowed(w)
		return
	}
	page := h.files["index.html"]
	body := bytes.ReplaceAll(page.body, []byte(noncePlaceholder), []byte(nonce))
	header.Set("Content-Type", page.contentType)
	header.Set("Cache-Control", page.cache)
	header.Set("Content-Length", strconv.Itoa(len(body)))
	w.WriteHeader(http.StatusOK)
	if r.Method != http.MethodHead {
		_, _ = w.Write(body) // a failed write means the client went away
	}
}

func methodNotAllowed(w http.ResponseWriter) {
	w.Header().Set("Allow", "GET, HEAD")
	http.Error(w, http.StatusText(http.StatusMethodNotAllowed), http.StatusMethodNotAllowed)
}

func serve(w http.ResponseWriter, r *http.Request, name string, f file) {
	header := w.Header()
	header.Set("Content-Type", f.contentType)
	header.Set("Cache-Control", f.cache)
	header.Set("ETag", f.etag)
	// The zero time leaves out Last-Modified, which an embedded file doesn't have;
	// the ETag answers a revalidation with 304.
	http.ServeContent(w, r, name, time.Time{}, bytes.NewReader(f.body))
}

// setPolicy sets the Content-Security-Policy, or its report-only form, naming nonce
// when there is one and ancestors as who may frame the answer.
func (h *Handler) setPolicy(header http.Header, nonce, ancestors string) {
	name := "Content-Security-Policy"
	if h.ReportOnly {
		name = "Content-Security-Policy-Report-Only"
	}
	header.Set(name, contentSecurityPolicy(nonce, ancestors))
}

// setSecurityHeaders sets the headers beside the policy that the page and every file
// of the build get. HSTS is left to the site's vhost.
func setSecurityHeaders(h http.Header) {
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("X-Frame-Options", "DENY")
	h.Set("Referrer-Policy", "same-origin")
	h.Set("Permissions-Policy", "geolocation=(), camera=(), microphone=()")
}
