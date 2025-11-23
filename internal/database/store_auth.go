package database

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

// CreatePasswordResetToken generates a secure token, saves it, and returns it.
func (s *Store) CreatePasswordResetToken(ctx context.Context, email string) (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes)

	now := time.Now().UTC()
	expiresAt := now.Add(1 * time.Hour)

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback() }()

	_, err = tx.ExecContext(ctx, "DELETE FROM password_resets WHERE email = ?", email)
	if err != nil {
		return "", fmt.Errorf("failed to cleanup old tokens: %w", err)
	}

	_, err = tx.ExecContext(ctx,
		"INSERT INTO password_resets (email, token, created_at, expires_at) VALUES (?, ?, ?, ?)",
		email, token, now, expiresAt,
	)
	if err != nil {
		return "", fmt.Errorf("failed to insert reset token: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return "", err
	}

	return token, nil
}

// CompletePasswordReset verifies a token and updates the password in a single transaction.
func (s *Store) CompletePasswordReset(ctx context.Context, token string, newHashedPassword string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	var email string
	var expiresAt time.Time

	err = tx.QueryRowContext(ctx,
		"SELECT email, expires_at FROM password_resets WHERE token = ?",
		token,
	).Scan(&email, &expiresAt)

	if errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("invalid or expired token")
	}
	if err != nil {
		return fmt.Errorf("database error: %w", err)
	}

	if time.Now().After(expiresAt) {
		_, _ = tx.ExecContext(ctx, "DELETE FROM password_resets WHERE token = ?", token)
		return fmt.Errorf("token expired")
	}

	res, err := tx.ExecContext(ctx, "UPDATE users SET password = ? WHERE email = ?", newHashedPassword, email)
	if err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	rowsAffected, _ := res.RowsAffected()
	if rowsAffected == 0 {
		return fmt.Errorf("user not found")
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM password_resets WHERE token = ?", token); err != nil {
		return fmt.Errorf("failed to delete token: %w", err)
	}

	return tx.Commit()
}

func (s *Store) UpdatePasswordByID(ctx context.Context, userID string, newHashedPassword string) error {
	_, err := s.db.ExecContext(ctx, "UPDATE users SET password = ? WHERE id = ?", newHashedPassword, userID)
	return err
}
