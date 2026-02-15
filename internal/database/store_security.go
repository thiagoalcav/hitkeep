package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
)

type PasskeyCredential struct {
	ID           uuid.UUID
	UserID       uuid.UUID
	Name         string
	CredentialID string
	PublicKey    string
	SignCount    uint32
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

type CreateLoginChallengeInput struct {
	UserID     *uuid.UUID
	RememberMe bool
	Flow       string
}

type LoginChallenge struct {
	ID         uuid.UUID
	UserID     uuid.UUID
	HasUserID  bool
	RememberMe bool
	Flow       string
	Challenge  string
	ExpiresAt  time.Time
}

func (s *Store) HasEnabledTOTP(ctx context.Context, userID uuid.UUID) (bool, error) {
	var count int
	if err := s.QueryRowOrNil(ctx,
		"SELECT COUNT(*) FROM user_totp_factors WHERE user_id = ?",
		[]any{&count},
		userID,
	); err != nil {
		return false, fmt.Errorf("could not check enabled totp factor: %w", err)
	}
	return count > 0, nil
}

func (s *Store) HasPendingTOTPSetup(ctx context.Context, userID uuid.UUID) (bool, error) {
	var count int
	if err := s.QueryRowOrNil(ctx,
		"SELECT COUNT(*) FROM user_totp_pending_setup WHERE user_id = ?",
		[]any{&count},
		userID,
	); err != nil {
		return false, fmt.Errorf("could not check pending totp setup: %w", err)
	}
	return count > 0, nil
}

func (s *Store) CreatePendingTOTPSetup(ctx context.Context, userID uuid.UUID, secret string, expiresAt time.Time) error {
	now := time.Now().UTC()
	return s.Transact(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM user_totp_pending_setup WHERE user_id = ?", userID); err != nil {
			return fmt.Errorf("could not remove existing pending totp setup: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO user_totp_pending_setup (user_id, secret, created_at, expires_at) VALUES (?, ?, ?, ?)",
			userID, secret, now, expiresAt.UTC(),
		); err != nil {
			return fmt.Errorf("could not insert pending totp setup: %w", err)
		}
		return nil
	})
}

func (s *Store) DeletePendingTOTPSetup(ctx context.Context, userID uuid.UUID) error {
	return s.Exec(ctx, "DELETE FROM user_totp_pending_setup WHERE user_id = ?", userID)
}

func (s *Store) GetPendingTOTPSetup(ctx context.Context, userID uuid.UUID) (secret string, expiresAt time.Time, found bool, err error) {
	err = s.QueryRowOrNil(ctx,
		"SELECT secret, expires_at FROM user_totp_pending_setup WHERE user_id = ?",
		[]any{&secret, &expiresAt},
		userID,
	)
	if err != nil {
		return "", time.Time{}, false, fmt.Errorf("could not get pending totp setup: %w", err)
	}
	if strings.TrimSpace(secret) == "" {
		return "", time.Time{}, false, nil
	}
	return secret, expiresAt, true, nil
}

func (s *Store) EnableUserTOTP(ctx context.Context, userID uuid.UUID, secret string) error {
	now := time.Now().UTC()
	return s.Transact(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM user_totp_pending_setup WHERE user_id = ?", userID); err != nil {
			return fmt.Errorf("could not remove pending totp setup: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM user_totp_factors WHERE user_id = ?", userID); err != nil {
			return fmt.Errorf("could not remove existing totp factor: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO user_totp_factors (user_id, secret, enabled_at, updated_at) VALUES (?, ?, ?, ?)",
			userID, secret, now, now,
		); err != nil {
			return fmt.Errorf("could not insert enabled totp factor: %w", err)
		}
		return nil
	})
}

func (s *Store) GetUserTOTPSecret(ctx context.Context, userID uuid.UUID) (secret string, found bool, err error) {
	var enabledAt time.Time
	err = s.QueryRowOrNil(ctx,
		"SELECT secret, enabled_at FROM user_totp_factors WHERE user_id = ?",
		[]any{&secret, &enabledAt},
		userID,
	)
	if err != nil {
		return "", false, fmt.Errorf("could not get user totp secret: %w", err)
	}
	if strings.TrimSpace(secret) == "" {
		return "", false, nil
	}
	return secret, true, nil
}

