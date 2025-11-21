package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"hitkeep/internal/api"
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

// GetHits returns paginated, sorted, and filtered hits.
func (s *Store) GetHits(ctx context.Context, params api.HitQueryParams) (*api.PaginatedHits, error) {
	baseQuery := `
		FROM hits h
		JOIN sites s ON h.site_id = s.id
		WHERE h.site_id = ? 
		  AND s.user_id = ? 
		  AND h.timestamp >= ? 
		  AND h.timestamp <= ?
	`
	args := []any{params.SiteID, params.UserID, params.Start, params.End}

	if params.Query != "" {
		baseQuery += ` AND (h.path ILIKE ? OR h.referrer ILIKE ? OR h.user_agent ILIKE ?)`
		wildcard := "%" + params.Query + "%"
		args = append(args, wildcard, wildcard, wildcard)
	}

	var total int
	countQuery := "SELECT COUNT(*) " + baseQuery
	if err := s.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, fmt.Errorf("failed to count hits: %w", err)
	}

	// Whitelist
	orderBy := "h.timestamp" // default
	switch params.SortField {
	case "path":
		orderBy = "h.path"
	case "referrer":
		orderBy = "h.referrer"
	case "timestamp":
		orderBy = "h.timestamp"
	}

	orderDir := "DESC"
	if strings.ToLower(params.SortOrder) == "asc" {
		orderDir = "ASC"
	}

	baseQuery += fmt.Sprintf(" ORDER BY %s %s", orderBy, orderDir)

	baseQuery += " LIMIT ? OFFSET ?"
	args = append(args, params.Limit, params.Offset)

	//nolint:gosec
	selectQuery := `
		SELECT
            h.id, h.site_id, h.session_id, h.page_id, h.timestamp, h.path, h.referrer, h.user_agent,
            h.viewport_width, h.viewport_height, h.screen_width, h.screen_height, h.language, h.is_unique
	` + baseQuery // whitelisted

	rows, err := s.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query hits: %w", err)
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

	return &api.PaginatedHits{
		Data:  hits,
		Total: total,
	}, nil
}
