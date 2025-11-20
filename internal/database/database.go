package database

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"

	_ "github.com/duckdb/duckdb-go/v2"
	"github.com/google/uuid"

	"hitkeep/internal/api"
)

type Store struct {
	db   *sql.DB
	path string
}

func NewStore(path string) *Store {
	return &Store{
		path: path,
	}
}

func (s *Store) Connect() error {
	slog.Info("Connecting to database...", "path", s.path)
	db, err := sql.Open("duckdb", s.path)
	if err != nil {
		return fmt.Errorf("could not open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return fmt.Errorf("could not connect to database: %w", err)
	}

	s.db = db
	slog.Debug("Database connection established successfully.")
	return nil
}

func (s *Store) Close() error {
	slog.Debug("Closing database connection...")
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *Store) DB() *sql.DB {
	return s.db
}

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

// GetSiteStats returns aggregated KPIs and time-series data for the chart.
func (s *Store) GetSiteStats(ctx context.Context, siteID uuid.UUID, userID uuid.UUID, start, end time.Time) (*api.SiteStats, error) {
	var exists int
	err := s.db.QueryRowContext(ctx, "SELECT 1 FROM sites WHERE id = ? AND user_id = ?", siteID, userID).Scan(&exists)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("site not found or access denied")
	} else if err != nil {
		return nil, err
	}

	stats := &api.SiteStats{
		ChartData: []api.ChartDataPoint{},
	}

	// 1. Get Totals and Bounce Rate
	kpiQuery := `
	WITH session_counts AS (
		SELECT session_id, count(*) as pvs
		FROM hits
		WHERE site_id = ? AND timestamp >= ? AND timestamp <= ?
		GROUP BY session_id
	)
	SELECT 
		COALESCE(SUM(pvs), 0) as total_pageviews,
		COUNT(session_id) as unique_sessions,
		CASE 
			WHEN COUNT(session_id) = 0 THEN 0 
			ELSE CAST(COUNT(CASE WHEN pvs = 1 THEN 1 END) AS FLOAT) / COUNT(session_id) * 100 
		END as bounce_rate
	FROM session_counts;
	`
	err = s.db.QueryRowContext(ctx, kpiQuery, siteID, start, end).Scan(&stats.TotalPageviews, &stats.UniqueSessions, &stats.BounceRate)
	if err != nil {
		return nil, fmt.Errorf("failed to calc KPIs: %w", err)
	}

	// 2. Get Time Series Data for Chart (Gap Filled)
	// Fix: Cast both sides of the join key to TIMESTAMP to prevent TIMESTAMPTZ vs TIMESTAMP mismatch.
	// We use `date_trunc('day', ...)` which is standard.
	chartQuery := `
	WITH time_range AS (
		SELECT unnest(generate_series(?::TIMESTAMP, ?::TIMESTAMP, INTERVAL 1 DAY)) as bucket
	),
	daily_hits AS (
		SELECT 
			date_trunc('day', timestamp)::TIMESTAMP as bucket,
			COUNT(*) as pageviews,
			COUNT(DISTINCT session_id) as visitors
		FROM hits
		WHERE site_id = ? AND timestamp >= ? AND timestamp <= ?
		GROUP BY bucket
	)
	SELECT 
		tr.bucket,
		COALESCE(dh.pageviews, 0),
		COALESCE(dh.visitors, 0)
	FROM time_range tr
	LEFT JOIN daily_hits dh ON tr.bucket = dh.bucket
	ORDER BY tr.bucket ASC;
	`

	// Normalize start/end to midnight for the series generation
	startDay := time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, time.UTC)
	endDay := time.Date(end.Year(), end.Month(), end.Day(), 0, 0, 0, 0, time.UTC)

	// Params:
	// 1, 2: generate_series bounds (startDay, endDay)
	// 3: siteID
	// 4: hits filter lower bound (startDay) - Include entire first day
	// 5: hits filter upper bound (end) - Include up to now
	rows, err := s.db.QueryContext(ctx, chartQuery, startDay, endDay, siteID, startDay, end)
	if err != nil {
		return nil, fmt.Errorf("failed to query chart data: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var p api.ChartDataPoint
		if err := rows.Scan(&p.Time, &p.Pageviews, &p.Visitors); err != nil {
			return nil, err
		}
		stats.ChartData = append(stats.ChartData, p)
	}

	return stats, nil
}

func (s *Store) GetUserCount(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("could not query user count: %w", err)
	}
	return count, nil
}

func (s *Store) GetUserByEmail(ctx context.Context, email string) (*api.User, error) {
	var user api.User
	err := s.db.QueryRowContext(ctx,
		"SELECT id, email, password, created_at FROM users WHERE email = ?",
		email,
	).Scan(&user.ID, &user.Email, &user.Password, &user.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("could not query user: %w", err)
	}
	return &user, nil
}

func (s *Store) CreateUser(ctx context.Context, email string, hashedPassword string) (uuid.UUID, error) {
	id := uuid.New()
	_, err := s.db.ExecContext(ctx,
		"INSERT INTO users (id, email, password, created_at) VALUES (?, ?, ?, ?)",
		id, email, hashedPassword, time.Now(),
	)
	if err != nil {
		return uuid.Nil, fmt.Errorf("could not create user: %w", err)
	}
	return id, nil
}