func (s *Store) DisableUserTOTP(ctx context.Context, userID uuid.UUID) error {
	return s.Transact(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM user_totp_pending_setup WHERE user_id = ?", userID); err != nil {
			return fmt.Errorf("could not remove pending totp setup: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM user_totp_factors WHERE user_id = ?", userID); err != nil {
			return fmt.Errorf("could not remove enabled totp factor: %w", err)
		}
		return nil
	})
}

func (s *Store) CreatePasskeyChallenge(ctx context.Context, userID uuid.UUID, challenge string, requestedName string, expiresAt time.Time) error {
	now := time.Now().UTC()
	return s.Transact(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM user_passkey_challenges WHERE user_id = ?", userID); err != nil {
			return fmt.Errorf("could not remove existing passkey challenge: %w", err)
		}
		if _, err := tx.ExecContext(ctx,
			"INSERT INTO user_passkey_challenges (user_id, challenge, requested_name, created_at, expires_at) VALUES (?, ?, ?, ?, ?)",
			userID, challenge, strings.TrimSpace(requestedName), now, expiresAt.UTC(),
		); err != nil {
			return fmt.Errorf("could not insert passkey challenge: %w", err)
		}
		return nil
	})
}

func (s *Store) GetPasskeyChallenge(ctx context.Context, userID uuid.UUID) (challenge string, requestedName string, expiresAt time.Time, found bool, err error) {
	err = s.QueryRowOrNil(ctx,
		"SELECT challenge, COALESCE(requested_name, ''), expires_at FROM user_passkey_challenges WHERE user_id = ?",
		[]any{&challenge, &requestedName, &expiresAt},
		userID,
	)
	if err != nil {
		return "", "", time.Time{}, false, fmt.Errorf("could not get passkey challenge: %w", err)
	}
	if strings.TrimSpace(challenge) == "" {
		return "", "", time.Time{}, false, nil
	}
	return challenge, requestedName, expiresAt, true, nil
}

func (s *Store) DeletePasskeyChallenge(ctx context.Context, userID uuid.UUID) error {
	return s.Exec(ctx, "DELETE FROM user_passkey_challenges WHERE user_id = ?", userID)
}

func (s *Store) CreateUserPasskey(ctx context.Context, userID uuid.UUID, name string, credentialID string, publicKey string, transports []string) (uuid.UUID, error) {
	passkeyID := uuid.New()
	now := time.Now().UTC()

	name = strings.TrimSpace(name)
	if name == "" {
		name = "Passkey"
	}

	credentialID = strings.TrimSpace(credentialID)
	if credentialID == "" {
		return uuid.Nil, fmt.Errorf("credential id is required")
	}

	var transportsJSON *string
	if len(transports) > 0 {
		payload, err := json.Marshal(transports)
		if err != nil {
			return uuid.Nil, fmt.Errorf("could not encode passkey transports: %w", err)
		}
		value := string(payload)
		transportsJSON = &value
	}

	if err := s.Exec(ctx,
		"INSERT INTO user_passkeys (id, user_id, name, credential_id, public_key, transports_json, sign_count, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)",
		passkeyID, userID, name, credentialID, strings.TrimSpace(publicKey), transportsJSON, 0, now, now,
	); err != nil {
		return uuid.Nil, fmt.Errorf("could not insert user passkey: %w", err)
	}

	return passkeyID, nil
}

func (s *Store) ListUserPasskeys(ctx context.Context, userID uuid.UUID) ([]api.UserPasskey, error) {
	passkeys := make([]api.UserPasskey, 0)
	if err := s.QueryList(ctx,
		"SELECT id, name, created_at, updated_at FROM user_passkeys WHERE user_id = ? ORDER BY created_at DESC",
		func(rows *sql.Rows) error {
			var row api.UserPasskey
			if err := rows.Scan(&row.ID, &row.Name, &row.CreatedAt, &row.UpdatedAt); err != nil {
				return fmt.Errorf("could not scan user passkey: %w", err)
			}
			passkeys = append(passkeys, row)
			return nil
		},
		userID,
	); err != nil {
		return nil, fmt.Errorf("could not list user passkeys: %w", err)
	}
	return passkeys, nil
}

