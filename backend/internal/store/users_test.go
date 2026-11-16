package store_test

import (
	"database/sql"
	"errors"
	"testing"

	"app-suite/internal/auth"
	"app-suite/internal/db"
)

// A new database has no account: the first admin is made on the host.
func TestNoAccountIsSeeded(t *testing.T) {
	s := newTestStore(t)
	users, err := db.New(s.ReadDB()).ListUsers(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(users) != 0 {
		t.Errorf("a new database holds %d accounts; want none", len(users))
	}
}

// MakeAdmin makes an account that isn't there: active, an admin, with the password
// given.
func TestMakeAdminMakesTheAccount(t *testing.T) {
	s := newTestStore(t)
	hash, err := auth.Hash("first-password")
	if err != nil {
		t.Fatal(err)
	}
	made, err := s.MakeAdmin(t.Context(), "Mochi", hash)
	if err != nil || !made {
		t.Fatalf("MakeAdmin = %v, %v; want the account made", made, err)
	}
	q := db.New(s.ReadDB())
	row, err := q.GetUserByUsername(t.Context(), "Mochi")
	if err != nil {
		t.Fatal(err)
	}
	if row.IsActive != 1 || row.IsAdmin != 1 || row.Permissions != "[]" {
		t.Errorf("made active=%d admin=%d permissions=%s; want an active admin", row.IsActive, row.IsAdmin, row.Permissions)
	}
	stored, err := q.GetPasswordHash(t.Context(), row.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := auth.Verify("first-password", stored); !ok {
		t.Error("the account doesn't take the password given")
	}
}

// MakeAdmin on an account that is there activates it, makes it an admin and resets its
// password: the epoch moves on and its token is revoked, while its permissions stay.
func TestMakeAdminResetsAnAccount(t *testing.T) {
	s := newTestStore(t)
	w := db.New(s.DB())
	old, err := auth.Hash("old-password")
	if err != nil {
		t.Fatal(err)
	}
	if err := w.CreateUser(t.Context(), db.CreateUserParams{Username: "kiri", PasswordHash: old}); err != nil {
		t.Fatal(err)
	}
	before, err := w.GetUserByUsername(t.Context(), "kiri")
	if err != nil {
		t.Fatal(err)
	}
	if err := w.UpdateUser(t.Context(), db.UpdateUserParams{IsActive: 0, IsAdmin: 0, Permissions: `["bingo-cards"]`, ID: before.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := w.SetToken(t.Context(), db.SetTokenParams{UserID: before.ID, TokenHash: "h", TokenPrefix: "pat_x"}); err != nil {
		t.Fatal(err)
	}

	fresh, err := auth.Hash("new-password")
	if err != nil {
		t.Fatal(err)
	}
	made, err := s.MakeAdmin(t.Context(), "kiri", fresh)
	if err != nil || made {
		t.Fatalf("MakeAdmin = %v, %v; want the account reset, not made", made, err)
	}
	q := db.New(s.ReadDB())
	after, err := q.GetUserByUsername(t.Context(), "kiri")
	if err != nil {
		t.Fatal(err)
	}
	if after.ID != before.ID || after.IsActive != 1 || after.IsAdmin != 1 || after.Permissions != `["bingo-cards"]` {
		t.Errorf("after the reset: %+v; want the same account, active, an admin, its permissions kept", after)
	}
	if after.PasswordEpoch != before.PasswordEpoch+1 {
		t.Errorf("password epoch %d; want %d, so its sessions end", after.PasswordEpoch, before.PasswordEpoch+1)
	}
	stored, err := q.GetPasswordHash(t.Context(), after.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ok, _ := auth.Verify("new-password", stored); !ok {
		t.Error("the account doesn't take the new password")
	}
	if _, err := q.GetToken(t.Context(), after.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("its token is still there (%v); want it revoked", err)
	}
}
