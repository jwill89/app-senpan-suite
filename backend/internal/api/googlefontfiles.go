package api

import (
	"context"
	"database/sql"
	"errors"
	"io/fs"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/danielgtaylor/huma/v2"
	"github.com/danielgtaylor/huma/v2/adapters/humago"

	"app-suite/internal/fonts"
	"app-suite/internal/media"
	"app-suite/internal/settings"
)

// The Google Fonts families pages pick at run time (the heading font, the plaque
// designer's fonts), served from this server: the first request for a family fetches
// its stylesheet and files from Google and keeps them in the data folder
// (media.GoogleFonts), and every request after is answered from there, so a browser
// never contacts Google.

const (
	// googleCSSCache keeps a family's stylesheet a day: a family evicted and fetched
	// again names the same files, since each is named for its bytes.
	googleCSSCache = "public, max-age=86400"
	// googleFileCache keeps a file for good: its name is the hash of its bytes.
	googleFileCache = "public, max-age=31536000, immutable"
)

type googleFontCSSInput struct {
	Family string `query:"family" required:"true" minLength:"1" maxLength:"100" doc:"The family as Google Fonts names it (\"Noto Sans JP\"). It may not hold quotes, backslashes, control characters, or any of { } ; < >."`
}

type googleFontFileInput struct {
	Name string `path:"name" maxLength:"100" doc:"A file's name, as a family's stylesheet gives it: the SHA-256 of its bytes in hex, then .woff2."`
}

func (h handlers) registerGoogleFontFiles(api huma.API) {
	huma.Register(api, huma.Operation{
		OperationID: "get-google-font-css", Method: http.MethodGet, Path: "/google-fonts/css",
		Summary: "A Google Fonts family's stylesheet",
		Description: "@font-face rules for the family's regular, bold, extra bold and italic faces, with every source " +
			"a file of this server's (google-fonts/files/<name>). A family the server doesn't keep yet is fetched from " +
			"Google once and kept; those requests are limited to 30 from one address in ten minutes (429, with " +
			"Retry-After), and a family not on the Google Fonts list the server's key reads is a 404. Cached publicly " +
			"for a day; If-None-Match and If-Modified-Since are answered.",
		Tags: fontTags,
		Errors: []int{
			http.StatusNotFound, http.StatusUnprocessableEntity, http.StatusTooManyRequests,
			http.StatusInternalServerError, http.StatusServiceUnavailable,
		},
		Responses: map[string]*huma.Response{
			"200": {
				Description: "The stylesheet.",
				Content:     map[string]*huma.MediaType{"text/css": {Schema: &huma.Schema{Type: "string"}}},
			},
			"304": {Description: "The stylesheet is unchanged since the If-None-Match tag or If-Modified-Since date."},
		},
	}, h.getGoogleFontCSS)
	huma.Register(api, huma.Operation{
		OperationID: "get-google-font-file", Method: http.MethodGet, Path: "/google-fonts/files/{name}",
		Summary: "A Google Fonts file",
		Description: "A WOFF2 file a kept family's stylesheet names. Its name is the hash of its bytes, so it is " +
			"cached publicly for a year as immutable. A name that isn't one, or a file no longer kept, is a 404.",
		Tags: fontTags, Errors: []int{http.StatusNotFound},
		Responses: map[string]*huma.Response{
			"200": {
				Description: "The font.",
				Content:     map[string]*huma.MediaType{"font/woff2": {Schema: &huma.Schema{Type: "string", Format: "binary"}}},
			},
			"304": {Description: "The file is unchanged since the If-None-Match tag."},
		},
	}, h.getGoogleFontFile)
}

