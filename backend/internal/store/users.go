package store

import (
	"context"
	"database/sql"
	"errors"

	"app-suite/internal/db"
)

// MakeAdmin makes the account named username an active admin whose password has the
// argon2id hash hash: the first admin, and the way back in when no admin can sign in
// (`app-suite admin`, run on the host). With no account by that name it makes one.
// Otherwise it activates that account, makes it an admin and sets its password as a
// reset does: the password epoch moves on, ending every session the account has, and
// its personal access token is revoked. Its permissions, passkeys and Discord link are
// kept. It reports whether it made the account.
//
// One transaction that takes the write lock up front, so it waits its turn beside a
// running server rather than racing it.
func (s *Store) MakeAdmin(ctx context.Context, username, hash string) (bool, error) {
	tx, err := s.beginImmediate(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback() }()
	q := db.New(tx)

	made := false
	row, err := q.GetUserByUsername(ctx, username)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		made = true
		if err := q.CreateUser(ctx, db.CreateUserParams{Username: username, PasswordHash: hash}); err != nil {
			return false, err
		}
		if row, err = q.GetUserByUsername(ctx, username); err != nil {
			return false, err
		}
	case err != nil:
		return false, err
	default:
		if err := q.SetPassword(ctx, db.SetPasswordParams{PasswordHash: hash, ID: row.ID}); err != nil {
			return false, err
		}
		if _, err := q.DeleteToken(ctx, row.ID); err != nil {
			return false, err
		}
	}
	if err := q.UpdateUser(ctx, db.UpdateUserParams{
		IsActive: 1, IsAdmin: 1, Permissions: row.Permissions, ID: row.ID,
	}); err != nil {
		return false, err
	}
	return made, tx.Commit()
}
