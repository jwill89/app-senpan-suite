package main

import (
	"bytes"
	"database/sql"
	"errors"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"app-suite/internal/auth"
	"app-suite/internal/db"
	"app-suite/internal/store"
)

// runAdmin runs `app-suite admin` with args and stdin, and answers its exit status and
// what it wrote to stdout and stderr.
func runAdmin(t *testing.T, stdin string, args ...string) (int, string, string) {
	t.Helper()
	var out, errOut bytes.Buffer
	status := adminCommand(t.Context(), args, strings.NewReader(stdin), &out, &errOut)
	return status, out.String(), errOut.String()
}

// account is the account named username in the database at path, and whether its
// password is password; nil when there is none.
func account(t *testing.T, path, username, password string) (*db.GetUserByUsernameRow, bool) {
	t.Helper()
	st, err := store.New(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	q := db.New(st.ReadDB())
	row, err := q.GetUserByUsername(t.Context(), username)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false
	}
	if err != nil {
		t.Fatal(err)
	}
	hash, err := q.GetPasswordHash(t.Context(), row.ID)
	if err != nil {
		t.Fatal(err)
	}
	ok, _ := auth.Verify(password, hash)
	return row, ok
}

var shownPassword = regexp.MustCompile(`Password \(shown this once\): (\S+)`)

// Without -password-stdin the command makes a password and prints it once; the account
// is made, active and an admin, in a database it makes and migrates.
func TestAdminCommandMakesAnAccountWithAPassword(t *testing.T) {
	path := filepath.Join(t.TempDir(), "data", "database.sqlite")
	status, out, errOut := runAdmin(t, "", "-db", path, "Mochi")
	if status != exitOK {
		t.Fatalf("exit %d; stderr %q", status, errOut)
	}
	m := shownPassword.FindStringSubmatch(out)
	if m == nil {
		t.Fatalf("no password shown:\n%s", out)
	}
	password := m[1]
	if len(password) < auth.MinPasswordLength || strings.Count(out, password) != 1 {
		t.Errorf("password %q shown %d times; want one strong password shown once", password, strings.Count(out, password))
	}
	if !strings.Contains(out, "Made a new database at") || !strings.Contains(out, `Made the account "Mochi"`) {
		t.Errorf("output doesn't say what it made:\n%s", out)
	}
	row, ok := account(t, path, "Mochi", password)
	if row == nil || !ok || row.IsActive != 1 || row.IsAdmin != 1 {
		t.Errorf("account %+v, takes the password %v; want an active admin with it", row, ok)
	}

	// A second run makes another password, which replaces the first.
	status, out, _ = runAdmin(t, "", "-db", path, "Mochi")
	again := shownPassword.FindStringSubmatch(out)
	if status != exitOK || again == nil || again[1] == password {
		t.Fatalf("second run: exit %d, output %q; want a new password", status, out)
	}
	if strings.Contains(out, "Made a new database") {
		t.Error("the second run says it made the database")
	}
	if _, ok := account(t, path, "Mochi", password); ok {
		t.Error("the first password still signs in")
	}
}

// With -password-stdin the password is read from the first line of stdin and never
// printed; on an account that is there, it activates it, makes it an admin and resets
// its password, moving the epoch on and revoking its token.
func TestAdminCommandResetsAnAccountFromStdin(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database.sqlite")
	st, err := store.New(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	w := db.New(st.DB())
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
	if _, err := w.SetToken(t.Context(), db.SetTokenParams{UserID: before.ID, TokenHash: "h", TokenPrefix: "pat_x"}); err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}

	const password = "a long enough secret"
	status, out, errOut := runAdmin(t, password+"\r\n", "-db", path, "-password-stdin", "kiri")
	if status != exitOK {
		t.Fatalf("exit %d; stderr %q", status, errOut)
	}
	if strings.Contains(out+errOut, password) || strings.Contains(out, "Password") {
		t.Errorf("the password given was printed:\n%s%s", out, errOut)
	}
	if !strings.Contains(out, `Reset the account "kiri"`) {
		t.Errorf("output doesn't say it reset the account:\n%s", out)
	}
	row, ok := account(t, path, "kiri", password)
	if row == nil || !ok || row.IsActive != 1 || row.IsAdmin != 1 {
		t.Fatalf("account %+v, takes the password %v; want an active admin with it", row, ok)
	}
	if row.PasswordEpoch != before.PasswordEpoch+1 {
		t.Errorf("password epoch %d; want %d, so its sessions end", row.PasswordEpoch, before.PasswordEpoch+1)
	}
	st, err = store.New(t.Context(), path)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	if _, err := db.New(st.ReadDB()).GetToken(t.Context(), row.ID); !errors.Is(err, sql.ErrNoRows) {
		t.Errorf("its token is still there (%v); want it revoked", err)
	}
}

// A call it can't carry out exits 2 without touching the database, and never quotes a
// password it was given.
func TestAdminCommandRefusesBadCalls(t *testing.T) {
	path := filepath.Join(t.TempDir(), "database.sqlite")
	for name, c := range map[string]struct {
		stdin string
		args  []string
	}{
		"no database":       {"", []string{"Mochi"}},
		"no username":       {"", []string{"-db", path}},
		"two usernames":     {"", []string{"-db", path, "a", "b"}},
		"a blank username":  {"", []string{"-db", path, "   "}},
		"a long username":   {"", []string{"-db", path, strings.Repeat("x", auth.MaxUsernameLength+1)}},
		"a short password":  {"short-1\n", []string{"-db", path, "-password-stdin", "Mochi"}},
		"no password":       {"", []string{"-db", path, "-password-stdin", "Mochi"}},
		"a long password":   {strings.Repeat("p", auth.MaxPasswordLength+1), []string{"-db", path, "-password-stdin", "Mochi"}},
		"an unknown flag":   {"", []string{"-db", path, "-force", "Mochi"}},
		"flags after names": {"", []string{"Mochi", "-db", path}},
	} {
		status, out, errOut := runAdmin(t, c.stdin, c.args...)
		if status != exitUsage {
			t.Errorf("%s: exit %d; want %d", name, status, exitUsage)
		}
		if c.stdin != "" && strings.Contains(out+errOut, strings.TrimSpace(c.stdin)) {
			t.Errorf("%s: the password given was printed", name)
		}
	}
	if row, _ := account(t, path, "Mochi", ""); row != nil {
		t.Error("a refused call made the account")
	}
}