func (h handlers) getGoogleFontCSS(ctx context.Context, in *googleFontCSSInput) (*fontStreamOutput, error) {
	family := strings.TrimSpace(in.Family)
	if family == "" || fonts.NameUnsafe(family) {
		return nil, huma.Error422UnprocessableEntity("That isn't a font family's name.", &huma.ErrorDetail{
			Location: "query.family", Value: in.Family,
			Message: "A font name may not be blank, or hold quotes, backslashes, control characters, or any of { } ; < >.",
		})
	}
	f, info, err := h.GoogleFonts.Stylesheet(family)
	if errors.Is(err, fs.ErrNotExist) {
		if err := h.fetchGoogleFont(ctx, family); err != nil {
			return nil, err
		}
		f, info, err = h.GoogleFonts.Stylesheet(family)
	}
	if err != nil {
		return nil, h.fail("open a google font stylesheet", err)
	}
	return &fontStreamOutput{Body: func(c huma.Context) {
		defer func() { _ = f.Close() }() // read only
		r, w := humago.Unwrap(c)
		hdr := w.Header()
		hdr.Set("Content-Type", "text/css; charset=utf-8")
		hdr.Set("Cache-Control", googleCSSCache)
		hdr.Set("ETag", fileTag(info))
		http.ServeContent(w, r, "", info.ModTime(), f)
	}}, nil
}

// fetchGoogleFont has the server keep a family it doesn't yet: counted against the
// caller's address first, then checked against the Google Fonts list when the server
// has one, then fetched.
func (h handlers) fetchGoogleFont(ctx context.Context, family string) error {
	ip := requestOf(ctx).IP
	if wait := retryAfter(h.GoogleFontLimit, ip); wait > 0 {
		return tooMany(wait, "Too many new fonts were asked for. Try again in a few minutes.")
	}
	if h.GoogleFontLimit != nil {
		h.GoogleFontLimit.Fail(ip)
	}
	listed, err := h.googleFamilyListed(ctx, family)
	if err != nil {
		return h.fail("read the google fonts key", err)
	}
	if !listed {
		return huma.Error404NotFound("Google Fonts has no family by that name.")
	}
	err = h.GoogleFonts.Fetch(ctx, family)
	switch {
	case err == nil:
		return nil
	case errors.Is(err, media.ErrUnknownFamily):
		return huma.Error404NotFound("Google Fonts has no family by that name.")
	case errors.Is(err, media.ErrGoogleUnreachable):
		h.Logger.Warn("fetch a google font", "family", family, "error", err)
		return huma.Error503ServiceUnavailable("Google Fonts couldn't be reached. Try again in a little while.")
	case errors.Is(err, media.ErrGoogleRefused):
		h.Logger.Warn("keep a google font", "family", family, "error", err)
		return huma.Error500InternalServerError("Google Fonts sent this family in a form this server won't serve.")
	default:
		return h.fail("keep a google font", err)
	}
}

// googleFamilyListed reports whether family may be fetched: it is on the Google Fonts
// list the server's key reads, or the server has no list to check it against.
func (h handlers) googleFamilyListed(ctx context.Context, family string) (bool, error) {
	key, err := h.read().GetSetting(ctx, settings.GoogleFontsAPIKey)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return false, err
	}
	return familyListed(family, h.googleFontFamilies(ctx, strings.TrimSpace(key))), nil
}

// familyListed reports whether family is among families, or families is empty.
func familyListed(family string, families []string) bool {
	return len(families) == 0 || slices.Contains(families, family)
}

func (h handlers) getGoogleFontFile(_ context.Context, in *googleFontFileInput) (*fontStreamOutput, error) {
	f, info, err := h.GoogleFonts.File(in.Name)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, huma.Error404NotFound("There is no such font file.")
	}
	if err != nil {
		return nil, h.fail("open a google font file", err)
	}
	return &fontStreamOutput{Body: func(c huma.Context) {
		defer func() { _ = f.Close() }() // read only
		r, w := humago.Unwrap(c)
		hdr := w.Header()
		hdr.Set("Content-Type", "font/woff2")
		hdr.Set("Cache-Control", googleFileCache)
		// The name is the hash of the bytes, so it is the tag too.
		hdr.Set("ETag", `"`+strings.TrimSuffix(in.Name, ".woff2")+`"`)
		http.ServeContent(w, r, "", info.ModTime(), f)
	}}, nil
}

// fileTag is an entity tag for a file, from its size and the time it was written.
func fileTag(info fs.FileInfo) string {
	return `"` + strconv.FormatInt(info.Size(), 16) + "-" + strconv.FormatInt(info.ModTime().UnixMicro(), 16) + `"`
}
