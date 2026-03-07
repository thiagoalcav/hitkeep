package database

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
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

	err := s.Transact(ctx, func(tx *sql.Tx) error {
		// Cleanup old tokens
		if _, err := tx.ExecContext(ctx, "DELETE FROM password_resets WHERE email = ?", email); err != nil {
			return fmt.Errorf("failed to cleanup old tokens: %w", err)
		}
		// Insert new
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO password_resets (email, token, created_at, expires_at) VALUES (?, ?, ?, ?)",
			email, token, now, expiresAt,
		); err != nil {
			return fmt.Errorf("failed to insert reset token: %w", err)
		}
		return nil
	})

	if err != nil {
		return "", err
	}
	return token, nil
}

// CompletePasswordReset verifies a token and updates the password in a single transaction.
func (s *Store) CompletePasswordReset(ctx context.Context, token string, newHashedPassword string) error {
	return s.Transact(ctx, func(tx *sql.Tx) error {
		var email string
		var expiresAt time.Time

		// 1. Validate Token
		err := tx.QueryRowContext(ctx,
			"SELECT email, expires_at FROM password_resets WHERE token = ?",
			token,
		).Scan(&email, &expiresAt)

		if err == sql.ErrNoRows {
			return fmt.Errorf("invalid or expired token")
		}
		if err != nil {
			return err
		}

		if time.Now().After(expiresAt) {
			_, _ = tx.ExecContext(ctx, "DELETE FROM password_resets WHERE token = ?", token)
			return fmt.Errorf("token expired")
		}

		// 2. Update Password
		res, err := tx.ExecContext(ctx, "UPDATE users SET password = ? WHERE email = ?", newHashedPassword, email)
		if err != nil {
			return err
		}
		if rows, _ := res.RowsAffected(); rows == 0 {
			return fmt.Errorf("user not found")
		}

		// 3. Cleanup
		_, _ = tx.ExecContext(ctx, "DELETE FROM remember_me_tokens WHERE user_id = (SELECT id FROM users WHERE email = ?)", email)
		_, err = tx.ExecContext(ctx, "DELETE FROM password_resets WHERE token = ?", token)
		return err
	})
}

func (s *Store) ResolvePasswordResetEmail(ctx context.Context, token string) (string, error) {
	var email string
	var expiresAt time.Time
	err := s.db.QueryRowContext(ctx,
		"SELECT email, expires_at FROM password_resets WHERE token = ?",
		token,
	).Scan(&email, &expiresAt)
	if err == sql.ErrNoRows {
		return "", fmt.Errorf("invalid or expired token")
	}
	if err != nil {
		return "", err
	}
	if time.Now().After(expiresAt) {
		_, _ = s.db.ExecContext(ctx, "DELETE FROM password_resets WHERE token = ?", token)
		return "", fmt.Errorf("token expired")
	}
	return email, nil
}

func (s *Store) UpdatePasswordByID(ctx context.Context, userID string, newHashedPassword string) error {
	return s.Transact(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "UPDATE users SET password = ? WHERE id = ?", newHashedPassword, userID); err != nil {
			return err
		}
		// Wipe active sessions
		if _, err := tx.ExecContext(ctx, "DELETE FROM remember_me_tokens WHERE user_id = ?", userID); err != nil {
			return err
		}
		return nil
	})
}

func (s *Store) CreateRememberMeToken(ctx context.Context, userID uuid.UUID) (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	token := hex.EncodeToString(bytes)
	now := time.Now().UTC()
	expiresAt := now.Add(30 * 24 * time.Hour)

	err := s.Exec(ctx,
		"INSERT INTO remember_me_tokens (token, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)",
		token, userID, now, expiresAt,
	)
	if err != nil {
		return "", fmt.Errorf("failed to insert remember me token: %w", err)
	}
	return token, nil
}

func (s *Store) ValidateRememberMeToken(ctx context.Context, token string) (uuid.UUID, error) {
	var userID uuid.UUID
	var expiresAt time.Time

	err := s.QueryRowOrNil(ctx,
		"SELECT user_id, expires_at FROM remember_me_tokens WHERE token = ?",
		[]any{&userID, &expiresAt},
		token,
	)
	if err != nil {
		return uuid.Nil, err
	}
	if userID == uuid.Nil {
		return uuid.Nil, nil
	}

	if time.Now().After(expiresAt) {
		_ = s.DeleteRememberMeToken(ctx, token)
		return uuid.Nil, nil
	}
	return userID, nil
}

func (s *Store) DeleteRememberMeToken(ctx context.Context, token string) error {
	return s.Exec(ctx, "DELETE FROM remember_me_tokens WHERE token = ?", token)
}

func (s *Store) DeleteAllRememberMeTokensForUser(ctx context.Context, userID uuid.UUID) error {
	return s.Exec(ctx, "DELETE FROM remember_me_tokens WHERE user_id = ?", userID)
}