func (s *Store) DeleteUserPasskey(ctx context.Context, userID uuid.UUID, passkeyID uuid.UUID) error {
	if err := s.ExecRowsAffected(ctx, "DELETE FROM user_passkeys WHERE id = ? AND user_id = ?", passkeyID, userID); err != nil {
		return fmt.Errorf("could not delete user passkey: %w", err)
	}
	return nil
}

func (s *Store) GetPasskeyByCredentialID(ctx context.Context, credentialID string) (*PasskeyCredential, error) {
	var row PasskeyCredential
	var signCount int64
	err := s.QueryRowOrNil(ctx,
		"SELECT id, user_id, name, credential_id, COALESCE(public_key, ''), sign_count, created_at, updated_at FROM user_passkeys WHERE credential_id = ?",
		[]any{&row.ID, &row.UserID, &row.Name, &row.CredentialID, &row.PublicKey, &signCount, &row.CreatedAt, &row.UpdatedAt},
		strings.TrimSpace(credentialID),
	)
	if err != nil {
		return nil, fmt.Errorf("could not get passkey by credential id: %w", err)
	}
	if row.ID == uuid.Nil {
		return nil, nil
	}
	if signCount < 0 {
		signCount = 0
	}
	row.SignCount = uint32(signCount)
	return &row, nil
}

func (s *Store) UpdatePasskeySignCount(ctx context.Context, passkeyID uuid.UUID, signCount uint32) error {
	if err := s.ExecRowsAffected(ctx,
		"UPDATE user_passkeys SET sign_count = ?, updated_at = ? WHERE id = ?",
		int64(signCount), time.Now().UTC(), passkeyID,
	); err != nil {
		return fmt.Errorf("could not update passkey sign count: %w", err)
	}
	return nil
}

func (s *Store) CreatePasskeyLoginChallenge(ctx context.Context, challenge string, input CreateLoginChallengeInput, expiresAt time.Time) (uuid.UUID, error) {
	challengeID := uuid.New()
	now := time.Now().UTC()
	flow := strings.TrimSpace(input.Flow)
	if flow == "" {
		flow = "passwordless"
	}
	var userIDArg any
	if input.UserID != nil && *input.UserID != uuid.Nil {
		userIDArg = *input.UserID
	}
	if err := s.Exec(ctx,
		"INSERT INTO passkey_login_challenges (id, user_id, challenge, remember_me, flow, created_at, expires_at) VALUES (?, ?, ?, ?, ?, ?, ?)",
		challengeID, userIDArg, strings.TrimSpace(challenge), input.RememberMe, flow, now, expiresAt.UTC(),
	); err != nil {
		return uuid.Nil, fmt.Errorf("could not create passkey login challenge: %w", err)
	}
	return challengeID, nil
}

func (s *Store) GetPasskeyLoginChallenge(ctx context.Context, challengeID uuid.UUID) (row LoginChallenge, found bool, err error) {
	var userIDRaw string
	err = s.QueryRowOrNil(ctx,
		"SELECT challenge, COALESCE(CAST(user_id AS VARCHAR), ''), remember_me, COALESCE(flow, ''), expires_at FROM passkey_login_challenges WHERE id = ?",
		[]any{&row.Challenge, &userIDRaw, &row.RememberMe, &row.Flow, &row.ExpiresAt},
		challengeID,
	)
	if err != nil {
		return LoginChallenge{}, false, fmt.Errorf("could not get passkey login challenge: %w", err)
	}
	if strings.TrimSpace(row.Challenge) == "" {
		return LoginChallenge{}, false, nil
	}
	row.ID = challengeID
	if parsedUserID, parseErr := uuid.Parse(strings.TrimSpace(userIDRaw)); parseErr == nil {
		row.UserID = parsedUserID
		row.HasUserID = parsedUserID != uuid.Nil
	}
	return row, true, nil
}

func (s *Store) DeletePasskeyLoginChallenge(ctx context.Context, challengeID uuid.UUID) error {
	return s.Exec(ctx, "DELETE FROM passkey_login_challenges WHERE id = ?", challengeID)
}
