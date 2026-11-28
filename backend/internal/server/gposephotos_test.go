package server_test

import (
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"app-suite/internal/server"
)

// gposePhotoFile is where a gpose's file lies, from its address.
func (e *testEnv) gposePhotoFile(url string) string {
	return filepath.Join(e.srv.DataDirForTest(), "gpose-photos", path.Base(url))
}

// gposePhotoPath is a gpose's address without its site, for the env's client.
func gposePhotoPath(t *testing.T, url string) string {
	t.Helper()
	i := strings.Index(url, "/api/v1/gpose-photos/")
	if i < 0 {
		t.Fatalf("not a gpose address: %q", url)
	}
	return url[i:]
}

// uploadGposePhoto sends a picture as a sign-up's gpose.
func (e *testEnv) uploadGposePhoto(t *testing.T, boothID int, s map[string]any, data []byte) *http.Response {
	t.Helper()
	return e.upload(t, signupPath(boothID, s, "/photo"), nil, filePart{field: "image", name: "shot.png", data: data, kind: "image/png"})
}

func exists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}

// Staff upload a gpose for a completed sign-up; the visitor sees it on their sign-up
// and downloads it by its address, named after their character, until its day is over.
func TestGposePhotoUploadAndDownload(t *testing.T) {
	env := newTestEnv(t)
	env.loginAdmin(t)
	solo := env.createGposeType(t, "Solo")
	id := env.createGposeBooth(t, map[string]any{"types": []map[string]any{{"typeId": solo, "price": 0}}})
	decodeInto(t, env.gposeSignUp(t, id, gposeSignUpBody("Aya Lantern", "Gilgamesh", solo, nil)), http.StatusCreated, nil)
	s := env.gposeSignups(t, id)["Aya Lantern"]
	if s["photoUrl"] != "" || s["photoExpiresAt"] != nil {
		t.Fatalf("a new sign-up has a gpose: %v", s)
	}

	png := realPNG(t, 40, 30)
	if got := status(env.uploadGposePhoto(t, id, s, png)); got != http.StatusConflict {
		t.Errorf("a gpose for a waiting sign-up: %d, want 409", got)
	}
	decodeInto(t, env.patchJSON(t, signupPath(id, s, ""), map[string]any{"status": "completed"}), http.StatusOK, nil)
	if got := status(env.uploadGposePhoto(t, id, s, []byte("not a picture"))); got != http.StatusUnprocessableEntity {
		t.Errorf("a gpose that isn't a picture: %d, want 422", got)
	}
	if got := status(env.uploadGposePhoto(t, id, s, smallGIF(t))); got != http.StatusUnprocessableEntity {
		t.Errorf("a GIF gpose: %d, want 422", got)
	}
	if got := status(env.upload(t, gposeBoothPath(id, "/signups/999/photo"), nil, filePart{field: "image", name: "a.png", data: png, kind: "image/png"})); got != http.StatusNotFound {
		t.Errorf("a gpose for no sign-up: %d, want 404", got)
	}

	before := time.Now().UTC()
	var up map[string]any
	decodeInto(t, env.uploadGposePhoto(t, id, s, png), http.StatusOK, &up)
	url, _ := up["photoUrl"].(string)
	if !strings.HasPrefix(url, "https://") || !strings.Contains(url, "/api/v1/gpose-photos/") {
		t.Fatalf("photoUrl %q", url)
	}
	until, err := time.Parse(time.RFC3339, up["photoExpiresAt"].(string))
	if err != nil || until.Before(before.Add(24*time.Hour-time.Minute)) || until.After(before.Add(24*time.Hour+time.Minute)) {
		t.Errorf("photoExpiresAt %v, want a day from now", up["photoExpiresAt"])
	}
	if !exists(env.gposePhotoFile(url)) {
		t.Fatal("the gpose's file wasn't written")
	}

	// The visitor's own view carries it.
	var mine struct {
		Signups []map[string]any `json:"signups"`
	}
	decodeInto(t, env.postJSON(t, gposeBoothPath(id, "/lookup"), map[string]any{"characterName": "aya lantern", "world": "Gilgamesh"}), http.StatusOK, &mine)
	if len(mine.Signups) != 1 || mine.Signups[0]["photoUrl"] != url {
		t.Errorf("the visitor's sign-ups %v", mine.Signups)
	}

	// Anyone with its address downloads it, signed in or not.
	env.signOut(t)
	resp := env.get(t, gposePhotoPath(t, url))
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "image/png" || len(body) == 0 {
		t.Fatalf("download: %d %q, %d bytes", resp.StatusCode, resp.Header.Get("Content-Type"), len(body))
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, `filename=gpose-Aya-Lantern.png`) {
		t.Errorf("Content-Disposition %q", cd)
	}
	cc := resp.Header.Get("Cache-Control")
	age, err := strconv.Atoi(strings.TrimPrefix(cc, "private, max-age="))
	if !strings.HasPrefix(cc, "private, max-age=") || err != nil || age > 86400 || age < 86000 {
		t.Errorf("Cache-Control %q, want private for the rest of the day", cc)
	}
	for _, bad := range []string{"/api/v1/gpose-photos/0123456789abcdef0123456789abcdef", "/api/v1/gpose-photos/..%2Fdatabase", "/api/v1/gpose-photos/ABC"} {
		if got := status(env.get(t, bad)); got != http.StatusNotFound {
			t.Errorf("GET %s: %d, want 404", bad, got)
		}
	}
	env.loginAdmin(t)

	// Another upload replaces it: the first address and file are gone.
	var again map[string]any
	decodeInto(t, env.uploadGposePhoto(t, id, s, realPNG(t, 20, 20)), http.StatusOK, &again)
	second := again["photoUrl"].(string)
	if second == url || exists(env.gposePhotoFile(url)) || !exists(env.gposePhotoFile(second)) {
		t.Errorf("replacing: first %q (file kept %v), second %q", url, exists(env.gposePhotoFile(url)), second)
	}
	if got := status(env.get(t, gposePhotoPath(t, url))); got != http.StatusNotFound {
		t.Errorf("the replaced gpose: %d, want 404", got)
	}

	// Deleting it takes the file and the address.
	if got := status(env.del(t, signupPath(id, s, "/photo"))); got != http.StatusNoContent {
		t.Fatalf("delete the gpose: %d", got)
	}
	if exists(env.gposePhotoFile(second)) || status(env.get(t, gposePhotoPath(t, second))) != http.StatusNotFound {
		t.Error("a deleted gpose is still there")
	}
	if got := env.gposeSignups(t, id)["Aya Lantern"]["photoUrl"]; got != "" {
		t.Errorf("photoUrl after the delete %v", got)
	}
	if got := status(env.del(t, signupPath(id, s, "/photo"))); got != http.StatusNoContent {
		t.Errorf("deleting no gpose: %d, want 204", got)
	}
}

