package media

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"time"
)

// The gposes staff upload for completed gpose booth sign-ups, for the visitor to
// download. Each is one file under <dataDir>/gpose-photos, named by a random token
// that is also its address (/api/v1/gpose-photos/<token>), with no extension: its
// content type is kept beside its sign-up. Nothing serves the folder as files; the API
// answers a token only while its gpose's day lasts, and a sweep deletes each file once
// it is over. The folder is left out of the uploads backup, since nothing in it lives
// past a day.

// gposePhotoGrace is how old a file must be before Prune counts it as nobody's: one
// whose upload is still recording its sign-up is never caught.
const gposePhotoGrace = time.Hour

// GposePhotos is the gposes under a data folder.
type GposePhotos struct{ root string }

// NewGposePhotos is the gposes under dataDir.
func NewGposePhotos(dataDir string) *GposePhotos { return &GposePhotos{root: dataDir} }

func (p *GposePhotos) dir() string { return filepath.Join(p.root, "gpose-photos") }

// ValidGposePhotoToken reports whether token is one Save could have made: 32 lowercase
// hex characters. Anything else names no file, so it is never opened.
func ValidGposePhotoToken(token string) bool {
	if len(token) != 32 {
		return false
	}
	for _, c := range token {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

// Save writes a gpose under a new random token and answers the token.
func (p *GposePhotos) Save(data []byte) (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	token := hex.EncodeToString(b)
	if err := os.MkdirAll(p.dir(), 0o750); err != nil {
		return "", err
	}
	if err := os.WriteFile(filepath.Join(p.dir(), token), data, 0o600); err != nil {
		return "", err
	}
	return token, nil
}

// Open opens the gpose of token, with its file's details. A token that isn't one is
// fs.ErrNotExist, like a file that is gone.
func (p *GposePhotos) Open(token string) (*os.File, fs.FileInfo, error) {
	if !ValidGposePhotoToken(token) {
		return nil, nil, fs.ErrNotExist
	}
	f, err := os.Open(filepath.Join(p.dir(), token)) //nolint:gosec // the token is checked as hex, so the path stays in the folder
	if err != nil {
		return nil, nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close() // the stat failed; the close has nothing to add
		return nil, nil, err
	}
	if !info.Mode().IsRegular() {
		_ = f.Close() // not a file of ours; nothing to report on close
		return nil, nil, fs.ErrNotExist
	}
	return f, info, nil
}

// Remove deletes the gpose of token. Best effort: one already gone is fine, and any
// other failure is logged, since the sweep tries again for a file left behind.
func (p *GposePhotos) Remove(token string) {
	if !ValidGposePhotoToken(token) {
		return
	}
	if err := os.Remove(filepath.Join(p.dir(), token)); err != nil && !errors.Is(err, fs.ErrNotExist) {
		slog.Warn("remove a gpose photo", "error", err)
	}
}

// Prune deletes the files that outlived their day: every one older than maxAge, whatever
// holds it, and every one held by no sign-up (an upload whose sign-up never recorded it)
// once it is an hour old. It answers how many it deleted.
func (p *GposePhotos) Prune(held map[string]bool, maxAge time.Duration, now time.Time) (int, error) {
	entries, err := os.ReadDir(p.dir())
	if errors.Is(err, fs.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	removed := 0
	for _, e := range entries {
		if !e.Type().IsRegular() {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		age := now.Sub(info.ModTime())
		if age < maxAge && (held[e.Name()] || age < gposePhotoGrace) {
			continue
		}
		if err := os.Remove(filepath.Join(p.dir(), e.Name())); err != nil && !errors.Is(err, fs.ErrNotExist) {
			slog.Warn("prune a gpose photo", "error", err)
			continue
		}
		removed++
	}
	return removed, nil
}
