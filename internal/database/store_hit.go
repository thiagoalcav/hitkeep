package database

import (
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

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

	filterSQL, filterArgs := buildHitFilter(params.FilterType, params.FilterValue, "h")
	baseQuery += filterSQL
	args = append(args, filterArgs...)

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

func (s *Store) ExportHitsCSV(ctx context.Context, params api.HitQueryParams, w io.Writer) error {
	baseQuery := `
		FROM hits h
		JOIN sites s ON h.site_id = s.id
		WHERE h.site_id = ? 
		  AND s.user_id = ? 
		  AND h.timestamp >= ? 
		  AND h.timestamp <= ?
	`
	args := []any{params.SiteID, params.UserID, params.Start, params.End}

	filterSQL, filterArgs := buildHitFilter(params.FilterType, params.FilterValue, "h")
	baseQuery += filterSQL
	args = append(args, filterArgs...)

	if params.Query != "" {
		baseQuery += ` AND (h.path ILIKE ? OR h.referrer ILIKE ? OR h.user_agent ILIKE ?)`
		wildcard := "%" + params.Query + "%"
		args = append(args, wildcard, wildcard, wildcard)
	}

	baseQuery += " ORDER BY h.timestamp DESC"

	selectQuery := `
		SELECT
            h.id, h.site_id, h.session_id, h.page_id, h.timestamp, h.path, h.referrer, h.user_agent,
            h.viewport_width, h.viewport_height, h.screen_width, h.screen_height, h.language, h.is_unique
	` + baseQuery

	rows, err := s.db.QueryContext(ctx, selectQuery, args...)
	if err != nil {
		return fmt.Errorf("failed to query hits for export: %w", err)
	}
	defer rows.Close()

	writer := csv.NewWriter(w)
	if err := writer.Write([]string{
		"id",
		"site_id",
		"session_id",
		"page_id",
		"timestamp",
		"path",
		"referrer",
		"user_agent",
		"viewport_width",
		"viewport_height",
		"screen_width",
		"screen_height",
		"language",
		"is_unique",
	}); err != nil {
		return fmt.Errorf("failed to write csv header: %w", err)
	}

	for rows.Next() {
		var (
			id, siteID, sessionID, pageID uuid.UUID
			timestamp                     time.Time
			path                          string
			referrer, userAgent, language sql.NullString
			viewportWidth, viewportHeight sql.NullInt32
			screenWidth, screenHeight     sql.NullInt32
			isUnique                      sql.NullBool
		)
		if err := rows.Scan(
			&id,
			&siteID,
			&sessionID,
			&pageID,
			&timestamp,
			&path,
			&referrer,
			&userAgent,
			&viewportWidth,
			&viewportHeight,
			&screenWidth,
			&screenHeight,
			&language,
			&isUnique,
		); err != nil {
			return fmt.Errorf("failed to scan export row: %w", err)
		}

		record := []string{
			id.String(),
			siteID.String(),
			sessionID.String(),
			pageID.String(),
			timestamp.Format(time.RFC3339),
			path,
			nullString(referrer),
			nullString(userAgent),
			nullInt32(viewportWidth),
			nullInt32(viewportHeight),
			nullInt32(screenWidth),
			nullInt32(screenHeight),
			nullString(language),
			nullBool(isUnique),
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("failed to write csv record: %w", err)
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return fmt.Errorf("failed to flush csv: %w", err)
	}
	return nil
}

func nullString(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func nullInt32(value sql.NullInt32) string {
	if value.Valid {
		return strconv.FormatInt(int64(value.Int32), 10)
	}
	return ""
}

func nullBool(value sql.NullBool) string {
	if value.Valid {
		if value.Bool {
			return "true"
		}
		return "false"
	}
	return ""
}