// A gpose's day ends to the moment: its address answers 404 and the visitor's pages
// drop it before the sweep has run, and the sweep then deletes its file.
func TestGposePhotoExpires(t *testing.T) {
	env := newTestEnv(t)
	env.loginAdmin(t)
	solo := env.createGposeType(t, "Solo")
	id := env.createGposeBooth(t, map[string]any{"types": []map[string]any{{"typeId": solo, "price": 0}}})
	decodeInto(t, env.gposeSignUp(t, id, gposeSignUpBody("Aya Lantern", "Gilgamesh", solo, nil)), http.StatusCreated, nil)
	s := env.gposeSignups(t, id)["Aya Lantern"]
	decodeInto(t, env.patchJSON(t, signupPath(id, s, ""), map[string]any{"status": "completed"}), http.StatusOK, nil)
	var up map[string]any
	decodeInto(t, env.uploadGposePhoto(t, id, s, realPNG(t, 10, 10)), http.StatusOK, &up)
	url := up["photoUrl"].(string)

	// A sweep before the day is over leaves it alone.
	server.ExpireGposePhotosForTest(env.srv, t.Context(), time.Now())
	if !exists(env.gposePhotoFile(url)) || status(env.get(t, gposePhotoPath(t, url))) != http.StatusOK {
		t.Fatal("a sweep took a gpose whose day isn't over")
	}

	// Its day ends: read as over at once, ahead of any sweep.
	if _, err := env.store.DB().ExecContext(t.Context(),
		"UPDATE gpose_signups SET photo_expires_at = strftime('%Y-%m-%dT%H:%M:%SZ', 'now', '-1 second') WHERE id = ?",
		num(s["id"])); err != nil {
		t.Fatal(err)
	}
	if got := status(env.get(t, gposePhotoPath(t, url))); got != http.StatusNotFound {
		t.Errorf("a gpose past its day: %d, want 404", got)
	}
	if got := env.gposeSignups(t, id)["Aya Lantern"]["photoUrl"]; got != "" {
		t.Errorf("staff still see a gpose past its day: %v", got)
	}
	if !exists(env.gposePhotoFile(url)) {
		t.Fatal("the file went before the sweep")
	}

	server.ExpireGposePhotosForTest(env.srv, t.Context(), time.Now())
	if exists(env.gposePhotoFile(url)) {
		t.Error("the sweep left the file of a gpose past its day")
	}
	var token string
	if err := env.store.DB().QueryRowContext(t.Context(), "SELECT photo_token FROM gpose_signups WHERE id = ?", num(s["id"])).Scan(&token); err != nil || token != "" {
		t.Errorf("the sign-up still names its gpose: %q, %v", token, err)
	}

	// A file nobody names goes once it is an hour old; any file goes after a day.
	dir := filepath.Join(env.srv.DataDirForTest(), "gpose-photos")
	orphan := filepath.Join(dir, "0123456789abcdef0123456789abcdef")
	writeFile(t, dir, "0123456789abcdef0123456789abcdef", []byte("left behind"))
	server.ExpireGposePhotosForTest(env.srv, t.Context(), time.Now())
	if !exists(orphan) {
		t.Error("a file nobody names went before its hour")
	}
	server.ExpireGposePhotosForTest(env.srv, t.Context(), time.Now().Add(2*time.Hour))
	if exists(orphan) {
		t.Error("a file nobody names outlived its hour")
	}
}

