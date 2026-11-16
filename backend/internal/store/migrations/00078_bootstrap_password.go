// Package migrations holds the migrations written in Go, for what SQL alone can't
// do. The numbered SQL files beside them are the store's, embedded there; goose runs
// both in version order.
package migrations

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"

	"github.com/pressly/goose/v3"

	"app-suite/internal/auth"
)

// Go is every migration written in Go, for goose's provider.
func Go() []*goose.Migration {
	return []*goose.Migration{
		goose.NewGoMigration(78, &goose.GoFunc{RunTx: clearBootstrapPassword}, nil),
	}
}

// clearBootstrapPassword is migration 78. Earlier releases seeded every new database
// with an account named admin whose password was "admin". When that account still has
// that password, the password is cleared, so a password everyone knows can't sign in:
// its password epoch moves on, ending every session it has, and its personal access
// token is revoked. The account, its passkeys and its Discord link stay; a password is
// set again on the host (`app-suite admin`). Any other password is left alone.
func clearBootstrapPassword(ctx context.Context, tx *sql.Tx) error {
	var id int64
	var hash string
	err := tx.QueryRowContext(ctx, `SELECT id, password_hash FROM users WHERE username = 'admin'`).Scan(&id, &hash)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && hash != "" {
		if known, _ := auth.Verify("admin", hash); known {
			if _, err := tx.ExecContext(ctx, `UPDATE users SET password_hash = '', password_epoch = password_epoch + 1 WHERE id = ?`, id); err != nil {
				return err
			}
			if _, err := tx.ExecContext(ctx, `DELETE FROM user_tokens WHERE user_id = ?`, id); err != nil {
				return err
			}
			slog.WarnContext(ctx, "the admin account's password was still \"admin\" and has been cleared; set one with `app-suite admin`")
		}
	}
	_, err = tx.ExecContext(ctx, `PRAGMA user_version = 78`)
	return err
}
