package database

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
)

func (s *Store) CreateShareLink(ctx context.Context, siteID uuid.UUID, createdBy uuid.UUID) (string, error) {
	token, tokenHash, err := generateShareToken()
	if err != nil {
		return "", err
	}

	now := time.Now().UTC()

	if err := s.Exec(ctx,
		"INSERT INTO share_links (site_id, token_hash, created_by, created_at) VALUES (?, ?, ?, ?)",
		siteID, tokenHash, createdBy, now,
	); err != nil {
		return "", fmt.Errorf("failed to create share link: %w", err)
	}

	return token, nil
}

func (s *Store) GetShareSiteByToken(ctx context.Context, token string) (*api.Site, error) {
	if token == "" {
		return nil, nil
	}

	tokenHash := hashShareToken(token)

	var site api.Site
	err := s.db.QueryRowContext(ctx, `
		SELECT s.id, s.user_id, s.domain, COALESCE(s.data_retention_days, 0), s.created_at
		FROM share_links sl
		JOIN sites s ON sl.site_id = s.id
		WHERE sl.token_hash = ? AND sl.revoked_at IS NULL
	`, tokenHash).Scan(&site.ID, &site.UserID, &site.Domain, &site.DataRetentionDays, &site.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to lookup share link: %w", err)
	}

	return &site, nil
}

func generateShareToken() (string, string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}
	token := hex.EncodeToString(bytes)
	return token, hashShareToken(token), nil
}

func hashShareToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