// Deleting a sign-up or its booth deletes its gpose.
func TestGposePhotoGoesWithItsSignup(t *testing.T) {
	env := newTestEnv(t)
	env.loginAdmin(t)
	solo := env.createGposeType(t, "Solo")
	id := env.createGposeBooth(t, map[string]any{"types": []map[string]any{{"typeId": solo, "price": 0}}})
	for _, name := range []string{"Aya Lantern", "Bo Lantern"} {
		decodeInto(t, env.gposeSignUp(t, id, gposeSignUpBody(name, "Gilgamesh", solo, nil)), http.StatusCreated, nil)
	}
	urls := map[string]string{}
	for name, s := range env.gposeSignups(t, id) {
		decodeInto(t, env.patchJSON(t, signupPath(id, s, ""), map[string]any{"status": "completed"}), http.StatusOK, nil)
		var up map[string]any
		decodeInto(t, env.uploadGposePhoto(t, id, s, realPNG(t, 10, 10)), http.StatusOK, &up)
		urls[name] = up["photoUrl"].(string)
	}
	// Undoing the completion keeps it: the 24 hours still run.
	aya := env.gposeSignups(t, id)["Aya Lantern"]
	decodeInto(t, env.patchJSON(t, signupPath(id, aya, ""), map[string]any{"status": "waiting"}), http.StatusOK, nil)
	if !exists(env.gposePhotoFile(urls["Aya Lantern"])) {
		t.Error("undoing a completion deleted its gpose")
	}

	if got := status(env.del(t, signupPath(id, aya, ""))); got != http.StatusNoContent {
		t.Fatalf("delete a sign-up: %d", got)
	}
	if exists(env.gposePhotoFile(urls["Aya Lantern"])) {
		t.Error("a deleted sign-up's gpose is still there")
	}
	if got := status(env.del(t, gposeBoothPath(id, ""))); got != http.StatusNoContent {
		t.Fatalf("delete the booth: %d", got)
	}
	if exists(env.gposePhotoFile(urls["Bo Lantern"])) {
		t.Error("a deleted booth's gpose is still there")
	}
}

// The festival dashboard shows a gpose at a closed booth too, so closing a booth at the
// end of a day doesn't take a visitor's gpose with it; a closed booth without one stays
// off the dashboard.
func TestGposePhotoOnTheDashboard(t *testing.T) {
	env := newTestEnv(t)
	env.loginAdmin(t)
	f := env.createFestival(t)
	solo := env.createGposeType(t, "Solo")
	id := env.createGposeBooth(t, map[string]any{
		"festivalMapId": f.mapID, "types": []map[string]any{{"typeId": solo, "price": 0}},
	})
	decodeInto(t, env.gposeSignUp(t, id, gposeSignUpBody("Aya Lantern", "Gilgamesh", solo, nil)), http.StatusCreated, nil)
	s := env.gposeSignups(t, id)["Aya Lantern"]
	decodeInto(t, env.patchJSON(t, signupPath(id, s, ""), map[string]any{"status": "completed"}), http.StatusOK, nil)
	decodeInto(t, env.postJSON(t, gposeBoothPath(id, "/close"), nil), http.StatusOK, nil)

	visitor := map[string]any{"characterName": "Aya Lantern", "world": "Gilgamesh"}
	if booths := env.dashboard(t, f.mapID, visitor)["gposeBooths"].([]any); len(booths) != 0 {
		t.Errorf("a closed booth without a gpose is on the dashboard: %v", booths)
	}

	var up map[string]any
	decodeInto(t, env.uploadGposePhoto(t, id, s, realPNG(t, 10, 10)), http.StatusOK, &up)
	booths := env.dashboard(t, f.mapID, visitor)["gposeBooths"].([]any)
	if len(booths) != 1 {
		t.Fatalf("dashboard booths %v", booths)
	}
	b := booths[0].(map[string]any)
	signups := b["signups"].([]any)
	if b["open"] != false || len(signups) != 1 || signups[0].(map[string]any)["photoUrl"] != up["photoUrl"] {
		t.Errorf("the closed booth on the dashboard %v", b)
	}
	if starts := b["booth"].(map[string]any)["types"].([]any)[0].(map[string]any)["starts"].([]any); len(starts) != 0 {
		t.Errorf("a closed booth offers times %v", starts)
	}
	// Somebody else sees neither the booth nor the gpose.
	other := env.dashboard(t, f.mapID, map[string]any{"characterName": "Bo Lantern", "world": "Gilgamesh"})
	if booths := other["gposeBooths"].([]any); len(booths) != 0 {
		t.Errorf("another visitor's dashboard %v", booths)
	}
}
