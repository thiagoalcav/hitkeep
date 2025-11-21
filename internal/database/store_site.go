package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/google/uuid"

	"hitkeep/internal/api"
)

func (s *Store) FindSiteByDomain(ctx context.Context, domain string) (*api.Site, error) {
	var site api.Site
	err := s.db.QueryRowContext(ctx, "SELECT id, user_id, domain, created_at FROM sites WHERE domain = ?", domain).Scan(&site.ID, &site.UserID, &site.Domain, &site.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, fmt.Errorf("could not query for site: %w", err)
	}
	return &site, nil
}

func (s *Store) CreateSite(ctx context.Context, userID uuid.UUID, domain string) (*api.Site, error) {
	id := uuid.New()
	now := time.Now()

	_, err := s.db.ExecContext(ctx,
		"INSERT INTO sites (id, user_id, domain, created_at) VALUES (?, ?, ?, ?)",
		id, userID, domain, now,
	)
	if err != nil {
		return nil, fmt.Errorf("could not create site: %w", err)
	}

	return &api.Site{
		ID:        id,
		UserID:    userID,
		Domain:    domain,
		CreatedAt: now,
	}, nil
}

func (s *Store) GetSites(ctx context.Context, userID uuid.UUID) ([]api.Site, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT id, user_id, domain, created_at FROM sites WHERE user_id = ? ORDER BY created_at DESC",
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sites := []api.Site{}
	for rows.Next() {
		var site api.Site
		if err := rows.Scan(&site.ID, &site.UserID, &site.Domain, &site.CreatedAt); err != nil {
			return nil, err
		}
		sites = append(sites, site)
	}
	return sites, nil
}
