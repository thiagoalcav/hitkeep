package database

import (
	"context"
	"fmt"
	"time"

	"hitkeep/internal/api"

	"github.com/google/uuid"
)

func (s *Store) CreateHit(ctx context.Context, hit *api.Hit) error {
	if hit.Timestamp.IsZero() {
		hit.Timestamp = time.Now()
	}

	_, err := s.db.ExecContext(ctx, `
        INSERT INTO hits (
            site_id, session_id, page_id, timestamp, path, referrer, user_agent, 
            viewport_width, viewport_height, screen_width, screen_height, 
            language, is_unique
        ) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		hit.SiteID, hit.SessionID, hit.PageID, hit.Timestamp, hit.Path, hit.Referrer,
		hit.UserAgent, hit.ViewportWidth, hit.ViewportHeight, hit.ScreenWidth,
		hit.ScreenHeight, hit.Language, hit.IsUnique,
	)
	if err != nil {
		return fmt.Errorf("could not insert hit: %w", err)
	}
	return nil
}

// GetHits returns raw hits.
// Note: In high-volume production, this query should be restricted or paginated heavily.
func (s *Store) GetHits(ctx context.Context, siteID uuid.UUID, userID uuid.UUID) ([]api.Hit, error) {
	query := `
        SELECT
            h.id, h.site_id, h.session_id, h.page_id, h.timestamp, h.path, h.referrer, h.user_agent,
            h.viewport_width, h.viewport_height, h.screen_width, h.screen_height, h.language, h.is_unique
        FROM hits h
        JOIN sites s ON h.site_id = s.id
        WHERE h.site_id = ? AND s.user_id = ?
        ORDER BY h.timestamp DESC
        LIMIT 100`

	rows, err := s.db.QueryContext(ctx, query, siteID, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	hits := []api.Hit{}
	for rows.Next() {
		var hit api.Hit
		if err := rows.Scan(
			&hit.ID, &hit.SiteID, &hit.SessionID, &hit.PageID, &hit.Timestamp, &hit.Path, &hit.Referrer,
			&hit.UserAgent, &hit.ViewportWidth, &hit.ViewportHeight, &hit.ScreenWidth,
			&hit.ScreenHeight, &hit.Language, &hit.IsUnique,
		); err != nil {
			return nil, err
		}
		hits = append(hits, hit)
	}
	return hits, nil
}
