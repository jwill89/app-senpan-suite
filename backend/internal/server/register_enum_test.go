package server

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"app-suite/internal/db"
	"app-suite/internal/live"
	"app-suite/internal/store"
	"app-suite/internal/store/storetest"
)

// newInternalServer is a whole server on a fresh database, for a test inside the
// package that sets what New doesn't take (such as the Turnstile secret).
func newInternalServer(t *testing.T) (*Server, *store.Store, *httptest.Server) {
	t.Helper()
	st, err := store.New(t.Context(), storetest.Path(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	s := New(t.Context(), st, live.New(), t.TempDir(), nil)
	ts := httptest.NewTLSServer(s)
	t.Cleanup(ts.Close)
	return s, st, ts
}

// postFrom sends body as JSON to path and answers the status and the body read.
func postFrom(t *testing.T, ts *httptest.Server, path string, body any) (int, string) {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := ts.Client().Post(ts.URL+path, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	got, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(got)
}

// TestRegisterNoEnumeration: a sign-up answers identically - status and body -
// whether the name is new or already taken, so it can't be used to find out which
// accounts exist. The eligible account is still made.
func TestRegisterNoEnumeration(t *testing.T) {
	_, st, ts := newInternalServer(t)
	post := func(username string) (int, string) {
		return postFrom(t, ts, "/api/v1/auth/register", map[string]string{"username": username, "password": "password123"})
	}

	newCode, newBody := post("brandnew") // made
	dupCode, dupBody := post("brandnew") // now taken

	if newCode != http.StatusAccepted || dupCode != http.StatusAccepted {
		t.Fatalf("every sign-up must answer 202; got new=%d taken=%d", newCode, dupCode)
	}
	if newBody != dupBody {
		t.Errorf("the answers must be identical:\n new=%q\n taken=%q", newBody, dupBody)
	}
	row, err := db.New(st.ReadDB()).GetUserByUsername(t.Context(), "brandnew")
	if err != nil {
		t.Fatalf("the eligible sign-up must still make the account: %v", err)
	}
	if row.IsActive != 0 {
		t.Error("a new account must wait for an admin to activate it")
	}
}

// Nothing is special about the name admin: signing up with it makes an account like
// any other, inactive, with no admin role.
func TestRegisterTakesTheNameAdmin(t *testing.T) {
	_, st, ts := newInternalServer(t)
	if code, _ := postFrom(t, ts, "/api/v1/auth/register", map[string]string{"username": "admin", "password": "password123"}); code != http.StatusAccepted {
		t.Fatalf("sign-up as admin: %d; want 202", code)
	}
	row, err := db.New(st.ReadDB()).GetUserByUsername(t.Context(), "admin")
	if err != nil {
		t.Fatalf("signing up as admin made no account: %v", err)
	}
	if row.IsActive != 0 || row.IsAdmin != 0 {
		t.Errorf("the account signed up as admin is active=%d admin=%d; want neither", row.IsActive, row.IsAdmin)
	}
}
