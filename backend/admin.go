package main

import (
	"bufio"
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"

	"app-suite/internal/auth"
	"app-suite/internal/store"
)

// adminUsage is how `app-suite admin` is called.
const adminUsage = "usage: app-suite admin -db <path> [-password-stdin] <username>"

// The exit statuses of `app-suite admin`.
const (
	exitOK     = 0
	exitFailed = 1
	exitUsage  = 2
)

// usernameShape is what a username must hold besides spaces, as the API's sign-up and
// rename require.
var usernameShape = regexp.MustCompile(`[\p{L}\p{N}\p{P}\p{S}]`)

// adminCommand is `app-suite admin`, run on the host: it makes username an active
// admin, making the account when there is none, or else activating it, making it an
// admin and giving it a new password, which signs out every session it has and revokes
// its access token (store.MakeAdmin). The first admin comes from here, and so does the
// way back in when no admin can sign in.
//
// The password is read from stdin with -password-stdin (one line; a trailing newline is
// dropped) and never printed; otherwise a random one is made and printed once. The
// database is migrated first, as the server does, and the change waits its turn beside
// a running server. It answers the process's exit status.
func adminCommand(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	say := lines{stdout}
	complain := lines{stderr}
	flags := flag.NewFlagSet("admin", flag.ContinueOnError)
	flags.SetOutput(stderr)
	dbPath := flags.String("db", "", "SQLite database path (required)")
	fromStdin := flags.Bool("password-stdin", false, "Read the password from standard input instead of making one")
	flags.Usage = func() {
		complain.line(adminUsage)
		flags.PrintDefaults()
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return exitOK
		}
		return exitUsage
	}
	if *dbPath == "" || flags.NArg() != 1 {
		flags.Usage()
		return exitUsage
	}
	username := strings.TrimSpace(flags.Arg(0))
	if err := checkUsername(username); err != nil {
		complain.line("admin: " + err.Error())
		return exitUsage
	}

	generated := !*fromStdin
	var password string
	if generated {
		password = rand.Text()
	} else {
		var err error
		if password, err = readPassword(stdin); err != nil {
			complain.line("admin: " + err.Error())
			return exitUsage
		}
	}
	hash, err := auth.Hash(password)
	if err != nil {
		complain.line("admin: hash the password: " + err.Error())
		return exitFailed
	}

	_, statErr := os.Stat(*dbPath)
	isNew := errors.Is(statErr, os.ErrNotExist)
	st, err := store.New(ctx, *dbPath)
	if err != nil {
		complain.line(fmt.Sprintf("admin: open the database %s: %v", *dbPath, err))
		return exitFailed
	}
	made, err := st.MakeAdmin(ctx, username, hash)
	if closeErr := st.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		complain.line(fmt.Sprintf("admin: make %q an admin: %v", username, err))
		return exitFailed
	}

	if isNew {
		say.line(fmt.Sprintf("Made a new database at %s.", *dbPath))
	}
	if made {
		say.line(fmt.Sprintf("Made the account %q: active, an admin.", username))
	} else {
		say.line(fmt.Sprintf("Reset the account %q: active, an admin, with a new password. "+
			"Its sessions are signed out and its access token is revoked.", username))
	}
	if generated {
		say.line("Password (shown this once): " + password)
		say.line("Sign in with it, then change it with Change Password.")
	}
	return exitOK
}

// lines writes the command's messages, a line each. A message that can't be written
// has nowhere else to go, so the error is dropped.
type lines struct{ w io.Writer }

func (l lines) line(text string) { _, _ = io.WriteString(l.w, text+"\n") }

// checkUsername is nil when name, already trimmed, can be a username.
func checkUsername(name string) error {
	switch {
	case !usernameShape.MatchString(name):
		return errors.New("a username needs more than spaces")
	case utf8.RuneCountInString(name) > auth.MaxUsernameLength:
		return fmt.Errorf("a username is at most %d characters", auth.MaxUsernameLength)
	}
	return nil
}

// maxPasswordLine is the most bytes a password's line may take: the longest password
// in four-byte characters, and a CRLF.
const maxPasswordLine = 4*auth.MaxPasswordLength + 2

// readPassword reads a password from the first line of r, without its line ending.
// Its length is checked, and an error never quotes it.
func readPassword(r io.Reader) (string, error) {
	line, err := bufio.NewReader(io.LimitReader(r, maxPasswordLine+1)).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return "", fmt.Errorf("read the password from standard input: %w", err)
	}
	password := strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
	switch n := utf8.RuneCountInString(password); {
	case n < auth.MinPasswordLength:
		return "", fmt.Errorf("the password from standard input is shorter than %d characters", auth.MinPasswordLength)
	case n > auth.MaxPasswordLength || len(line) > maxPasswordLine:
		return "", fmt.Errorf("the password from standard input is longer than %d characters", auth.MaxPasswordLength)
	}
	return password, nil
}
