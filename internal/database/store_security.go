package database

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
	"hitkeep/internal/security"
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

type DisableUserMFAResult struct {
	TOTPDisabled        bool
	PasskeysDeleted     int
	SessionsInvalidated int
}

type RecoveryCodeStatus struct {
	Generated bool
	Remaining int
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

func (s *Store) DisableUserMFA(ctx context.Context, userID uuid.UUID) (DisableUserMFAResult, error) {
	var result DisableUserMFAResult

	err := s.Transact(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM user_totp_pending_setup WHERE user_id = ?", userID); err != nil {
			return fmt.Errorf("could not remove pending totp setup: %w", err)
		}

		totpRes, err := tx.ExecContext(ctx, "DELETE FROM user_totp_factors WHERE user_id = ?", userID)
		if err != nil {
			return fmt.Errorf("could not remove enabled totp factor: %w", err)
		}
		if rows, rowsErr := totpRes.RowsAffected(); rowsErr == nil {
			result.TOTPDisabled = rows > 0
		}

		if _, err := tx.ExecContext(ctx, "DELETE FROM user_passkey_challenges WHERE user_id = ?", userID); err != nil {
			return fmt.Errorf("could not remove pending passkey challenge: %w", err)
		}

		passkeyRes, err := tx.ExecContext(ctx, "DELETE FROM user_passkeys WHERE user_id = ?", userID)
		if err != nil {
			return fmt.Errorf("could not remove user passkeys: %w", err)
		}
		if rows, rowsErr := passkeyRes.RowsAffected(); rowsErr == nil {
			result.PasskeysDeleted = int(rows)
		}

		if _, err := tx.ExecContext(ctx, "DELETE FROM passkey_login_challenges WHERE user_id = ?", userID); err != nil {
			return fmt.Errorf("could not remove passkey login challenges: %w", err)
		}
		if _, err := tx.ExecContext(ctx, "DELETE FROM user_recovery_codes WHERE user_id = ?", userID); err != nil {
			return fmt.Errorf("could not remove recovery codes: %w", err)
		}

		sessionRes, err := tx.ExecContext(ctx, "DELETE FROM remember_me_tokens WHERE user_id = ?", userID)
		if err != nil {
			return fmt.Errorf("could not invalidate remember me tokens: %w", err)
		}
		if rows, rowsErr := sessionRes.RowsAffected(); rowsErr == nil {
			result.SessionsInvalidated = int(rows)
		}

		return nil
	})
	if err != nil {
		return DisableUserMFAResult{}, err
	}

	return result, nil
}

func (s *Store) ReplaceUserRecoveryCodes(ctx context.Context, userID uuid.UUID, codeHashes []string) error {
	now := time.Now().UTC()
	return s.Transact(ctx, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, "DELETE FROM user_recovery_codes WHERE user_id = ?", userID); err != nil {
			return fmt.Errorf("could not delete existing recovery codes: %w", err)
		}
		for _, codeHash := range codeHashes {
			if strings.TrimSpace(codeHash) == "" {
				return fmt.Errorf("recovery code hash is required")
			}
			if _, err := tx.ExecContext(ctx,
				"INSERT INTO user_recovery_codes (id, user_id, code_hash, created_at) VALUES (?, ?, ?, ?)",
				uuid.New(), userID, strings.TrimSpace(codeHash), now,
			); err != nil {
				return fmt.Errorf("could not insert recovery code: %w", err)
			}
		}
		return nil
	})
}

func (s *Store) GetRecoveryCodeStatus(ctx context.Context, userID uuid.UUID) (RecoveryCodeStatus, error) {
	var (
		total     int
		remaining int
	)
	if err := s.QueryRowOrNil(ctx,
		"SELECT COUNT(*), COALESCE(SUM(CASE WHEN used_at IS NULL THEN 1 ELSE 0 END), 0) FROM user_recovery_codes WHERE user_id = ?",
		[]any{&total, &remaining},
		userID,
	); err != nil {
		return RecoveryCodeStatus{}, fmt.Errorf("could not load recovery code status: %w", err)
	}
	return RecoveryCodeStatus{
		Generated: total > 0,
		Remaining: remaining,
	}, nil
}

func (s *Store) CountActiveRecoveryCodes(ctx context.Context, userID uuid.UUID) (int, error) {
	status, err := s.GetRecoveryCodeStatus(ctx, userID)
	if err != nil {
		return 0, err
	}
	return status.Remaining, nil
}

func (s *Store) ConsumeRecoveryCode(ctx context.Context, userID uuid.UUID, code string) (int, bool, error) {
	var remaining int
	err := s.Transact(ctx, func(tx *sql.Tx) error {
		rows, err := tx.QueryContext(ctx,
			"SELECT id, code_hash FROM user_recovery_codes WHERE user_id = ? AND used_at IS NULL",
			userID,
		)
		if err != nil {
			return fmt.Errorf("could not load active recovery codes: %w", err)
		}
		defer rows.Close()

		var matchedID uuid.UUID
		for rows.Next() {
			var (
				id          uuid.UUID
				encodedHash string
			)
			if err := rows.Scan(&id, &encodedHash); err != nil {
				return fmt.Errorf("could not scan recovery code: %w", err)
			}
			match, err := security.VerifyRecoveryCode(code, encodedHash)
			if err != nil {
				return fmt.Errorf("could not verify recovery code: %w", err)
			}
			if match {
				matchedID = id
				break
			}
		}
		if err := rows.Err(); err != nil {
			return fmt.Errorf("could not iterate recovery codes: %w", err)
		}
		if matchedID == uuid.Nil {
			remaining = -1
			return nil
		}

		result, err := tx.ExecContext(ctx,
			"UPDATE user_recovery_codes SET used_at = ? WHERE id = ? AND user_id = ? AND used_at IS NULL",
			time.Now().UTC(), matchedID, userID,
		)
		if err != nil {
			return fmt.Errorf("could not consume recovery code: %w", err)
		}
		affected, err := result.RowsAffected()
		if err != nil {
			return fmt.Errorf("could not determine consumed recovery code rows: %w", err)
		}
		if affected == 0 {
			remaining = -1
			return nil
		}

		if err := tx.QueryRowContext(ctx,
			"SELECT COUNT(*) FROM user_recovery_codes WHERE user_id = ? AND used_at IS NULL",
			userID,
		).Scan(&remaining); err != nil {
			return fmt.Errorf("could not count remaining recovery codes: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, false, err
	}
	if remaining < 0 {
		return 0, false, nil
	}
	return remaining, true, nil
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
	if signCount > math.MaxUint32 {
		return nil, fmt.Errorf("passkey sign count is out of range: %d", signCount)
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
