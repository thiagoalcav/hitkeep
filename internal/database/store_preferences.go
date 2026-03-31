package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
)

const defaultUserLocale = "en"

// GetUserPreferences returns stored preferences or nil if none exist.
func (s *Store) GetUserPreferences(ctx context.Context, userID uuid.UUID) (*api.UserPreferences, error) {
	var defaultLocale sql.NullString
	err := s.QueryRowOrNil(
		ctx,
		"SELECT default_locale FROM user_preferences WHERE user_id = ?",
		[]any{&defaultLocale},
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("could not query user preferences: %w", err)
	}
	if !defaultLocale.Valid {
		return nil, nil
	}

	return &api.UserPreferences{
		DefaultLocale: defaultLocale.String,
	}, nil
}

func (s *Store) GetUserLocale(ctx context.Context, userID uuid.UUID) (string, error) {
	prefs, err := s.GetUserPreferences(ctx, userID)
	if err != nil {
		return "", err
	}
	if prefs == nil || strings.TrimSpace(prefs.DefaultLocale) == "" {
		return defaultUserLocale, nil
	}
	return prefs.DefaultLocale, nil
}

// UpsertUserPreferences inserts or updates user preferences.
func (s *Store) UpsertUserPreferences(ctx context.Context, userID uuid.UUID, prefs api.UserPreferences) error {
	now := time.Now().UTC()
	err := s.Exec(ctx, `
		INSERT INTO user_preferences (user_id, default_locale, updated_at)
		VALUES (?, ?, ?)
		ON CONFLICT (user_id) DO UPDATE SET
			default_locale = excluded.default_locale,
			updated_at = excluded.updated_at
	`, userID, prefs.DefaultLocale, now)
	if err != nil {
		return fmt.Errorf("could not upsert user preferences: %w", err)
	}

	return nil
}
